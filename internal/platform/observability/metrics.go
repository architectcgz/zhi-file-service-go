package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	enabled  bool
	registry *prometheus.Registry
	handler  http.Handler
}

func NewMetrics(enabled bool) *Metrics {
	if !enabled {
		return &Metrics{enabled: false, handler: http.NotFoundHandler()}
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return &Metrics{
		enabled:  true,
		registry: registry,
		handler:  promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}
}

func (m *Metrics) Enabled() bool {
	if m == nil {
		return false
	}
	return m.enabled
}

func (m *Metrics) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}
	return m.registry
}

func (m *Metrics) Handler() http.Handler {
	if m == nil || m.handler == nil {
		return http.NotFoundHandler()
	}
	return m.handler
}
