package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const ebayUSMarketplace = "EBAY_US"

// handleOAuth replies with the static OAuth token. The shape mirrors
// the real /identity/v1/oauth2/token response so the production client
// parses it without special-casing.
func (s *Server) handleOAuth(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"access_token": s.token.value,
		"expires_in":   s.token.expiresIn,
		"token_type":   "Bearer",
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleSearch returns the active scenario's search.json envelope
// filtered by the query parameters. Multi-word `q` filters against the
// lowercased title cache via containsAllWords.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	active := s.activeScenario
	s.mu.RUnlock()

	tmpl := s.scenarios.SearchTemplate(active)
	if tmpl == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"href":          "",
			"total":         0,
			"itemSummaries": []any{},
		})
		return
	}

	marketplace := r.Header.Get("X-EBAY-C-MARKETPLACE-ID")
	if marketplace == "" {
		marketplace = ebayUSMarketplace
	}

	q := r.URL.Query()
	terms := strings.Fields(strings.ToLower(q.Get("q")))
	limit := atoiOr(q.Get("limit"), 50)
	offset := atoiOr(q.Get("offset"), 0)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	filtered := make([]itemSummary, 0, len(tmpl.items))
	for _, it := range tmpl.items {
		if len(terms) > 0 && !containsAllWords(it.titleLowercase, terms) {
			continue
		}
		filtered = append(filtered, it)
	}

	start := offset
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := filtered[start:end]

	summaries := make([]json.RawMessage, len(page))
	for i, it := range page {
		summaries[i] = it.raw
	}

	resp := map[string]any{
		"href":          fmt.Sprintf("/buy/browse/v1/item_summary/search?q=%s", q.Get("q")),
		"total":         len(filtered),
		"limit":         limit,
		"offset":        offset,
		"itemSummaries": summaries,
		"marketplaceId": marketplace,
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetItem resolves the path-bound item_id through the scenario
// registry. Misses produce an eBay-shaped 404 so the client's error
// classification kicks in the same as against the real API.
func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("item_id")
	s.mu.RLock()
	active := s.activeScenario
	s.mu.RUnlock()

	body, ok := s.scenarios.Resolve(active, itemID)
	if !ok {
		writeEbayError(w, http.StatusNotFound, 11001, "API_BROWSE",
			fmt.Sprintf("Item %q not found.", itemID))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body) //nolint:errcheck // best-effort write to client
}

// handleAnalytics replies with a synthesized rate_limit envelope from
// the current QuotaState.
func (s *Server) handleAnalytics(w http.ResponseWriter, _ *http.Request) {
	snap := s.quota.Snapshot()
	resp := map[string]any{
		"rateLimits": []map[string]any{
			{
				"apiContext": "buy",
				"apiName":    "browse",
				"resources": []map[string]any{
					{
						"name": "search",
						"rates": []map[string]any{
							{
								"limit":          snap.Limit,
								"remaining":      snap.Limit - snap.Count,
								"timeWindow":     86400,
								"reset":          time.Now().Add(snap.ResetAfter).Format(time.RFC3339),
								"timeWindowName": snap.TimeWindow,
							},
						},
					},
				},
			},
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleSetScenario flips the active scenario via JSON {"name": "..."}.
func (s *Server) handleSetScenario(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.SetActiveScenario(body.Name); err != nil {
		writeAdminError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active": body.Name})
}

// handleSetQuota overlays a QuotaSnapshot onto the live state.
func (s *Server) handleSetQuota(w http.ResponseWriter, r *http.Request) {
	var snap QuotaSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	s.quota.Apply(snap)
	writeJSON(w, http.StatusOK, s.quota.Snapshot())
}

// handleSetFault appends a fault rule. Body shape:
// {"endpoint": "/buy/browse/v1/item/.*", "latency_ms": 1000, "fail_rate": 0.1}
// An empty body clears all rules.
func (s *Server) handleSetFault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Endpoint  string  `json:"endpoint"`
		LatencyMs int     `json:"latency_ms"`
		FailRate  float64 `json:"fail_rate"`
		Clear     bool    `json:"clear"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Clear {
		s.fault.SetRules(nil)
		writeJSON(w, http.StatusOK, map[string]any{"rules": []any{}})
		return
	}
	if body.Endpoint == "" {
		writeAdminError(w, http.StatusBadRequest, "endpoint pattern required")
		return
	}
	re, err := regexp.Compile(body.Endpoint)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest,
			fmt.Sprintf("invalid endpoint regex: %v", err))
		return
	}
	rules := append(s.fault.Rules(), FaultRule{
		EndpointPattern: re,
		LatencyMs:       body.LatencyMs,
		FailRate:        body.FailRate,
	})
	s.fault.SetRules(rules)
	writeJSON(w, http.StatusOK, map[string]any{"rules": len(rules)})
}

// handleListScenarios is a small convenience for operators / tests:
// GET /admin/scenarios → {"active": "...", "available": ["default", ...]}.
func (s *Server) handleListScenarios(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	active := s.activeScenario
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"active":    active,
		"available": s.scenarios.Names(),
	})
}

// containsAllWords reports whether every term in terms appears as a
// substring of titleLower. Both inputs must already be lowercased by
// the caller so the per-request cost stays at len(terms) substring
// scans.
func containsAllWords(titleLower string, terms []string) bool {
	for _, t := range terms {
		if t == "" {
			continue
		}
		if !strings.Contains(titleLower, t) {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	// Encode errors are best-effort: client gone or response truncated;
	// nothing more useful to do here.
	_ = json.NewEncoder(w).Encode(body) //nolint:errcheck,errchkjson // mock-only fixtures
}

// atoiOr parses s as a base-10 int, returning fallback when the input
// is empty or malformed.
func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func writeEbayError(w http.ResponseWriter, status, errorID int, domain, message string) {
	resp := map[string]any{
		"errors": []map[string]any{
			{
				"errorId":  errorID,
				"domain":   domain,
				"category": "REQUEST",
				"message":  message,
			},
		},
	}
	writeJSON(w, status, resp)
}

func writeAdminError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
