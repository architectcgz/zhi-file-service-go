package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"go.opentelemetry.io/otel/trace"
)

func TestNewTracingWithoutExporterStillBuildsProvider(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	tracing, err := NewTracing(context.Background(), "upload-service", config.OTELConfig{
		ServiceVersion: "test",
	}, logger)
	if err != nil {
		t.Fatalf("NewTracing returned error: %v", err)
	}

	if tracing == nil {
		t.Fatal("expected tracing instance")
	}
	if err := tracing.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func TestWrapHTTPInjectsTraceContext(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	tracing, err := NewTracing(context.Background(), "access-service", config.OTELConfig{
		ServiceVersion: "test",
	}, logger)
	if err != nil {
		t.Fatalf("NewTracing returned error: %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := tracing.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown returned error: %v", shutdownErr)
		}
	})

	var traceID string
	var spanID string

	handler := WrapHTTP("access-service", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spanContext := trace.SpanContextFromContext(r.Context())
		traceID = spanContext.TraceID().String()
		spanID = spanContext.SpanID().String()
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", res.Code)
	}
	if traceID == "" || traceID == "00000000000000000000000000000000" {
		t.Fatalf("expected valid trace id, got: %s", traceID)
	}
	if spanID == "" || spanID == "0000000000000000" {
		t.Fatalf("expected valid span id, got: %s", spanID)
	}
}
