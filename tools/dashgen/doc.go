// Package main hosts the spt-dashgen tool. Generates Grafana dashboard
// JSON and Prometheus rule YAML into charts/spt/files/ so the Helm
// chart ships them as ConfigMaps / PrometheusRule resources.
//
// `-validate` mode regenerates to memory and diffs against the
// on-disk files — CI uses this to fail on dashboard drift.
//
// See DESIGN-0006 "dashgen" and IMPL-0002 Phase 7.
package main
