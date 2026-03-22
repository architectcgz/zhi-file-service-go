package bootstrap

import (
	"context"
	"net/http"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
)

func TestReadyReturnsErrorWhenRuntimeIsNotRegistered(t *testing.T) {
	app := &App{
		Config: config.Config{
			App: config.AppConfig{
				ServiceName: config.ServiceUpload,
			},
		},
	}
	app.ready.Store(true)
	app.runtimeRegistered.Store(false)

	err := app.Ready(context.Background())
	if err == nil {
		t.Fatal("expected readiness error")
	}
	if err.Error() != "service runtime not registered" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterRuntimeMarksRuntimeAsRegistered(t *testing.T) {
	app := &App{
		Config: config.Config{
			App: config.AppConfig{
				ServiceName: config.ServiceAccess,
			},
			HTTP: config.HTTPConfig{
				Port: 8080,
			},
		},
		Metrics: observability.NewMetrics(false),
	}

	app.RegisterRuntime(RuntimeOptions{
		Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	})

	if !app.runtimeRegistered.Load() {
		t.Fatal("expected runtime to be registered")
	}
	if app.Server == nil {
		t.Fatal("expected server to be initialized")
	}
}
