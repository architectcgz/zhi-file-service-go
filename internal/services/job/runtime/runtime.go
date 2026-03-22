package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
	jobjobs "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	jobobs "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/observability"
	jobscheduler "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/scheduler"
	jobpostgres "github.com/architectcgz/zhi-file-service-go/internal/services/job/infra/postgres"
	jobrunner "github.com/architectcgz/zhi-file-service-go/internal/services/job/infra/runner"
	jobstorage "github.com/architectcgz/zhi-file-service-go/internal/services/job/infra/storage"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

func Build(app *bootstrap.App) (bootstrap.RuntimeOptions, error) {
	if app == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap app is nil")
	}
	if app.DB == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap database is nil")
	}
	if app.Redis == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap redis is nil")
	}
	if app.Storage == nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("bootstrap storage is nil")
	}
	if strings.TrimSpace(app.Config.Job.LockBackend) != "redis" {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("unsupported job lock backend: %s", app.Config.Job.LockBackend)
	}

	storageAdapter, err := jobstorage.NewAdapter(app.Storage, app.Config.Storage)
	if err != nil {
		return bootstrap.RuntimeOptions{}, fmt.Errorf("build job storage adapter: %w", err)
	}

	health := jobobs.NewMemoryHealthStore()
	observer := jobobs.NewObserver(jobobs.Options{Health: health})
	clk := clock.SystemClock{}

	uploadSessions := jobpostgres.NewUploadSessionRepository(app.DB)
	fileCleanup := jobpostgres.NewFileCleanupRepository(app.DB, storageAdapter)
	blobRepo := jobpostgres.NewBlobRepository(app.DB, storageAdapter)
	tenantUsage := jobpostgres.NewTenantUsageRepository(app.DB)
	locker := jobrunner.NewRedisLocker(app.Redis)
	scheduler := jobscheduler.New(locker, nil)

	runtime := jobrunner.New(jobrunner.RuntimeConfig{
		Scheduler: scheduler,
		Jobs: []jobrunner.ScheduledJob{
			{
				Job: jobjobs.NewExpireUploadSessionsJob(uploadSessions, clk, jobjobs.ExpireUploadSessionsConfig{
					BatchSize: app.Config.Job.DefaultBatchSize,
				}),
				Interval: app.Config.Job.ExpireUploadSessionsInterval,
			},
			{
				Job: jobjobs.NewRepairStuckCompletingJob(uploadSessions, clk, jobjobs.RepairStuckCompletingConfig{
					BatchSize: app.Config.Job.DefaultBatchSize,
				}),
				Interval: app.Config.Job.RepairStuckCompletingInterval,
			},
			{
				Job: jobjobs.NewFinalizeFileDeleteJob(fileCleanup, clk, jobjobs.FinalizeFileDeleteConfig{
					BatchSize: app.Config.Job.DefaultBatchSize,
					Retention: app.Config.Job.FileDeleteRetention,
				}),
				Interval: app.Config.Job.FinalizeFileDeleteInterval,
			},
			{
				Job: jobjobs.NewCleanupOrphanBlobsJob(blobRepo, clk, jobjobs.CleanupOrphanBlobsConfig{
					BatchSize: app.Config.Job.DefaultBatchSize,
				}),
				Interval: app.Config.Job.CleanupOrphanBlobsInterval,
			},
			{
				Job: jobjobs.NewReconcileTenantUsageJob(tenantUsage, jobjobs.ReconcileTenantUsageConfig{
					BatchSize: app.Config.Job.DefaultBatchSize,
				}),
				Interval: app.Config.Job.ReconcileTenantUsageInterval,
			},
		},
		LockTTL:       app.Config.Job.LockTTL,
		RenewInterval: app.Config.Job.LockRenewInterval,
		Observer:      observer,
	})

	return bootstrap.RuntimeOptions{
		Ready: func(ctx context.Context, _ *bootstrap.App) error {
			if !app.Config.Job.SchedulerEnabled {
				return nil
			}
			return runtime.Ready(ctx)
		},
		Start: func(ctx context.Context, _ *bootstrap.App) error {
			if !app.Config.Job.SchedulerEnabled {
				return nil
			}
			return runtime.Start(ctx)
		},
		Stop: func(ctx context.Context, _ *bootstrap.App) error {
			return runtime.Stop(ctx)
		},
	}, nil
}
