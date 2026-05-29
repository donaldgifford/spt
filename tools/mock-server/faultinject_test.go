package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newPassthrough() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
}

func TestFaultInjector_PassthroughWhenNoMatch(t *testing.T) {
	f := NewFaultInjector()
	f.SetRules([]FaultRule{
		{EndpointPattern: regexp.MustCompile(`/never`), LatencyMs: 0, FailRate: 1.0},
	})

	rec := httptest.NewRecorder()
	f.Middleware(newPassthrough()).ServeHTTP(rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/buy/browse/v1/item/x", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
}

func TestFaultInjector_FailRateOne(t *testing.T) {
	f := NewFaultInjector()
	f.SetRules([]FaultRule{
		{EndpointPattern: regexp.MustCompile(`/buy/browse/v1/item/.*`), FailRate: 1.0},
	})

	rec := httptest.NewRecorder()
	f.Middleware(newPassthrough()).ServeHTTP(rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/buy/browse/v1/item/x", nil))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "fault-injected")
}

func TestFaultInjector_Latency(t *testing.T) {
	f := NewFaultInjector()
	f.SetRules([]FaultRule{
		{EndpointPattern: regexp.MustCompile(`/lat`), LatencyMs: 30},
	})

	start := time.Now()
	rec := httptest.NewRecorder()
	f.Middleware(newPassthrough()).ServeHTTP(rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/lat", nil))
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, rec.Code)
	require.GreaterOrEqual(t, elapsed, 30*time.Millisecond,
		"middleware should sleep at least the configured LatencyMs")
}

func TestFaultInjector_FirstMatchWins(t *testing.T) {
	// Two overlapping rules; the first one declared (LatencyMs: 0,
	// FailRate: 0) must win and let the request through. If the second
	// one matched instead the response would be 503.
	f := NewFaultInjector()
	f.SetRules([]FaultRule{
		{EndpointPattern: regexp.MustCompile(`/buy/.*`)},
		{EndpointPattern: regexp.MustCompile(`/buy/browse/v1/item/.*`), FailRate: 1.0},
	})

	rec := httptest.NewRecorder()
	f.Middleware(newPassthrough()).ServeHTTP(rec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/buy/browse/v1/item/x", nil))

	require.Equal(t, http.StatusOK, rec.Code)
}
