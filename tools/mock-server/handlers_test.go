package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer(ServerOptions{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Scenario: "default",
	})
	require.NoError(t, err)
	return srv
}

func TestContainsAllWords(t *testing.T) {
	cases := []struct {
		title string
		terms []string
		want  bool
	}{
		{"dell poweredge r740 xeon gold", []string{"dell", "r740"}, true},
		{"dell poweredge r740 xeon gold", []string{"Dell", "R740"}, false}, // caller must lowercase
		{"dell poweredge r740 xeon gold", []string{"dell", "epyc"}, false},
		{"hp proliant dl380 gen10", []string{}, true},
		{"hp proliant dl380 gen10", []string{""}, true}, // empty terms tolerated
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, containsAllWords(tc.title, tc.terms),
			"containsAllWords(%q, %v)", tc.title, tc.terms)
	}
}

func TestServer_GetItem_HitAndMiss(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Routes()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/buy/browse/v1/item/v1%7C151234567890%7C0", nil)
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"v1|151234567890|0"`)

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/buy/browse/v1/item/v1%7Cnope", nil)
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), `"errorId":11001`)
}

func TestServer_Search_Filters(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Routes()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/buy/browse/v1/item_summary/search?q=dell+r740", nil)
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, float64(1), body["total"])
}

func TestServer_OAuthToken(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Routes()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/identity/v1/oauth2/token", strings.NewReader(""))
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.NotEmpty(t, resp.AccessToken)
	require.Equal(t, "Bearer", resp.TokenType)
}

func TestServer_AdminScenarioFlip(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Routes()

	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"name":"sold-listings"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/scenario", body)
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "sold-listings", srv.ActiveScenario())

	// Item under sold-listings is now OUT_OF_STOCK.
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/buy/browse/v1/item/v1%7C151234567890%7C0", nil)
	handler.ServeHTTP(rec, req)
	require.Contains(t, rec.Body.String(), "OUT_OF_STOCK")
}

func TestServer_Analytics(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Routes()

	// Drive one search to bump the counter.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/buy/browse/v1/item_summary/search?q=dell", nil)
	handler.ServeHTTP(rec, req)

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/developer/analytics/v1_beta/rate_limit/?api_context=buy&api_name=browse", nil)
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"apiContext":"buy"`)
}

func TestServer_AdminSetQuota_HeaderReflects(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Routes()

	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"count":4500,"limit":5000}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/quota", body)
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/buy/browse/v1/item_summary/search?q=dell", nil)
	handler.ServeHTTP(rec, req)
	require.Equal(t, "5000", rec.Header().Get("X-EBAY-API-Call-Limit"))
	require.Equal(t, "4501", rec.Header().Get("X-EBAY-API-Calls-Made"))
}

// TestServer_EndToEnd_SmokeOverHTTP starts the server on :0 and drives
// requests through net/http to confirm the wire shape — a thin
// substitute for the "real internal/ebay/Client" smoke test the IMPL
// calls for, which we can wire once internal/ebay/Client has an
// implementation (currently it's interface-only).
func TestServer_EndToEnd_SmokeOverHTTP(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	client := ts.Client()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/buy/browse/v1/item_summary/search?q=dell", http.NoBody)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	req, _ = http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/buy/browse/v1/item/v1%7C151234567890%7C0", http.NoBody)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}
