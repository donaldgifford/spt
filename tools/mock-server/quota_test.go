package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuotaState_IncrementAndStamp(t *testing.T) {
	q := NewQuotaState(100, time.Hour)
	handler := q.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))

	require.Equal(t, "100", rec.Header().Get("X-EBAY-API-Call-Limit"))
	require.Equal(t, "1", rec.Header().Get("X-EBAY-API-Calls-Made"))
	require.Equal(t, "99", rec.Header().Get("X-EBAY-API-Calls-Remaining"))
}

func TestQuotaState_ErrorPathDoesNotCount(t *testing.T) {
	q := NewQuotaState(100, time.Hour)
	handler := q.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))

	// No stamp on non-2xx, and count must still be 0.
	require.Empty(t, rec.Header().Get("X-EBAY-API-Calls-Made"))
	require.Equal(t, int64(0), q.Snapshot().Count)
}

func TestQuotaState_ConcurrentIncrements(t *testing.T) {
	q := NewQuotaState(10000, time.Hour)
	handler := q.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const goroutines, perGoroutine = 50, 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range perGoroutine {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec,
					httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))
			}
		}()
	}
	wg.Wait()

	got := q.Snapshot().Count
	require.Equal(t, int64(goroutines*perGoroutine), got)
}

func TestQuotaState_ApplyOverrides(t *testing.T) {
	q := NewQuotaState(100, time.Hour)
	tru := true
	q.Apply(QuotaSnapshot{Count: 50, Limit: 200, AutoIncr: &tru})

	snap := q.Snapshot()
	require.Equal(t, int64(50), snap.Count)
	require.Equal(t, int64(200), snap.Limit)
	require.NotNil(t, snap.AutoIncr)
	require.True(t, *snap.AutoIncr)

	// And the Middleware should pick up the new limit.
	handler := q.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))
	limit, _ := strconv.Atoi(rec.Header().Get("X-EBAY-API-Call-Limit"))
	require.Equal(t, 200, limit)
}
