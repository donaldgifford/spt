// Package obs wires structured logging (log/slog), distributed tracing
// (OTel with the agent/system span-category split), and Prometheus
// metrics. Initialized once per role's Run via Setup. See DESIGN-0001
// and DESIGN-0005 for the trace routing and metric label conventions.
package obs
