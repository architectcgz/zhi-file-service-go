package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Printf("job-service exited with error: %v", err)
		os.Exit(1)
	}
}

func run(_ context.Context) error {
	// TODO: 初始化 config、logger、DB、Redis、Storage、Scheduler
	return nil
}
