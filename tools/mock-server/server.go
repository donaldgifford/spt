package main

import (
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

//go:embed all:fixtures
var fixturesFS embed.FS

// ServerOptions configures NewServer. All fields except Logger have
// sane zero-value defaults; Logger is required.
type ServerOptions struct {
	Logger       *slog.Logger
	Scenario     string        // initial active scenario; defaults to "default"
	FixturesDir  string        // when non-empty, overrides the embedded fixtures
	TokenExpires time.Duration // OAuth token TTL; defaults to 2h
}

// Server holds the mutable state for a single mock-server instance:
// the loaded scenario registry, the active scenario name, the quota
// counter, and the fault injector. Routes() returns a wrapped handler
// applying the middleware chain.
type Server struct {
	mu             sync.RWMutex
	logger         *slog.Logger
	scenarios      *ScenarioRegistry
	activeScenario string
	quota          *QuotaState
	fault          *FaultInjector
	token          tokenConfig
}

type tokenConfig struct {
	value      string
	expiresIn  int64
	expiresRaw time.Duration
}

// ErrUnknownScenario is returned by SetActiveScenario when the named
// scenario isn't loaded.
var ErrUnknownScenario = errors.New("mock-server: unknown scenario")

// NewServer loads fixtures, initializes scenario/quota/fault state, and
// returns a Server ready to wire into http.Server. When opts.FixturesDir
// is set, the on-disk path replaces the embedded fixtures — useful for
// editing fixtures without a rebuild.
func NewServer(opts ServerOptions) (*Server, error) {
	if opts.Logger == nil {
		return nil, errors.New("mock-server: ServerOptions.Logger is required")
	}
	if opts.TokenExpires == 0 {
		opts.TokenExpires = 2 * time.Hour
	}

	var fsys fs.FS = fixturesFS
	root := "fixtures"
	if opts.FixturesDir != "" {
		fsys = os.DirFS(opts.FixturesDir)
		root = "."
	}

	reg, err := LoadScenarios(fsys, root)
	if err != nil {
		return nil, err
	}

	active := opts.Scenario
	if active == "" {
		active = scenarioDefault
	}
	if _, ok := reg.scenarios[active]; !ok && active != scenarioDefault {
		return nil, ErrUnknownScenario
	}

	srv := &Server{
		logger:         opts.Logger,
		scenarios:      reg,
		activeScenario: active,
		quota:          NewQuotaState(5000, 24*time.Hour),
		fault:          NewFaultInjector(),
		token: tokenConfig{
			value:      "spt-mock-bearer-token",
			expiresIn:  int64(opts.TokenExpires.Seconds()),
			expiresRaw: opts.TokenExpires,
		},
	}

	// Apply scenario-supplied quota override if present.
	if q := reg.QuotaForScenario(active); q != nil {
		srv.quota.Apply(*q)
	}
	return srv, nil
}

// ActiveScenario returns the currently selected scenario name.
func (s *Server) ActiveScenario() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeScenario
}

// SetActiveScenario flips the active scenario. Returns ErrUnknownScenario
// if name is not loaded.
func (s *Server) SetActiveScenario(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.scenarios.scenarios[name]; !ok {
		return ErrUnknownScenario
	}
	s.activeScenario = name
	if q := s.scenarios.QuotaForScenario(name); q != nil {
		s.quota.Apply(*q)
	}
	return nil
}

// Routes returns the fully composed http.Handler. The fault middleware
// wraps everything (so admin endpoints are also subject to faults if
// the operator wires a rule that matches /admin/*, which is rare but
// possible), and the quota middleware wraps the eBay-shape routes via
// per-route registration in routes.go.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /identity/v1/oauth2/token", s.handleOAuth)
	mux.Handle("GET /buy/browse/v1/item_summary/search",
		s.quota.Middleware(http.HandlerFunc(s.handleSearch)))
	mux.Handle("GET /buy/browse/v1/item/{item_id}",
		s.quota.Middleware(http.HandlerFunc(s.handleGetItem)))
	mux.HandleFunc("GET /developer/analytics/v1_beta/rate_limit/", s.handleAnalytics)

	mux.HandleFunc("POST /admin/scenario", s.handleSetScenario)
	mux.HandleFunc("POST /admin/quota", s.handleSetQuota)
	mux.HandleFunc("POST /admin/fault", s.handleSetFault)
	mux.HandleFunc("GET /admin/scenarios", s.handleListScenarios)

	return s.fault.Middleware(mux)
}
