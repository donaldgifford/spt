package obs

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewRegistryHasGoAndProcessCollectors(t *testing.T) {
	r := NewRegistry()

	mfs, err := r.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	var hasGoMetric, hasProcessMetric bool
	for _, mf := range mfs {
		name := mf.GetName()
		if strings.HasPrefix(name, "go_") {
			hasGoMetric = true
		}
		if strings.HasPrefix(name, "process_") {
			hasProcessMetric = true
		}
	}
	if !hasGoMetric {
		t.Error("Go runtime collector missing")
	}
	if !hasProcessMetric {
		t.Error("process collector missing")
	}
}

func TestWithInstanceAttachesLabel(t *testing.T) {
	r := prometheus.NewRegistry()
	reg := WithInstance(r, "spt-api-0")

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spt_test_total",
		Help: "test counter",
	})
	reg.MustRegister(counter)
	counter.Inc()

	if got := testutil.ToFloat64(counter); got != 1 {
		t.Errorf("counter value: got %v want 1", got)
	}

	mfs, err := r.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	for _, mf := range mfs {
		if mf.GetName() != "spt_test_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			var found bool
			for _, l := range m.GetLabel() {
				if l.GetName() == "instance" && l.GetValue() == "spt-api-0" {
					found = true
				}
			}
			if !found {
				t.Errorf("instance label missing on registered metric: %v", m.GetLabel())
			}
		}
	}
}
