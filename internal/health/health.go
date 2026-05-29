// Package health serves /healthz, /readyz, and /metrics on the admin
// port. Roles register dependency readiness probes at construction
// time; the server invokes them when /readyz is hit. See DESIGN-0001
// § "Health and metrics endpoints".
package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ErrServerClosed is returned by Server.Serve when the server stops
// cleanly via Shutdown. Callers can use errors.Is to discriminate
// from real listen/accept failures.
var ErrServerClosed = errors.New("health: server closed")

// Probe is a single readiness check. Implementations should be cheap
// (a connection ping, not a query) and respect the supplied ctx
// deadline. A nil return means the dependency is healthy.
type Probe func(ctx context.Context) error

// probeTimeout caps each individual probe; the /readyz handler shares
// a parent deadline so the whole request bounds at this value too.
const probeTimeout = 2 * time.Second

// Server owns the admin http.Server, its readiness probes, and the
// Prometheus registry it exposes via /metrics. Use New to construct,
// register probes via RegisterReadiness, then call Serve.
type Server struct {
	registry *prometheus.Registry

	mu     sync.RWMutex
	probes map[string]Probe

	// addr is the actual address the listener bound to. Differs from
	// the configured addr when ":0" was passed; tests use this to
	// build URLs against an OS-assigned port.
	addrMu sync.RWMutex
	addr   string

	httpSrv *http.Server
}

// Addr returns the address the listener actually bound to. Empty
// before Serve has installed its listener; intended for tests that
// need to dial a Server bound to ":0".
func (s *Server) Addr() string {
	s.addrMu.RLock()
	defer s.addrMu.RUnlock()
	return s.addr
}

// New returns a Server backed by registry for /metrics. Callers
// typically pass obs.NewRegistry() so the Go runtime + process
// collectors are pre-loaded.
func New(registry *prometheus.Registry) *Server {
	return &Server{
		registry: registry,
		probes:   make(map[string]Probe),
	}
}

// RegisterReadiness attaches a probe to the /readyz handler under
// name. Replacing a probe is allowed (useful for reconfiguration);
// concurrent calls are safe.
func (s *Server) RegisterReadiness(name string, p Probe) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probes[name] = p
}

// Serve binds addr and blocks until ctx cancels or the listener
// errors. The returned error is ErrServerClosed for the cancel path,
// the listen error otherwise. Each role calls Serve from a goroutine
// inside Run; cancel ctx (typically via the role's parent ctx)
// triggers a bounded shutdown.
//
// Serve opens the listener synchronously before returning to its own
// goroutine so Addr() is populated by the time callers see the
// listener active. This matters for tests passing ":0".
func (s *Server) Serve(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("health: listen %s: %w", addr, err)
	}

	s.addrMu.Lock()
	s.addr = lis.Addr().String()
	s.addrMu.Unlock()

	s.httpSrv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := s.httpSrv.Serve(lis)
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("health: shutdown: %w", err)
		}
		return ErrServerClosed
	}
}

func (*Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		// Client closed the connection before we finished writing the
		// 2-byte body — uninteresting, log nothing.
		_ = err
	}
}

// readyResponse is the JSON body returned by /readyz. Status is "ok"
// or "error: <message>" per probe so operators can see which
// dependency failed without scraping logs.
type readyResponse map[string]string

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	names := make([]string, 0, len(s.probes))
	for name := range s.probes {
		names = append(names, name)
	}
	probes := make(map[string]Probe, len(s.probes))
	for name, p := range s.probes {
		probes[name] = p
	}
	s.mu.RUnlock()

	sort.Strings(names)

	resp := make(readyResponse, len(names))
	allOK := true
	for _, name := range names {
		ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
		err := probes[name](ctx)
		cancel()
		if err != nil {
			resp[name] = "error: " + err.Error()
			allOK = false
		} else {
			resp[name] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if allOK {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Encoding a tiny map[string]string into an http.ResponseWriter
		// only fails when the client disconnected mid-write — nothing
		// useful to do beyond moving on.
		_ = err
	}
}
