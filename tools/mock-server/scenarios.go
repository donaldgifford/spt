package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"time"
)

// Scenario is a loaded fixture set keyed by item ID, plus an optional
// search response template and quota override.
type Scenario struct {
	Name   string
	Items  map[string]json.RawMessage // item_id → full GetItem response body
	Search json.RawMessage            // search response template; may be nil if scenario doesn't override
	Quota  *QuotaSnapshot             // optional initial state override
}

// itemSummary captures the subset of item_summary fields the search
// endpoint filters on. Populated at fixture load and reused per request.
type itemSummary struct {
	raw            json.RawMessage
	title          string
	titleLowercase string
	itemID         string
}

// scenarioSearch is the parsed search response template with each
// itemSummary expanded for filter-time use.
type scenarioSearch struct {
	envelope map[string]any
	items    []itemSummary
}

// ScenarioRegistry holds every loaded scenario plus the default
// fallback. Resolve walks the active scenario first then defaults.
type ScenarioRegistry struct {
	scenarios       map[string]*Scenario
	defaultScenario *Scenario
	// searchByScenario caches the parsed search template per scenario,
	// so handleSearch doesn't re-decode on every request.
	searchByScenario map[string]*scenarioSearch
}

// QuotaSnapshot mirrors the optional `quota.json` body a scenario can
// ship to seed the quota state at scenario activation.
type QuotaSnapshot struct {
	Count      int64         `json:"count"`
	Limit      int64         `json:"limit"`
	ResetAfter time.Duration `json:"reset_after"`
	TimeWindow string        `json:"time_window"`
	AutoIncr   *bool         `json:"auto_incr,omitempty"`
}

// scenarioDefault is the well-known fallback scenario name. Every
// fixtures root must define it.
const scenarioDefault = "default"

// ErrFixturesNotFound indicates the fixtures root has no usable
// scenarios (no default/ subdirectory).
var ErrFixturesNotFound = errors.New("mock-server: fixtures root missing 'default' scenario")

// LoadScenarios walks every immediate subdirectory of root and loads it
// as a scenario. The 'default' scenario is required.
func LoadScenarios(fsys fs.FS, root string) (*ScenarioRegistry, error) {
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, fmt.Errorf("mock-server: read fixtures dir %q: %w", root, err)
	}

	reg := &ScenarioRegistry{
		scenarios:        make(map[string]*Scenario),
		searchByScenario: make(map[string]*scenarioSearch),
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		scenarioPath := path.Join(root, entry.Name())
		s, err := loadScenarioDir(fsys, scenarioPath, entry.Name())
		if err != nil {
			return nil, err
		}
		reg.scenarios[entry.Name()] = s
		if entry.Name() == scenarioDefault {
			reg.defaultScenario = s
		}
		if search, ok := parseSearch(s.Search); ok {
			reg.searchByScenario[entry.Name()] = search
		}
	}

	if reg.defaultScenario == nil {
		return nil, ErrFixturesNotFound
	}
	return reg, nil
}

func loadScenarioDir(fsys fs.FS, dir, name string) (*Scenario, error) {
	s := &Scenario{Name: name, Items: make(map[string]json.RawMessage)}

	if data, err := fs.ReadFile(fsys, path.Join(dir, "search.json")); err == nil {
		s.Search = data
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("mock-server: read %s/search.json: %w", dir, err)
	}

	if data, err := fs.ReadFile(fsys, path.Join(dir, "quota.json")); err == nil {
		var snap QuotaSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			return nil, fmt.Errorf("mock-server: parse %s/quota.json: %w", dir, err)
		}
		s.Quota = &snap
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("mock-server: read %s/quota.json: %w", dir, err)
	}

	itemsDir := path.Join(dir, "items")
	items, err := fs.ReadDir(fsys, itemsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("mock-server: read items dir %s: %w", itemsDir, err)
	}
	for _, it := range items {
		if it.IsDir() || !strings.HasSuffix(it.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(itemsDir, it.Name()))
		if err != nil {
			return nil, fmt.Errorf("mock-server: read item %s: %w", it.Name(), err)
		}
		key := decodeItemIDFromFilename(it.Name())
		s.Items[key] = data
	}
	return s, nil
}

// decodeItemIDFromFilename converts a fixture filename like
// "v1%7C151234567890%7C0.json" to the eBay item_id
// "v1|151234567890|0". eBay item IDs use `|` as a path separator,
// which Go's embed package forbids in filenames, so fixtures are
// URL-encoded on disk and decoded at load time.
func decodeItemIDFromFilename(name string) string {
	stripped := strings.TrimSuffix(name, ".json")
	decoded, err := url.PathUnescape(stripped)
	if err != nil {
		return stripped
	}
	return decoded
}

// Resolve walks the active scenario then defaults. Returns the item
// body and true on hit, nil/false on miss.
func (r *ScenarioRegistry) Resolve(active, itemID string) (json.RawMessage, bool) {
	if s, ok := r.scenarios[active]; ok {
		if item, ok := s.Items[itemID]; ok {
			return item, true
		}
	}
	if r.defaultScenario != nil {
		if item, ok := r.defaultScenario.Items[itemID]; ok {
			return item, true
		}
	}
	return nil, false
}

// QuotaForScenario returns the per-scenario quota override, or nil.
func (r *ScenarioRegistry) QuotaForScenario(name string) *QuotaSnapshot {
	if s, ok := r.scenarios[name]; ok {
		return s.Quota
	}
	return nil
}

// Names returns the loaded scenario names; used by /admin/scenarios.
func (r *ScenarioRegistry) Names() []string {
	names := make([]string, 0, len(r.scenarios))
	for n := range r.scenarios {
		names = append(names, n)
	}
	return names
}

// SearchTemplate returns the parsed search template for active, falling
// back to default. Returns nil if neither has one — handleSearch then
// synthesizes a minimal envelope.
func (r *ScenarioRegistry) SearchTemplate(active string) *scenarioSearch {
	if s, ok := r.searchByScenario[active]; ok {
		return s
	}
	return r.searchByScenario[scenarioDefault]
}

// parseSearch decodes a fixtures search.json envelope and pre-computes
// per-item lowercased titles for cheap filter matching.
func parseSearch(raw json.RawMessage) (*scenarioSearch, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, false
	}
	rawItems, ok := envelope["itemSummaries"].([]any)
	if !ok {
		return &scenarioSearch{envelope: envelope}, true
	}
	items := make([]itemSummary, 0, len(rawItems))
	for _, raw := range rawItems {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		title, ok := m["title"].(string)
		if !ok {
			continue
		}
		itemID, ok := m["itemId"].(string)
		if !ok {
			continue
		}
		bytes, err := json.Marshal(m)
		if err != nil {
			continue
		}
		items = append(items, itemSummary{
			raw:            bytes,
			title:          title,
			titleLowercase: strings.ToLower(title),
			itemID:         itemID,
		})
	}
	return &scenarioSearch{envelope: envelope, items: items}, true
}
