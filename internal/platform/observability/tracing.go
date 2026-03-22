package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const defaultTraceSampleRatio = 0.05

type Tracing struct {
	provider *sdktrace.TracerProvider
}

func NewTracing(ctx context.Context, serviceName string, cfg config.OTELConfig, logger *slog.Logger) (*Tracing, error) {
	resourceAttrs, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resourceAttrs),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(defaultTraceSampleRatio))),
	}

	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		exporter, exporterErr := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if exporterErr != nil {
			return nil, fmt.Errorf("init otlp trace exporter: %w", exporterErr)
		}
		options = append(options, sdktrace.WithBatcher(exporter))

		if logger != nil {
			logger.Info("trace_exporter_enabled", "endpoint", endpoint)
		}
	} else if logger != nil {
		logger.Info("trace_exporter_disabled")
	}

	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracing{provider: provider}, nil
}

func (t *Tracing) Shutdown(ctx context.Context) error {
	if t == nil || t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

func WrapHTTP(serviceName string, next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}

	operation := strings.TrimSpace(serviceName)
	if operation == "" {
		operation = "http.server"
	}

	return otelhttp.NewHandler(next, operation,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return spanNameForRequest(operation, r)
		}),
	)
}

func spanNameForRequest(serviceName string, r *http.Request) string {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		name = "http"
	}

	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		path = "root"
	}

	replacer := strings.NewReplacer("/", ".", "-", "_")
	return name + "." + replacer.Replace(path)
}
