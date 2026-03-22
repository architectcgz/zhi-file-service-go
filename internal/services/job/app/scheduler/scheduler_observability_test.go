package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/observability"
)

func TestRunOnceRecordsObservedJobSuccess(t *testing.T) {
	t.Parallel()

	metrics := &stubMetricsRecorder{}
	logger := &stubJobLogger{}
	tracer := &stubJobTracer{}
	health := observability.NewMemoryHealthStore()
	observer := observability.NewObserver(observability.Options{
		Metrics: metrics,
		Logger:  logger,
		Tracer:  tracer,
		Health:  health,
	})

	handle := &stubLockHandle{key: "job:expire_upload_sessions", owner: "node-a"}
	locker := &stubLocker{acquired: true, handle: handle}
	worker := &stubWorker{
		result: jobs.Result{ItemsProcessed: 7, RetryCount: 2},
	}
	s := New(locker, worker)

	result, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameExpireUploadSessions}, Config{
		LockTTL:       30 * time.Second,
		RenewInterval: time.Hour,
		Observer:      observer,
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if !result.Acquired || !result.Executed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if tracer.started[0] != "job.expire_upload_sessions" {
		t.Fatalf("unexpected span name: %#v", tracer.started)
	}
	if metrics.counterValue("job_run_total") != 1 {
		t.Fatalf("job_run_total = %d, want 1", metrics.counterValue("job_run_total"))
	}
	if metrics.counterValue("job_items_processed_total") != 7 {
		t.Fatalf("job_items_processed_total = %d, want 7", metrics.counterValue("job_items_processed_total"))
	}
	if metrics.counterValue("job_retry_total") != 2 {
		t.Fatalf("job_retry_total = %d, want 2", metrics.counterValue("job_retry_total"))
	}
	if metrics.durationCount("job_duration_seconds") != 1 {
		t.Fatalf("job_duration_seconds count = %d, want 1", metrics.durationCount("job_duration_seconds"))
	}

	snapshot := health.Snapshot()
	jobHealth, ok := snapshot.Jobs[jobs.JobNameExpireUploadSessions]
	if !ok {
		t.Fatalf("job health not recorded: %#v", snapshot)
	}
	if jobHealth.Status != observability.StatusSuccess {
		t.Fatalf("job status = %q, want %q", jobHealth.Status, observability.StatusSuccess)
	}
	if !jobHealth.LockAcquired || jobHealth.ItemsProcessed != 7 || jobHealth.RetryCount != 2 {
		t.Fatalf("unexpected job health: %#v", jobHealth)
	}
	if !logger.hasMessage("job_run_completed") {
		t.Fatalf("expected completion log, got %#v", logger.entries)
	}
}

func TestRunOnceRecordsLockAcquireMissInHealth(t *testing.T) {
	t.Parallel()

	metrics := &stubMetricsRecorder{}
	health := observability.NewMemoryHealthStore()
	observer := observability.NewObserver(observability.Options{
		Metrics: metrics,
		Health:  health,
	})

	locker := &stubLocker{acquired: false}
	s := New(locker, &stubWorker{})

	result, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupOrphanBlobs}, Config{
		LockTTL:  30 * time.Second,
		Observer: observer,
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Acquired || result.Executed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if metrics.counterValue("job_lock_acquire_failed_total") != 1 {
		t.Fatalf("job_lock_acquire_failed_total = %d, want 1", metrics.counterValue("job_lock_acquire_failed_total"))
	}

	jobHealth := health.Snapshot().Jobs[jobs.JobNameCleanupOrphanBlobs]
	if jobHealth.Status != observability.StatusSkipped {
		t.Fatalf("job status = %q, want %q", jobHealth.Status, observability.StatusSkipped)
	}
	if jobHealth.LockAcquired {
		t.Fatalf("expected lock_acquired=false, got %#v", jobHealth)
	}
}

