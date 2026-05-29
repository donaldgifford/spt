package obs

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// NewRegistry returns a Prometheus registry pre-loaded with the Go
// runtime and process collectors. Roles register their own metrics on
// top of this base; the /metrics handler in internal/health serves
// the registry over HTTP.
func NewRegistry() *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return r
}

// WithInstance returns a Registerer that prepends an `instance` label
// to every metric registered through it. spt scales horizontally
// (DESIGN-0005 — Multi-instance scaling); the instance label is what
// lets Grafana queries distinguish replicas.
func WithInstance(r prometheus.Registerer, instance string) prometheus.Registerer {
	return prometheus.WrapRegistererWith(prometheus.Labels{"instance": instance}, r)
}
