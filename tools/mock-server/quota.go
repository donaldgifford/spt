package main

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// QuotaState is the mock's mutable rate-limit counter, served via
// /developer/analytics/v1_beta/rate_limit/ and stamped onto every
// successful eBay-shape response via Middleware.
type QuotaState struct {
	mu         sync.Mutex
	count      int64
	limit      int64
	resetAt    time.Time
	timeWindow string
	autoIncr   bool
}

// NewQuotaState builds a fresh quota with the given daily limit and
// reset window. AutoIncr defaults to true so test callers don't have
// to manually call /admin/quota for every request.
func NewQuotaState(limit int64, window time.Duration) *QuotaState {
	return &QuotaState{
		limit:      limit,
		resetAt:    time.Now().Add(window),
		timeWindow: "DAY",
		autoIncr:   true,
	}
}

// Snapshot returns a copy of the current quota counters for read-only
// callers (the analytics endpoint and tests).
func (q *QuotaState) Snapshot() QuotaSnapshot {
	q.mu.Lock()
	defer q.mu.Unlock()
	autoIncr := q.autoIncr
	return QuotaSnapshot{
		Count:      q.count,
		Limit:      q.limit,
		ResetAfter: time.Until(q.resetAt),
		TimeWindow: q.timeWindow,
		AutoIncr:   &autoIncr,
	}
}

// Apply overlays a snapshot onto the live state. Used at scenario
// activation and by /admin/quota. Zero-valued fields in the snapshot
// leave the existing value intact, except AutoIncr which is a pointer
// and only writes when non-nil.
func (q *QuotaState) Apply(snap QuotaSnapshot) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if snap.Limit > 0 {
		q.limit = snap.Limit
	}
	q.count = snap.Count
	if snap.ResetAfter > 0 {
		q.resetAt = time.Now().Add(snap.ResetAfter)
	}
	if snap.TimeWindow != "" {
		q.timeWindow = snap.TimeWindow
	}
	if snap.AutoIncr != nil {
		q.autoIncr = *snap.AutoIncr
	}
}

// incrementAndStamp increments the counter (when autoIncr is on) and
// writes the three X-EBAY-* headers onto the response.
func (q *QuotaState) incrementAndStamp(w http.ResponseWriter) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.autoIncr {
		q.count++
	}
	remaining := q.limit - q.count
	if remaining < 0 {
		remaining = 0
	}
	w.Header().Set("X-EBAY-API-Call-Limit", strconv.FormatInt(q.limit, 10))
	w.Header().Set("X-EBAY-API-Calls-Made", strconv.FormatInt(q.count, 10))
	w.Header().Set("X-EBAY-API-Calls-Remaining", strconv.FormatInt(remaining, 10))
}

// Middleware wraps an http.Handler, stamping quota headers and
// incrementing the counter on every successful (2xx) response. We use
// a response writer wrapper so we only count successful calls — error
// paths shouldn't burn quota.
func (q *QuotaState) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		if rw.status >= 200 && rw.status < 300 {
			q.incrementAndStamp(w)
		}
	})
}

// statusRecorder wraps http.ResponseWriter to capture the response
// code so Middleware can decide whether to count the call.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}
