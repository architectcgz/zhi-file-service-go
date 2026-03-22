package jobs

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

const JobNameFinalizeFileDelete = "finalize_file_delete"

type FinalizeFileDeleteConfig struct {
	BatchSize int
	Retention time.Duration
}

type FinalizeFileDeleteJob struct {
	repo      ports.FileCleanupRepository
	clock     clock.Clock
	batchSize int
	retention time.Duration
}

func NewFinalizeFileDeleteJob(
	repo ports.FileCleanupRepository,
	clk clock.Clock,
	cfg FinalizeFileDeleteConfig,
) FinalizeFileDeleteJob {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.Retention <= 0 {
		cfg.Retention = 24 * time.Hour
	}

	return FinalizeFileDeleteJob{
		repo:      repo,
		clock:     clk,
		batchSize: normalizeBatchSize(cfg.BatchSize),
		retention: cfg.Retention,
	}
}

func (j FinalizeFileDeleteJob) Name() string {
	return JobNameFinalizeFileDelete
}

func (j FinalizeFileDeleteJob) Execute(ctx context.Context) error {
	_, err := j.ExecuteWithResult(ctx)
	return err
}

func (j FinalizeFileDeleteJob) ExecuteWithResult(ctx context.Context) (Result, error) {
	processed, err := j.repo.FinalizeDeletedFiles(ctx, j.clock.Now().Add(-j.retention), j.batchSize)
	return Result{ItemsProcessed: processed}, err
}
