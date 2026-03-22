package jobs

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
)

const JobNameReconcileTenantUsage = "reconcile_tenant_usage"

type ReconcileTenantUsageConfig struct {
	BatchSize int
}

type ReconcileTenantUsageJob struct {
	repo      ports.TenantUsageRepository
	batchSize int
}

func NewReconcileTenantUsageJob(
	repo ports.TenantUsageRepository,
	cfg ReconcileTenantUsageConfig,
) ReconcileTenantUsageJob {
	return ReconcileTenantUsageJob{
		repo:      repo,
		batchSize: normalizeBatchSize(cfg.BatchSize),
	}
}

func (j ReconcileTenantUsageJob) Name() string {
	return JobNameReconcileTenantUsage
}

func (j ReconcileTenantUsageJob) Execute(ctx context.Context) error {
	_, err := j.ExecuteWithResult(ctx)
	return err
}

func (j ReconcileTenantUsageJob) ExecuteWithResult(ctx context.Context) (Result, error) {
	processed, err := j.repo.ReconcileTenantUsage(ctx, j.batchSize)
	return Result{ItemsProcessed: processed}, err
}
