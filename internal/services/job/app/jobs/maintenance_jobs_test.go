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

type stubUploadSessionRepository struct {
	expireBefore time.Time
	expireLimit  int
	repairBefore time.Time
	repairLimit  int
}

func (s *stubUploadSessionRepository) ExpirePendingSessions(_ context.Context, expiredBefore time.Time, limit int) (int, error) {
	s.expireBefore = expiredBefore
	s.expireLimit = limit
	return 1, nil
}

func (s *stubUploadSessionRepository) RepairStuckCompleting(_ context.Context, staleBefore time.Time, limit int) (int, error) {
	s.repairBefore = staleBefore
	s.repairLimit = limit
	return 1, nil
}

type stubFileCleanupRepository struct {
	eligibleBefore time.Time
	limit          int
}

func (s *stubFileCleanupRepository) FinalizeDeletedFiles(_ context.Context, eligibleBefore time.Time, limit int) (int, error) {
	s.eligibleBefore = eligibleBefore
	s.limit = limit
	return 1, nil
}

type stubBlobRepository struct {
	staleBefore time.Time
	limit       int
}

func (s *stubBlobRepository) CleanupOrphanBlobs(_ context.Context, staleBefore time.Time, limit int) (int, error) {
	s.staleBefore = staleBefore
	s.limit = limit
	return 1, nil
}

type stubTenantUsageRepository struct {
	limit int
}

func (s *stubTenantUsageRepository) ReconcileTenantUsage(_ context.Context, limit int) (int, error) {
	s.limit = limit
	return 1, nil
}

type stubMultipartRepository struct {
	staleBefore time.Time
	limit       int
}

func (s *stubMultipartRepository) CleanupMultipartUploads(_ context.Context, staleBefore time.Time, limit int) (int, error) {
	s.staleBefore = staleBefore
	s.limit = limit
	return 1, nil
}
