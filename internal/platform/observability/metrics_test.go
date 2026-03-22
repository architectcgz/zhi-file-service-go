package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEnabledExposesPrometheusFormat(t *testing.T) {
	m := NewMetrics(true)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	res := httptest.NewRecorder()
	m.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "go_gc_duration_seconds") {
		t.Fatalf("expected go runtime metric, got: %s", res.Body.String())
	}
}

func TestMetricsDisabledReturnsNotFound(t *testing.T) {
	m := NewMetrics(false)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	res := httptest.NewRecorder()
	m.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", res.Code)
	}
}
