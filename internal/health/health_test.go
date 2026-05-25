package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestHealthzAlwaysOK(t *testing.T) {
	s := New(prometheus.NewRegistry())

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
	s.handleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body: got %q want %q", rec.Body.String(), "ok")
	}
}

func TestReadyzNoProbesReturnsOK(t *testing.T) {
	s := New(prometheus.NewRegistry())

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
	s.handleReadyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d want 200", rec.Code)
	}
}

func TestReadyzAllProbesPass(t *testing.T) {
	s := New(prometheus.NewRegistry())
	s.RegisterReadiness("postgres", func(context.Context) error { return nil })
	s.RegisterReadiness("valkey", func(context.Context) error { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
	s.handleReadyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d want 200", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["postgres"] != "ok" || body["valkey"] != "ok" {
		t.Errorf("body: %v", body)
	}
}

func TestReadyzFailingProbeReturns503(t *testing.T) {
	s := New(prometheus.NewRegistry())
	s.RegisterReadiness("postgres", func(context.Context) error { return nil })
	s.RegisterReadiness("valkey", func(context.Context) error { return errors.New("connection refused") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
	s.handleReadyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d want 503", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["postgres"] != "ok" {
		t.Errorf("postgres: got %q", body["postgres"])
	}
	if !strings.HasPrefix(body["valkey"], "error:") || !strings.Contains(body["valkey"], "connection refused") {
		t.Errorf("valkey: got %q", body["valkey"])
	}
}

func TestReadyzProbeTimeoutFires(t *testing.T) {
	s := New(prometheus.NewRegistry())
	s.RegisterReadiness("slow", func(ctx context.Context) error {
		// Block longer than the probeTimeout (2s). The probe should
		// see ctx cancel and return context.DeadlineExceeded.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	})

	start := time.Now()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", http.NoBody)
	s.handleReadyz(rec, req)
	elapsed := time.Since(start)

	if elapsed > probeTimeout+500*time.Millisecond {
		t.Errorf("probe timeout did not fire: elapsed=%v", elapsed)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d want 503", rec.Code)
	}
}

func TestMetricsExposesPrometheusFormat(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spt_health_test_total",
		Help: "test",
	}))
	s := New(reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, "127.0.0.1:0") }()

	// Wait for the listener; Serve installs httpSrv before its goroutine
	// calls ListenAndServe, but the OS-assigned port comes via the
	// underlying net.Listener which Serve doesn't expose. Easiest reliable
	// path: read the bound address from the listener via a tiny retry.
	addr := waitForListener(t, s)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/metrics", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "spt_health_test_total") {
		t.Errorf("metric not exposed: %s", body)
	}

	cancel()
	<-errCh
}

func waitForListener(t *testing.T, s *Server) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a := s.Addr(); a != "" {
			return a
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("listener did not bind within deadline")
	return ""
}

func TestServeStopsOnContextCancel(t *testing.T) {
	s := New(prometheus.NewRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, "127.0.0.1:0") }()

	// Give the listener a moment to bind.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrServerClosed) {
			t.Errorf("Serve returned: %v, want ErrServerClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after 2s")
	}
}

func TestRegisterReadinessIsThreadSafe(_ *testing.T) {
	// Sole purpose: race-detector smoke test for RegisterReadiness.
	s := New(prometheus.NewRegistry())
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.RegisterReadiness("p"+string(rune('a'+i%26)), func(context.Context) error { return nil })
		}(i)
	}
	wg.Wait()
}
