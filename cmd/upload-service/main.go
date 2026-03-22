package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
	uploadruntime "github.com/architectcgz/zhi-file-service-go/internal/services/upload/runtime"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	logger := observability.NewBootstrapLogger("upload-service")

	if err := run(ctx); err != nil {
		logger.Error("service_exit", "error", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	app, err := bootstrap.New(ctx, "upload-service")
	if err != nil {
		return err
	}

	runtimeOptions, err := uploadruntime.Build(app)
	if err != nil {
		_ = app.Close(context.Background())
		return err
	}

	app.RegisterRuntime(runtimeOptions)
	return app.Run(ctx)
}
