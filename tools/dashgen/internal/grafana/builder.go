// Package grafana is a thin in-house builder that emits Grafana
// dashboard JSON via typed Go. ~200 LOC target; only the panel/row
// types we actually use — no dependency on a third-party Grafana SDK.
// Resolved Decision #9 in IMPL-0002 Phase 7. Revisit if dashboard
// count grows past ~10 distinct dashboards.
package grafana

// DatasourcePrometheus is the hardcoded datasource for v1 dashboards
// (Resolved Decision #10 in IMPL-0002). Datasource templating becomes
// a follow-up if a multi-cluster user requests it.
const DatasourcePrometheus = "Prometheus"

// Dashboard is the top-level Grafana dashboard shape, minus fields
// the chart doesn't customize.
type Dashboard struct {
	Title         string   `json:"title"`
	UID           string   `json:"uid"`
	Tags          []string `json:"tags,omitempty"`
	Timezone      string   `json:"timezone,omitempty"`
	Refresh       string   `json:"refresh,omitempty"`
	SchemaVersion int      `json:"schemaVersion"`
	Version       int      `json:"version"`
	Panels        []Panel  `json:"panels"`
}

// Panel is one Grafana panel.
type Panel struct {
	Title       string      `json:"title"`
	Type        string      `json:"type"`
	ID          int         `json:"id"`
	GridPos     GridPos     `json:"gridPos"`
	Datasource  Datasource  `json:"datasource"`
	Targets     []Target    `json:"targets"`
	FieldConfig FieldConfig `json:"fieldConfig,omitempty"`
}

// GridPos sizes and positions a panel.
type GridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

// Datasource selector — uid:"prometheus" pins the chart-installed
// Prometheus datasource.
type Datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

// Target is one PromQL expression in a panel.
type Target struct {
	Expr         string `json:"expr"`
	LegendFormat string `json:"legendFormat,omitempty"`
	RefID        string `json:"refId"`
}

// FieldConfig is the minimal shape needed for unit/decimals/thresholds.
type FieldConfig struct {
	Defaults FieldDefaults `json:"defaults"`
}

// FieldDefaults is the per-panel defaults block.
type FieldDefaults struct {
	Unit     string `json:"unit,omitempty"`
	Decimals int    `json:"decimals,omitempty"`
}

// New returns a Dashboard skeleton with title, UID, and the standard
// schema/version metadata.
func New(title, uid string) Dashboard {
	return Dashboard{
		Title:         title,
		UID:           uid,
		Tags:          []string{"spt"},
		Timezone:      "browser",
		Refresh:       "30s",
		SchemaVersion: 38,
		Version:       1,
	}
}

// AddPanel appends a panel to the dashboard. Panel is large enough
// (~150 bytes) that we accept a pointer; callers commonly construct
// the Panel inline though, so we provide both flavors.
func (d *Dashboard) AddPanel(p *Panel) {
	d.Panels = append(d.Panels, *p)
}

// PromTarget is a small convenience wrapper for the most common case:
// a single PromQL expression with a legend.
func PromTarget(expr, legend string) Target {
	return Target{Expr: expr, LegendFormat: legend, RefID: "A"}
}

// PromDatasource returns the standard hardcoded Prometheus datasource
// reference used by every spt v1 dashboard.
func PromDatasource() Datasource {
	return Datasource{Type: "prometheus", UID: "prometheus"}
}
