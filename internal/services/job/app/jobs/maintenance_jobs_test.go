package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

func TestExpireUploadSessionsJobUsesConfiguredCutoffAndBatchSize(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 0, 0, 0, time.UTC)
	repo := &stubUploadSessionRepository{}
	job := jobs.NewExpireUploadSessionsJob(repo, clock.NewFixed(now), jobs.ExpireUploadSessionsConfig{
		BatchSize:   50,
		ExpireAfter: 45 * time.Minute,
	})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if job.Name() != jobs.JobNameExpireUploadSessions {
		t.Fatalf("Name() = %q, want %q", job.Name(), jobs.JobNameExpireUploadSessions)
	}
	if repo.expireLimit != 50 {
		t.Fatalf("expire limit = %d, want 50", repo.expireLimit)
	}
	if !repo.expireBefore.Equal(now.Add(-45 * time.Minute)) {
		t.Fatalf("expire before = %s, want %s", repo.expireBefore, now.Add(-45*time.Minute))
	}
}

func TestRepairStuckCompletingJobUsesConfiguredCutoff(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 10, 0, 0, time.UTC)
	repo := &stubUploadSessionRepository{}
	job := jobs.NewRepairStuckCompletingJob(repo, clock.NewFixed(now), jobs.RepairStuckCompletingConfig{
		BatchSize:  25,
		StaleAfter: 20 * time.Minute,
	})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.repairLimit != 25 {
		t.Fatalf("repair limit = %d, want 25", repo.repairLimit)
	}
	if !repo.repairBefore.Equal(now.Add(-20 * time.Minute)) {
		t.Fatalf("repair before = %s, want %s", repo.repairBefore, now.Add(-20*time.Minute))
	}
}

func TestRepairStuckCompletingJobUsesDefaultConfigWhenUnset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 12, 0, 0, time.UTC)
	repo := &stubUploadSessionRepository{}
	job := jobs.NewRepairStuckCompletingJob(repo, clock.NewFixed(now), jobs.RepairStuckCompletingConfig{})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.repairLimit != 100 {
		t.Fatalf("repair limit = %d, want 100", repo.repairLimit)
	}
	if !repo.repairBefore.Equal(now.Add(-15 * time.Minute)) {
		t.Fatalf("repair before = %s, want %s", repo.repairBefore, now.Add(-15*time.Minute))
	}
}

func TestRepairStuckCompletingJobReturnsProcessedCountWhenNoWork(t *testing.T) {
	t.Parallel()

	repo := &stubUploadSessionRepository{repairProcessed: 0}
	job := jobs.NewRepairStuckCompletingJob(repo, nil, jobs.RepairStuckCompletingConfig{BatchSize: 20})

	result, err := jobs.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ItemsProcessed != 0 {
		t.Fatalf("ItemsProcessed = %d, want 0", result.ItemsProcessed)
	}
	if repo.repairLimit != 20 {
		t.Fatalf("repair limit = %d, want 20", repo.repairLimit)
	}
}

func TestFinalizeFileDeleteJobUsesRetentionCutoff(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 20, 0, 0, time.UTC)
	repo := &stubFileCleanupRepository{}
	job := jobs.NewFinalizeFileDeleteJob(repo, clock.NewFixed(now), jobs.FinalizeFileDeleteConfig{
		BatchSize: 10,
		Retention: 12 * time.Hour,
	})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 10 {
		t.Fatalf("limit = %d, want 10", repo.limit)
	}
	if !repo.eligibleBefore.Equal(now.Add(-12 * time.Hour)) {
		t.Fatalf("eligible before = %s, want %s", repo.eligibleBefore, now.Add(-12*time.Hour))
	}
}

func TestFinalizeFileDeleteJobUsesDefaultConfigWhenUnset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 21, 0, 0, time.UTC)
	repo := &stubFileCleanupRepository{}
	job := jobs.NewFinalizeFileDeleteJob(repo, clock.NewFixed(now), jobs.FinalizeFileDeleteConfig{})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 100 {
		t.Fatalf("limit = %d, want 100", repo.limit)
	}
	if !repo.eligibleBefore.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("eligible before = %s, want %s", repo.eligibleBefore, now.Add(-24*time.Hour))
	}
}

func TestCleanupOrphanBlobsJobUsesDefaultBatchWhenUnset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 30, 0, 0, time.UTC)
	repo := &stubBlobRepository{}
	job := jobs.NewCleanupOrphanBlobsJob(repo, clock.NewFixed(now), jobs.CleanupOrphanBlobsConfig{
		StaleAfter: 6 * time.Hour,
	})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 100 {
		t.Fatalf("default limit = %d, want 100", repo.limit)
	}
	if !repo.staleBefore.Equal(now.Add(-6 * time.Hour)) {
		t.Fatalf("stale before = %s, want %s", repo.staleBefore, now.Add(-6*time.Hour))
	}
}

func TestCleanupOrphanBlobsJobUsesDefaultConfigWhenUnset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 31, 0, 0, time.UTC)
	repo := &stubBlobRepository{}
	job := jobs.NewCleanupOrphanBlobsJob(repo, clock.NewFixed(now), jobs.CleanupOrphanBlobsConfig{})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 100 {
		t.Fatalf("limit = %d, want 100", repo.limit)
	}
	if !repo.staleBefore.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("stale before = %s, want %s", repo.staleBefore, now.Add(-24*time.Hour))
	}
}

