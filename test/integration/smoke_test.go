//go:build integration

// Package integration carries the end-to-end smoke checks that run
// against the Compose stack in test/integration/docker-compose.yml.
// Build-tagged so `go test ./...` (and CI's fast path) skip them;
// `just test-integration` (or any `go test -tags=integration`) opts in.
package integration

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	postgresAddr    = "127.0.0.1:55432"
	valkeyAddr      = "127.0.0.1:56379"
	meilisearchURL  = "http://127.0.0.1:57700/health"
	healthcheckWait = 30 * time.Second
)

func TestComposeStackReachable(t *testing.T) {
	t.Run("postgres", func(t *testing.T) { tcpReachable(t, postgresAddr) })
	t.Run("valkey", func(t *testing.T) { tcpReachable(t, valkeyAddr) })
	t.Run("meilisearch", func(t *testing.T) { httpReachable(t, meilisearchURL) })
}

func tcpReachable(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(healthcheckWait)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, lastErr, "TCP connect %s never succeeded", addr)
}

func httpReachable(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(healthcheckWait)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			cancel()
			t.Fatalf("build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = errStatus(resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, lastErr, "HTTP GET %s never returned 200", url)
}

type errStatus int

func (e errStatus) Error() string { return http.StatusText(int(e)) }
