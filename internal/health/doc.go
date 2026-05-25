// Package health serves /healthz, /readyz, and /metrics on the admin port
// (default :9090). Every long-running role registers its dependency
// readiness probes at construction time. See DESIGN-0001.
package health