func TestReconcileTenantUsageJobUsesConfiguredBatchSize(t *testing.T) {
	t.Parallel()

	repo := &stubTenantUsageRepository{}
	job := jobs.NewReconcileTenantUsageJob(repo, jobs.ReconcileTenantUsageConfig{BatchSize: 77})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 77 {
		t.Fatalf("limit = %d, want 77", repo.limit)
	}
}

func TestReconcileTenantUsageJobUsesDefaultBatchWhenUnset(t *testing.T) {
	t.Parallel()

	repo := &stubTenantUsageRepository{}
	job := jobs.NewReconcileTenantUsageJob(repo, jobs.ReconcileTenantUsageConfig{})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 100 {
		t.Fatalf("limit = %d, want 100", repo.limit)
	}
}

func TestReconcileTenantUsageJobReturnsProcessedCountForNoopBatch(t *testing.T) {
	t.Parallel()

	repo := &stubTenantUsageRepository{processed: 0}
	job := jobs.NewReconcileTenantUsageJob(repo, jobs.ReconcileTenantUsageConfig{BatchSize: 12})

	result, err := jobs.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ItemsProcessed != 0 {
		t.Fatalf("ItemsProcessed = %d, want 0", result.ItemsProcessed)
	}
	if repo.limit != 12 {
		t.Fatalf("limit = %d, want 12", repo.limit)
	}
}

func TestCleanupMultipartJobUsesConfiguredCutoff(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 40, 0, 0, time.UTC)
	repo := &stubMultipartRepository{}
	job := jobs.NewCleanupMultipartJob(repo, clock.NewFixed(now), jobs.CleanupMultipartConfig{
		BatchSize:  5,
		StaleAfter: 90 * time.Minute,
	})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 5 {
		t.Fatalf("limit = %d, want 5", repo.limit)
	}
	if !repo.staleBefore.Equal(now.Add(-90 * time.Minute)) {
		t.Fatalf("stale before = %s, want %s", repo.staleBefore, now.Add(-90*time.Minute))
	}
}

func TestCleanupMultipartJobUsesDefaultConfigWhenUnset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 41, 0, 0, time.UTC)
	repo := &stubMultipartRepository{}
	job := jobs.NewCleanupMultipartJob(repo, clock.NewFixed(now), jobs.CleanupMultipartConfig{})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.limit != 100 {
		t.Fatalf("limit = %d, want 100", repo.limit)
	}
	if !repo.staleBefore.Equal(now.Add(-2 * time.Hour)) {
		t.Fatalf("stale before = %s, want %s", repo.staleBefore, now.Add(-2*time.Hour))
	}
}

func TestExpireUploadSessionsJobUsesDefaultConfigWhenUnset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 14, 1, 0, 0, time.UTC)
	repo := &stubUploadSessionRepository{}
	job := jobs.NewExpireUploadSessionsJob(repo, clock.NewFixed(now), jobs.ExpireUploadSessionsConfig{})

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.expireLimit != 100 {
		t.Fatalf("expire limit = %d, want 100", repo.expireLimit)
	}
	if !repo.expireBefore.Equal(now.Add(-30 * time.Minute)) {
		t.Fatalf("expire before = %s, want %s", repo.expireBefore, now.Add(-30*time.Minute))
	}
}

type stubUploadSessionRepository struct {
	expireBefore    time.Time
	expireLimit     int
	expireProcessed int
	repairBefore    time.Time
	repairLimit     int
	repairProcessed int
}

func (s *stubUploadSessionRepository) ExpirePendingSessions(_ context.Context, expiredBefore time.Time, limit int) (int, error) {
	s.expireBefore = expiredBefore
	s.expireLimit = limit
	return s.expireProcessed, nil
}

func (s *stubUploadSessionRepository) RepairStuckCompleting(_ context.Context, staleBefore time.Time, limit int) (int, error) {
	s.repairBefore = staleBefore
	s.repairLimit = limit
	return s.repairProcessed, nil
}

type stubFileCleanupRepository struct {
	eligibleBefore time.Time
	limit          int
	processed      int
}

func (s *stubFileCleanupRepository) FinalizeDeletedFiles(_ context.Context, eligibleBefore time.Time, limit int) (int, error) {
	s.eligibleBefore = eligibleBefore
	s.limit = limit
	return s.processed, nil
}

type stubBlobRepository struct {
	staleBefore time.Time
	limit       int
	processed   int
}

func (s *stubBlobRepository) CleanupOrphanBlobs(_ context.Context, staleBefore time.Time, limit int) (int, error) {
	s.staleBefore = staleBefore
	s.limit = limit
	return s.processed, nil
}

type stubTenantUsageRepository struct {
	limit     int
	processed int
}

func (s *stubTenantUsageRepository) ReconcileTenantUsage(_ context.Context, limit int) (int, error) {
	s.limit = limit
	return s.processed, nil
}

type stubMultipartRepository struct {
	staleBefore time.Time
	limit       int
	processed   int
}

func (s *stubMultipartRepository) CleanupMultipartUploads(_ context.Context, staleBefore time.Time, limit int) (int, error) {
	s.staleBefore = staleBefore
	s.limit = limit
	return s.processed, nil
}
