package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDFor_Deterministic(t *testing.T) {
	a := IDFor([]byte("hello world"))
	b := IDFor([]byte("hello world"))
	require.Equal(t, a, b, "same content must produce same ID")
	require.Len(t, a, 16, "ID must be 16 hex chars (8 SHA-256 bytes)")
}

func TestIDFor_ContentChangeShiftsID(t *testing.T) {
	a := IDFor([]byte("hello world"))
	b := IDFor([]byte("hello worle")) // one byte different
	require.NotEqual(t, a, b)
}

// countingClient records every upsert call.
type countingClient struct {
	calls atomic.Int64
	items map[string][]byte
}

func newCountingClient() *countingClient {
	return &countingClient{items: make(map[string][]byte)}
}

func (c *countingClient) UpsertDatasetItem(_ context.Context, _, id string, content []byte) error {
	c.calls.Add(1)
	c.items[id] = content
	return nil
}

func TestUploader_DryRunMakesNoCalls(t *testing.T) {
	c := newCountingClient()
	u := &Uploader{
		Client:    c,
		DatasetID: "ds-test",
		DryRun:    true,
		Logger:    discardLogger(),
	}
	err := u.Upsert(t.Context(), []DatasetItem{
		{Title: "a", Content: []byte(`{"x":1}`)},
		{Title: "b", Content: []byte(`{"x":2}`)},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), c.calls.Load(), "dry-run should not hit the client")
}

func TestUploader_IdempotentIDs(t *testing.T) {
	c := newCountingClient()
	u := &Uploader{
		Client:    c,
		DatasetID: "ds-test",
		Logger:    discardLogger(),
	}
	items := []DatasetItem{
		{Title: "a", Content: []byte(`{"x":1}`)},
		{Title: "b", Content: []byte(`{"x":2}`)},
	}
	require.NoError(t, u.Upsert(t.Context(), items))
	require.NoError(t, u.Upsert(t.Context(), items))

	// Same content uploaded twice; the IDs must collide so the
	// dataset has exactly two distinct rows.
	require.Len(t, c.items, 2, "idempotent IDs should converge to 2 items")
	require.Equal(t, int64(4), c.calls.Load(), "each upload is still 2 calls")
}

func TestNewHTTPClient_MissingCredentials(t *testing.T) {
	_, err := NewHTTPClient(ClientOptions{Host: "https://example.com"})
	require.ErrorIs(t, err, ErrMissingCredentials)
}

func TestHTTPClient_UpsertHits4xxAndSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer srv.Close()

	c, err := NewHTTPClient(ClientOptions{
		Host:      srv.URL,
		PublicKey: "pk",
		SecretKey: "sk",
	})
	require.NoError(t, err)

	err = c.UpsertDatasetItem(t.Context(), "ds", "id1", []byte(`{"x":1}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 400")
}

func TestParseItems_DecodesRegressionEnvelope(t *testing.T) {
	envelope := map[string]any{
		"version": "v1",
		"sample": map[string]any{
			"listings": []map[string]any{
				{"ID": "L0001", "Title": "Dell R740"},
				{"ID": "L0002", "Title": "HP DL380"},
			},
		},
	}
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)

	items, err := ParseItems(raw)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "Dell R740", items[0].Title)
}

func TestParseItems_RejectsMissingVersion(t *testing.T) {
	_, err := ParseItems([]byte(`{"sample":{"listings":[]}}`))
	require.Error(t, err)
}
