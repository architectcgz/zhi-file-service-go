package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
)

func TestRunStartsAndStopsRuntimeHooks(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	stopped := make(chan struct{}, 1)
	serverDone := make(chan struct{}, 1)
	shutdownDone := make(chan struct{}, 1)

	app := &App{
		Config: config.Config{
			App: config.AppConfig{
				ServiceName:     config.ServiceJob,
				ShutdownTimeout: time.Second,
			},
			HTTP: config.HTTPConfig{Port: 8080},
		},
		Metrics: observability.NewMetrics(false),
		Server: &stubHTTPServer{
			start: func() error {
				close(serverDone)
				<-shutdownDone
				return nil
			},
			shutdown: func(context.Context) error {
				close(shutdownDone)
				return nil
			},
		},
	}
	app.ready.Store(true)
	app.runtimeRegistered.Store(true)
	app.runtimeStart = func(context.Context, *App) error {
		close(started)
		return nil
	}
	app.runtimeStop = func(context.Context, *App) error {
		close(stopped)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run(ctx)
	}()

	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("expected server to be started")
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("expected Run() to return")
	}

	select {
	case <-started:
	default:
		t.Fatal("expected runtime start hook to run")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("expected runtime stop hook to run")
	}
}

type stubHTTPServer struct {
	start    func() error
	shutdown func(context.Context) error
}

func (s *stubHTTPServer) Start() error {
	if s.start != nil {
		return s.start()
	}
	return nil
}

func (s *stubHTTPServer) Shutdown(ctx context.Context) error {
	if s.shutdown != nil {
		return s.shutdown(ctx)
	}
	return nil
}

func (s *stubHTTPServer) Handler() http.Handler {
	return http.NotFoundHandler()
}

func (s *stubHTTPServer) HasBusinessHandler() bool {
	return false
}

func TestRunReturnsRuntimeStartError(t *testing.T) {
	t.Parallel()

	app := &App{
		Config: config.Config{
			App: config.AppConfig{
				ServiceName:     config.ServiceJob,
				ShutdownTimeout: time.Second,
			},
		},
		Server:  &stubHTTPServer{},
		Metrics: observability.NewMetrics(false),
	}
	app.ready.Store(true)
	app.runtimeRegistered.Store(true)
	app.runtimeStart = func(context.Context, *App) error {
		return errors.New("runtime start failed")
	}

	err := app.Run(context.Background())
	if err == nil || err.Error() != "start service runtime: runtime start failed" {
		t.Fatalf("Run() error = %v, want runtime start failure", err)
	}
}
