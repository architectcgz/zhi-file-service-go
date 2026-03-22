package bootstrap

import (
	"context"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
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
