package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// FaultRule describes a single fault-injection rule. EndpointPattern
// matches request URL paths; LatencyMs (when > 0) sleeps before the
// downstream handler runs; FailRate (0.0-1.0) replies 503 with an
// eBay-shaped error envelope at that probability.
type FaultRule struct {
	EndpointPattern *regexp.Regexp
	LatencyMs       int
	FailRate        float64
}

// FaultInjector holds the live rule set. Rules are appended (multiple
// rules may match a single request — the first matching one wins to
// keep semantics predictable).
type FaultInjector struct {
	mu    sync.RWMutex
	rules []FaultRule
}

// NewFaultInjector returns an empty injector with no rules — all
// requests pass through.
func NewFaultInjector() *FaultInjector {
	return &FaultInjector{}
}

// SetRules replaces the rule set atomically.
func (f *FaultInjector) SetRules(rules []FaultRule) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules = rules
}

// Rules returns a snapshot of the current rule set.
func (f *FaultInjector) Rules() []FaultRule {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]FaultRule, len(f.rules))
	copy(out, f.rules)
	return out
}

// Middleware applies the first matching rule (if any) to each request.
// Latency is applied before forwarding; FailRate is rolled per-request.
func (f *FaultInjector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rule, ok := f.match(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if rule.LatencyMs > 0 {
			time.Sleep(time.Duration(rule.LatencyMs) * time.Millisecond)
		}
		if rule.FailRate > 0 && rand.Float64() < rule.FailRate { //nolint:gosec // mock tooling; math/rand is fine
			writeFaultError(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (f *FaultInjector) match(path string) (FaultRule, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, r := range f.rules {
		if r.EndpointPattern != nil && r.EndpointPattern.MatchString(path) {
			return r, true
		}
	}
	return FaultRule{}, false
}

// writeFaultError responds with an eBay-shaped 503. Keep the body shape
// aligned with what the real API returns under transient outages so
// production error handling exercises the right code paths in tests.
func writeFaultError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprint(w, `{"errors":[{`+ //nolint:errcheck // best-effort fault response
		`"errorId":50001,`+
		`"domain":"API_BROWSE",`+
		`"category":"APPLICATION",`+
		`"message":"Service unavailable (fault-injected by mock-server).",`+
		`"longMessage":"Injected by mock-server FaultInjector."}]}`)
}
