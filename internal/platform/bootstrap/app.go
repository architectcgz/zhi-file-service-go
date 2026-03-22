package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/httpserver"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/persistence"
	platformredis "github.com/architectcgz/zhi-file-service-go/internal/platform/redis"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/storage"
)

type RuntimeReadyFunc func(context.Context, *App) error

type RuntimeOptions struct {
	Handler http.Handler
	Ready   RuntimeReadyFunc
}

type Options struct {
	ServiceName string
	Runtime     RuntimeOptions
}

type App struct {
	Config  config.Config
	Logger  *slog.Logger
	Metrics *observability.Metrics
	Tracing *observability.Tracing

	DB      *sql.DB
	Redis   *platformredis.Client
	Storage *storage.Client
	Server  *httpserver.Server

	ready             atomic.Bool
	runtimeRegistered atomic.Bool
	runtimeReadyCheck RuntimeReadyFunc
}

func New(ctx context.Context, serviceName string) (*App, error) {
	return NewWithOptions(ctx, Options{ServiceName: serviceName})
}

func NewWithOptions(ctx context.Context, options Options) (*App, error) {
	cfg, err := config.Load(options.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logger, err := observability.NewLogger(cfg.App.ServiceName, cfg.App.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	app := &App{
		Config:  cfg,
		Logger:  logger,
		Metrics: observability.NewMetrics(cfg.Metrics.Enabled),
	}

	tracing, err := observability.NewTracing(ctx, cfg.App.ServiceName, cfg.OTEL, logger)
	if err != nil {
		return nil, fmt.Errorf("init tracing: %w", err)
	}
	app.Tracing = tracing

	if err := app.initDependencies(ctx); err != nil {
		_ = app.Close(context.Background())
		return nil, err
	}

	app.registerRuntime(options.Runtime)

	app.ready.Store(true)
	logger.Info("platform_bootstrap_initialized", "service", cfg.App.ServiceName)

	return app, nil
}

func Run(ctx context.Context, serviceName string) error {
	return RunWithOptions(ctx, Options{ServiceName: serviceName})
}

func RunWithOptions(ctx context.Context, options Options) error {
	app, err := NewWithOptions(ctx, options)
	if err != nil {
		return err
	}
	return app.Run(ctx)
}

func (a *App) Run(ctx context.Context) error {
	if a.Server == nil {
		return errors.New("http server is not initialized")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Server.Start()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			_ = a.Close(context.Background())
			return fmt.Errorf("run http server: %w", err)
		}
		return a.Close(context.Background())
	case <-ctx.Done():
		a.ready.Store(false)
		a.runtimeRegistered.Store(false)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.Config.App.ShutdownTimeout)
		defer cancel()

		var errs []error
		if err := a.Server.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
		}
		if err := a.Close(shutdownCtx); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}
}

func (a *App) RegisterRuntime(options RuntimeOptions) {
	a.registerRuntime(options)
}

func (a *App) Ready(ctx context.Context) error {
	if !a.ready.Load() {
		return errors.New("app not ready")
	}
	if !a.runtimeRegistered.Load() {
		return errors.New("service runtime not registered")
	}

	probeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if a.DB == nil {
		return errors.New("database not initialized")
	}
	if err := a.DB.PingContext(probeCtx); err != nil {
		return fmt.Errorf("database not ready: %w", err)
	}

	if a.Config.RedisRequired() && a.Redis == nil {
		return errors.New("redis not initialized")
	}
	if a.Redis != nil {
		if err := a.Redis.Ping(probeCtx); err != nil {
			return fmt.Errorf("redis not ready: %w", err)
		}
	}

	if a.Storage == nil {
		return errors.New("storage not initialized")
	}
	if err := a.Storage.Validate(probeCtx); err != nil {
		return fmt.Errorf("storage not ready: %w", err)
	}
	if a.runtimeReadyCheck != nil {
		if err := a.runtimeReadyCheck(probeCtx, a); err != nil {
			return fmt.Errorf("service runtime not ready: %w", err)
		}
	}

	return nil
}

func (a *App) Close(ctx context.Context) error {
	a.ready.Store(false)

	var errs []error

	if a.Redis != nil {
		if err := a.Redis.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close redis: %w", err))
		}
	}
	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close database: %w", err))
		}
	}
	if a.Tracing != nil {
		if err := a.Tracing.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown tracing: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (a *App) initDependencies(ctx context.Context) error {
	setupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db, err := persistence.Open(setupCtx, a.Config.DB)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	a.DB = db

	redisClient, err := platformredis.Open(setupCtx, a.Config.Redis, a.Config.RedisRequired(), a.Logger)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	a.Redis = redisClient

	storageClient, err := storage.Open(a.Config.Storage)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	if err := storageClient.Validate(setupCtx); err != nil {
		return fmt.Errorf("validate storage: %w", err)
	}
	a.Storage = storageClient

	return nil
}

func (a *App) registerRuntime(options RuntimeOptions) {
	a.runtimeReadyCheck = options.Ready
	a.Server = httpserver.New(httpserver.Options{
		ServiceName:    a.Config.App.ServiceName,
		HTTP:           a.Config.HTTP,
		Logger:         a.Logger,
		Ready:          a.Ready,
		MetricsHandler: a.Metrics.Handler(),
		Handler:        options.Handler,
	})
	a.runtimeRegistered.Store(a.Server.HasBusinessHandler() || options.Ready != nil)
	if !a.runtimeRegistered.Load() && a.Logger != nil {
		a.Logger.Warn("service_runtime_not_registered", "service", a.Config.App.ServiceName)
	}
}
