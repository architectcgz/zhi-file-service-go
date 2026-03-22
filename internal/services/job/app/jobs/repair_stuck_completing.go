package jobs

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

const JobNameRepairStuckCompleting = "repair_stuck_completing"

type RepairStuckCompletingConfig struct {
	BatchSize  int
	StaleAfter time.Duration
}

type RepairStuckCompletingJob struct {
	repo       ports.UploadSessionRepository
	clock      clock.Clock
	batchSize  int
	staleAfter time.Duration
}

func NewRepairStuckCompletingJob(
	repo ports.UploadSessionRepository,
	clk clock.Clock,
	cfg RepairStuckCompletingConfig,
) RepairStuckCompletingJob {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = 15 * time.Minute
	}

	return RepairStuckCompletingJob{
		repo:       repo,
		clock:      clk,
		batchSize:  normalizeBatchSize(cfg.BatchSize),
		staleAfter: cfg.StaleAfter,
	}
}

func (j RepairStuckCompletingJob) Name() string {
	return JobNameRepairStuckCompleting
}

func (j RepairStuckCompletingJob) Execute(ctx context.Context) error {
	_, err := j.ExecuteWithResult(ctx)
	return err
}

func (j RepairStuckCompletingJob) ExecuteWithResult(ctx context.Context) (Result, error) {
	processed, err := j.repo.RepairStuckCompleting(ctx, j.clock.Now().Add(-j.staleAfter), j.batchSize)
	return Result{ItemsProcessed: processed}, err
}
