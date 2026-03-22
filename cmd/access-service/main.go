package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	logger := observability.NewBootstrapLogger("access-service")

	if err := run(ctx); err != nil {
		logger.Error("service_exit", "error", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	return bootstrap.Run(ctx, "access-service")
}
