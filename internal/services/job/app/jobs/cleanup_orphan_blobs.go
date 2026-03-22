package jobs

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

const JobNameCleanupOrphanBlobs = "cleanup_orphan_blobs"

type CleanupOrphanBlobsConfig struct {
	BatchSize  int
	StaleAfter time.Duration
}

type CleanupOrphanBlobsJob struct {
	repo       ports.BlobRepository
	clock      clock.Clock
	batchSize  int
	staleAfter time.Duration
}

func NewCleanupOrphanBlobsJob(
	repo ports.BlobRepository,
	clk clock.Clock,
	cfg CleanupOrphanBlobsConfig,
) CleanupOrphanBlobsJob {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = 24 * time.Hour
	}

	return CleanupOrphanBlobsJob{
		repo:       repo,
		clock:      clk,
		batchSize:  normalizeBatchSize(cfg.BatchSize),
		staleAfter: cfg.StaleAfter,
	}
}

func (j CleanupOrphanBlobsJob) Name() string {
	return JobNameCleanupOrphanBlobs
}

func (j CleanupOrphanBlobsJob) Execute(ctx context.Context) error {
	_, err := j.repo.CleanupOrphanBlobs(ctx, j.clock.Now().Add(-j.staleAfter), j.batchSize)
	return err
}