func TestRunOnceRecordsObservedJobFailure(t *testing.T) {
	t.Parallel()

	metrics := &stubMetricsRecorder{}
	logger := &stubJobLogger{}
	health := observability.NewMemoryHealthStore()
	observer := observability.NewObserver(observability.Options{
		Metrics: metrics,
		Logger:  logger,
		Health:  health,
	})

	locker := &stubLocker{
		acquired: true,
		handle:   &stubLockHandle{key: "job:cleanup_multipart", owner: "node-a"},
	}
	worker := &stubWorker{
		result: jobs.Result{ItemsProcessed: 3, RetryCount: 1},
		err:    errors.New("cleanup failed"),
		run: func(context.Context, jobs.Job) (jobs.Result, error) {
			return jobs.Result{ItemsProcessed: 3, RetryCount: 1}, errors.New("cleanup failed")
		},
	}
	s := New(locker, worker)

	_, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupMultipart}, Config{
		LockTTL:  30 * time.Second,
		Observer: observer,
	})
	if err == nil || err.Error() != "cleanup failed" {
		t.Fatalf("RunOnce() error = %v, want %q", err, "cleanup failed")
	}
	if metrics.counterValue("job_run_failed_total") != 1 {
		t.Fatalf("job_run_failed_total = %d, want 1", metrics.counterValue("job_run_failed_total"))
	}
	if !logger.hasMessage("job_run_failed") {
		t.Fatalf("expected failure log, got %#v", logger.entries)
	}

	jobHealth := health.Snapshot().Jobs[jobs.JobNameCleanupMultipart]
	if jobHealth.Status != observability.StatusFailed || jobHealth.LastError != "cleanup failed" {
		t.Fatalf("unexpected job health: %#v", jobHealth)
	}
	if jobHealth.ItemsProcessed != 3 || jobHealth.RetryCount != 1 {
		t.Fatalf("job health result = %#v, want items=3 retry=1", jobHealth)
	}
	if metrics.counterValue("job_items_processed_total") != 3 {
		t.Fatalf("job_items_processed_total = %d, want 3", metrics.counterValue("job_items_processed_total"))
	}
	if metrics.counterValue("job_retry_total") != 1 {
		t.Fatalf("job_retry_total = %d, want 1", metrics.counterValue("job_retry_total"))
	}
}

type stubMetricsRecorder struct {
	counters  map[string]int64
	durations map[string]int
}

func (s *stubMetricsRecorder) AddCounter(name string, value int64, _ observability.Fields) {
	if s.counters == nil {
		s.counters = make(map[string]int64)
	}
	s.counters[name] += value
}

func (s *stubMetricsRecorder) ObserveDuration(name string, _ time.Duration, _ observability.Fields) {
	if s.durations == nil {
		s.durations = make(map[string]int)
	}
	s.durations[name]++
}

func (s *stubMetricsRecorder) counterValue(name string) int64 {
	return s.counters[name]
}

func (s *stubMetricsRecorder) durationCount(name string) int {
	return s.durations[name]
}

type stubJobLogger struct {
	entries []string
}

func (s *stubJobLogger) Log(_ context.Context, _ observability.Level, message string, _ observability.Fields) {
	s.entries = append(s.entries, message)
}

func (s *stubJobLogger) hasMessage(message string) bool {
	for _, entry := range s.entries {
		if entry == message {
			return true
		}
	}
	return false
}

type stubJobTracer struct {
	started []string
	spans   []*stubJobSpan
}

func (s *stubJobTracer) Start(ctx context.Context, name string, _ observability.Fields) (context.Context, observability.Span) {
	s.started = append(s.started, name)
	span := &stubJobSpan{}
	s.spans = append(s.spans, span)
	return ctx, span
}

type stubJobSpan struct{}

func (s *stubJobSpan) AddFields(observability.Fields) {}

func (s *stubJobSpan) RecordError(error) {}

func (s *stubJobSpan) End() {}
