package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the Langfuse surface this tool needs. Concrete
// implementations: httpClient (production) and tests' own mocks.
// Re-check for an official Langfuse Go SDK before each release; swap
// behind this same interface if one exists.
type Client interface {
	UpsertDatasetItem(ctx context.Context, datasetID, itemID string, content []byte) error
}

// httpClient is the production Langfuse client. Talks to the upstream
// REST API directly — no third-party SDK dependency (Resolved
// Decision #4 in IMPL-0002).
type httpClient struct {
	host       string
	publicKey  string
	secretKey  string
	httpClient *http.Client
}

// ClientOptions configures NewHTTPClient.
type ClientOptions struct {
	Host       string // e.g., "https://cloud.langfuse.com"
	PublicKey  string
	SecretKey  string
	HTTPClient *http.Client // optional; defaults to a 30s-timeout client
}

// ErrMissingCredentials is returned when Host, PublicKey, or
// SecretKey is empty.
var ErrMissingCredentials = errors.New("dataset-upload: missing Langfuse credentials")

// NewHTTPClient validates credentials and returns a Client.
func NewHTTPClient(opts ClientOptions) (Client, error) {
	if opts.Host == "" || opts.PublicKey == "" || opts.SecretKey == "" {
		return nil, ErrMissingCredentials
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &httpClient{
		host:       opts.Host,
		publicKey:  opts.PublicKey,
		secretKey:  opts.SecretKey,
		httpClient: hc,
	}, nil
}

// UpsertDatasetItem POSTs to /api/public/dataset-items. Langfuse
// treats the request as idempotent on (datasetId, id) — same content
// is a no-op, different content updates in place.
func (c *httpClient) UpsertDatasetItem(
	ctx context.Context, datasetID, itemID string, content []byte,
) error {
	body, err := json.Marshal(map[string]any{
		"datasetName": datasetID,
		"id":          itemID,
		"input":       json.RawMessage(content),
	})
	if err != nil {
		return fmt.Errorf("dataset-upload: marshal body: %w", err)
	}

	url := c.host + "/api/public/dataset-items"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("dataset-upload: build request: %w", err)
	}
	req.SetBasicAuth(c.publicKey, c.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("dataset-upload: upsert %q: HTTP %d (body read failed: %w)",
				itemID, resp.StatusCode, err)
		}
		return fmt.Errorf(
			"dataset-upload: upsert %q: HTTP %d: %s",
			itemID, resp.StatusCode, string(msg),
		)
	}
	return nil
}

// doWithRetry retries 5xx responses once with a short backoff. 4xx
// failures surface immediately — they're caller errors. The Langfuse
// host comes from operator-controlled env vars; SSRF gating happens
// at the credentials boundary, not here.
func (c *httpClient) doWithRetry(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req) //nolint:gosec // operator-controlled host
	if err != nil {
		return nil, fmt.Errorf("dataset-upload: do request: %w", err)
	}
	if resp.StatusCode < 500 {
		return resp, nil
	}
	_ = resp.Body.Close()
	time.Sleep(500 * time.Millisecond)
	resp, err = c.httpClient.Do(req) //nolint:gosec // operator-controlled host
	if err != nil {
		return nil, fmt.Errorf("dataset-upload: retry request: %w", err)
	}
	return resp, nil
}
