package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	jobscheduler "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/scheduler"
)

func TestRunnerReadyReflectsLifecycle(t *testing.T) {
	t.Parallel()

	scheduler := &stubScheduler{}
	r := New(RuntimeConfig{
		Scheduler: scheduler,
		Jobs: []ScheduledJob{
			{
				Job:      jobs.Func{JobName: jobs.JobNameExpireUploadSessions},
				Interval: 10 * time.Millisecond,
			},
		},
		LockTTL:       time.Second,
		RenewInterval: 100 * time.Millisecond,
	})

	if err := r.Ready(context.Background()); err == nil {
		t.Fatal("expected readiness error before start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for scheduler.Calls() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if scheduler.Calls() == 0 {
		t.Fatal("expected scheduled job to run at least once")
	}
	if err := r.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := r.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := r.Ready(context.Background()); err == nil {
		t.Fatal("expected readiness error after stop")
	}
}

type stubScheduler struct {
	calls atomic.Int64
}

func (s *stubScheduler) RunOnce(_ context.Context, _ jobs.Job, _ jobscheduler.Config) (jobscheduler.RunResult, error) {
	s.calls.Add(1)
	return jobscheduler.RunResult{Acquired: true, Executed: true}, nil
}

func (s *stubScheduler) Calls() int64 {
	if s == nil {
		return 0
	}
	return s.calls.Load()
}

func TestRunnerRejectsSecondStart(t *testing.T) {
	t.Parallel()

	r := New(RuntimeConfig{
		Scheduler: &stubScheduler{},
		Jobs: []ScheduledJob{
			{Job: jobs.Func{JobName: jobs.JobNameCleanupMultipart}, Interval: time.Second},
		},
		LockTTL: time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	if err := r.Start(ctx); err == nil {
		t.Fatal("expected second Start() to fail")
	}
}

func TestRunnerStopWithoutStartIsNoop(t *testing.T) {
	t.Parallel()

	r := New(RuntimeConfig{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestRunnerRequiresSchedulerWhenJobsConfigured(t *testing.T) {
	t.Parallel()

	r := New(RuntimeConfig{
		Jobs: []ScheduledJob{
			{Job: jobs.Func{JobName: jobs.JobNameCleanupOrphanBlobs}, Interval: time.Second},
		},
	})

	err := r.Start(context.Background())
	if err == nil || !errors.Is(err, ErrSchedulerRequired) {
		t.Fatalf("Start() error = %v, want %v", err, ErrSchedulerRequired)
	}
}
