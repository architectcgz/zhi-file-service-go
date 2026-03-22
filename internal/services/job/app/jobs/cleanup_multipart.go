package jobs

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

const JobNameCleanupMultipart = "cleanup_multipart"

type CleanupMultipartConfig struct {
	BatchSize  int
	StaleAfter time.Duration
}

type CleanupMultipartJob struct {
	repo       ports.MultipartRepository
	clock      clock.Clock
	batchSize  int
	staleAfter time.Duration
}

func NewCleanupMultipartJob(
	repo ports.MultipartRepository,
	clk clock.Clock,
	cfg CleanupMultipartConfig,
) CleanupMultipartJob {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = 2 * time.Hour
	}

	return CleanupMultipartJob{
		repo:       repo,
		clock:      clk,
		batchSize:  normalizeBatchSize(cfg.BatchSize),
		staleAfter: cfg.StaleAfter,
	}
}

func (j CleanupMultipartJob) Name() string {
	return JobNameCleanupMultipart
}

func (j CleanupMultipartJob) Execute(ctx context.Context) error {
	_, err := j.repo.CleanupMultipartUploads(ctx, j.clock.Now().Add(-j.staleAfter), j.batchSize)
	return err
}
