package main

import (
	"github.com/donaldgifford/spt/tools/dashgen/internal/grafana"
)

// DashboardSpec ties a dashboard name to its file path and builder
// function. The DashboardSpecs() registry enumerates everything
// dashgen writes.
type DashboardSpec struct {
	Name  string
	File  string
	Build func() any
}

// DashboardSpecs returns the per-dashboard builders. Add a new entry
// here to ship a new dashboard with the chart.
func DashboardSpecs() []DashboardSpec {
	return []DashboardSpec{
		{Name: "API overview", File: "dashboards/api-overview.json", Build: buildAPIOverview},
		{Name: "Worker pools", File: "dashboards/worker-pools.json", Build: buildWorkerPools},
		{Name: "eBay quota", File: "dashboards/ebay-quota.json", Build: buildEbayQuota},
		{Name: "Alerts", File: "dashboards/alerts.json", Build: buildAlertsDashboard},
	}
}

func buildAPIOverview() any {
	d := grafana.New("spt — API overview", "spt-api-overview")
	d.AddPanel(&grafana.Panel{
		Title:      "Request rate (per handler)",
		Type:       "timeseries",
		ID:         1,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 0, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`sum by (handler) (rate(http_requests_total{job="spt-api"}[5m]))`,
			"{{ handler }}",
		)},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Latency p50 / p95 / p99",
		Type:       "timeseries",
		ID:         2,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 12, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{
			grafana.PromTarget(`histogram_quantile(0.50, sum by (le) (rate(http_request_duration_seconds_bucket{job="spt-api"}[5m])))`, "p50"),
			grafana.PromTarget(`histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{job="spt-api"}[5m])))`, "p95"),
			grafana.PromTarget(`histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{job="spt-api"}[5m])))`, "p99"),
		},
		FieldConfig: grafana.FieldConfig{Defaults: grafana.FieldDefaults{Unit: "s"}},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Error rate (5xx)",
		Type:       "timeseries",
		ID:         3,
		GridPos:    grafana.GridPos{H: 8, W: 24, X: 0, Y: 8},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`sum by (handler) (rate(http_requests_total{job="spt-api",status=~"5.."}[5m]))`,
			"{{ handler }}",
		)},
	})
	return d
}

func buildWorkerPools() any {
	d := grafana.New("spt — Worker pools", "spt-worker-pools")
	d.AddPanel(&grafana.Panel{
		Title:      "In-flight tasks (per pool)",
		Type:       "timeseries",
		ID:         1,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 0, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`sum by (pool) (spt_worker_pool_inflight)`,
			"{{ pool }}",
		)},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Queue depth (per pool)",
		Type:       "timeseries",
		ID:         2,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 12, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`sum by (pool) (spt_worker_pool_queue_depth)`,
			"{{ pool }}",
		)},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Task duration p95",
		Type:       "timeseries",
		ID:         3,
		GridPos:    grafana.GridPos{H: 8, W: 24, X: 0, Y: 8},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`histogram_quantile(0.95, sum by (le, pool) (rate(spt_worker_task_duration_seconds_bucket[5m])))`,
			"{{ pool }}",
		)},
		FieldConfig: grafana.FieldConfig{Defaults: grafana.FieldDefaults{Unit: "s"}},
	})
	return d
}

func buildEbayQuota() any {
	d := grafana.New("spt — eBay quota", "spt-ebay-quota")
	d.AddPanel(&grafana.Panel{
		Title:      "API calls (rate)",
		Type:       "timeseries",
		ID:         1,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 0, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`sum(rate(spt_ebay_api_calls_total[5m]))`,
			"calls/s",
		)},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Quota remaining",
		Type:       "stat",
		ID:         2,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 12, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`spt_ebay_quota_remaining`,
			"remaining",
		)},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Quota exhausted",
		Type:       "stat",
		ID:         3,
		GridPos:    grafana.GridPos{H: 8, W: 24, X: 0, Y: 8},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`spt_ebay_quota_exhausted`,
			"exhausted",
		)},
	})
	return d
}

func buildAlertsDashboard() any {
	d := grafana.New("spt — Alerts", "spt-alerts")
	d.AddPanel(&grafana.Panel{
		Title:      "Open alerts",
		Type:       "stat",
		ID:         1,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 0, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`sum(spt_alerts_open_total)`, "open",
		)},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Stale alerts",
		Type:       "stat",
		ID:         2,
		GridPos:    grafana.GridPos{H: 8, W: 12, X: 12, Y: 0},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{grafana.PromTarget(
			`sum(spt_alerts_stale_total)`, "stale",
		)},
	})
	d.AddPanel(&grafana.Panel{
		Title:      "Reconciliation rate (alerts vs bulk)",
		Type:       "timeseries",
		ID:         3,
		GridPos:    grafana.GridPos{H: 8, W: 24, X: 0, Y: 8},
		Datasource: grafana.PromDatasource(),
		Targets: []grafana.Target{
			grafana.PromTarget(`rate(spt_reconcile_alerts_total[5m])`, "alerts"),
			grafana.PromTarget(`rate(spt_reconcile_bulk_total[5m])`, "bulk"),
		},
	})
	return d
}
