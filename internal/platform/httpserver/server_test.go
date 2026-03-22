package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
)

func TestProbesAndMetricsEndpoint(t *testing.T) {
	var readyErr error = errors.New("not ready")
	metrics := observability.NewMetrics(true)

	s := New(Options{
		ServiceName: "platform-test",
		HTTP: config.HTTPConfig{
			Port:         8080,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			IdleTimeout:  time.Second,
		},
		Ready: func(context.Context) error {
			return readyErr
		},
		MetricsHandler: metrics.Handler(),
	})

	h := s.Handler()

	liveReq := httptest.NewRequest(http.MethodGet, "/live", nil)
	liveRes := httptest.NewRecorder()
	h.ServeHTTP(liveRes, liveReq)
	if liveRes.Code != http.StatusOK {
		t.Fatalf("unexpected /live status: %d", liveRes.Code)
	}
	if liveRes.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected X-Request-Id header")
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/ready", nil)
	readyRes := httptest.NewRecorder()
	h.ServeHTTP(readyRes, readyReq)
	if readyRes.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected /ready status when unready: %d", readyRes.Code)
	}

	readyErr = nil
	readyReq2 := httptest.NewRequest(http.MethodGet, "/ready", nil)
	readyRes2 := httptest.NewRecorder()
	h.ServeHTTP(readyRes2, readyReq2)
	if readyRes2.Code != http.StatusOK {
		t.Fatalf("unexpected /ready status when ready: %d", readyRes2.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRes := httptest.NewRecorder()
	h.ServeHTTP(metricsRes, metricsReq)
	if metricsRes.Code != http.StatusOK {
		t.Fatalf("unexpected /metrics status: %d", metricsRes.Code)
	}
	if metricsRes.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected X-Request-Id on metrics response")
	}
}
