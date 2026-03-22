package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
)

func TestRunOnceSkipsWhenLeadershipNotAcquired(t *testing.T) {
	t.Parallel()

	locker := &stubLocker{acquired: false}
	worker := &stubWorker{}
	s := New(locker, worker)

	result, err := s.RunOnce(context.Background(), jobs.Func{
		JobName: "cleanup_orphan_blobs",
	}, Config{
		LockTTL: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Acquired || result.Executed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if worker.calls != 0 {
		t.Fatalf("worker calls = %d, want 0", worker.calls)
	}
	if locker.lastKey != "job:cleanup_orphan_blobs" {
		t.Fatalf("locker key = %q, want %q", locker.lastKey, "job:cleanup_orphan_blobs")
	}
}

func TestRunOnceExecutesAndReleasesLock(t *testing.T) {
	t.Parallel()

	handle := &stubLockHandle{
		key:   "job:expire_upload_sessions",
		owner: "node-a",
	}
	locker := &stubLocker{
		acquired: true,
		handle:   handle,
	}
	worker := &stubWorker{}
	s := New(locker, worker)

	result, err := s.RunOnce(context.Background(), jobs.Func{
		JobName: "expire_upload_sessions",
	}, Config{
		LockTTL:       30 * time.Second,
		RenewInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if !result.Acquired || !result.Executed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if worker.calls != 1 {
		t.Fatalf("worker calls = %d, want 1", worker.calls)
	}
	if handle.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", handle.releaseCalls)
	}
}

func TestRunOnceRenewsWhileWorkerIsRunning(t *testing.T) {
	t.Parallel()

	handle := &stubLockHandle{
		key:     "job:reconcile_tenant_usage",
		owner:   "node-a",
		renewed: make(chan struct{}, 4),
	}
	locker := &stubLocker{
		acquired: true,
		handle:   handle,
	}
	worker := &stubWorker{
		run: func(ctx context.Context, job jobs.Job) error {
			<-handle.renewed
			<-handle.renewed
			return nil
		},
	}
	s := New(locker, worker)

	result, err := s.RunOnce(context.Background(), jobs.Func{
		JobName: "reconcile_tenant_usage",
	}, Config{
		LockTTL:       100 * time.Millisecond,
		RenewInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if !result.Acquired || !result.Executed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if handle.renewCalls < 2 {
		t.Fatalf("renew calls = %d, want >= 2", handle.renewCalls)
	}
	if handle.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", handle.releaseCalls)
	}
}

func TestRunOnceReturnsRenewError(t *testing.T) {
	t.Parallel()

	handle := &stubLockHandle{
		key:      "job:cleanup_multipart",
		owner:    "node-a",
		renewErr: errors.New("lease lost"),
	}
	locker := &stubLocker{
		acquired: true,
		handle:   handle,
	}
	worker := &stubWorker{
		run: func(ctx context.Context, job jobs.Job) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	s := New(locker, worker)

	_, err := s.RunOnce(context.Background(), jobs.Func{
		JobName: "cleanup_multipart",
	}, Config{
		LockTTL:       100 * time.Millisecond,
		RenewInterval: 10 * time.Millisecond,
	})
	if err == nil || err.Error() != "lease lost" {
		t.Fatalf("RunOnce() error = %v, want %q", err, "lease lost")
	}
	if handle.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", handle.releaseCalls)
	}
}

type stubWorker struct {
	calls  int
	result jobs.Result
	run    func(context.Context, jobs.Job) error
}

func (s *stubWorker) Execute(ctx context.Context, job jobs.Job) (jobs.Result, error) {
	s.calls++
	if s.run != nil {
		return jobs.Result{}, s.run(ctx, job)
	}

	return s.result, nil
}

type stubLocker struct {
	acquired bool
	handle   ports.LockHandle
	err      error
	lastKey  string
	lastTTL  time.Duration
}

func (s *stubLocker) Acquire(_ context.Context, key string, ttl time.Duration) (ports.LockHandle, bool, error) {
	s.lastKey = key
	s.lastTTL = ttl
	return s.handle, s.acquired, s.err
}

type stubLockHandle struct {
	key          string
	owner        string
	renewCalls   int
	releaseCalls int
	renewErr     error
	releaseErr   error
	renewed      chan struct{}
}

func (s *stubLockHandle) Key() string {
	return s.key
}

func (s *stubLockHandle) Owner() string {
	return s.owner
}

func (s *stubLockHandle) Renew(context.Context, time.Duration) error {
	s.renewCalls++
	if s.renewed != nil {
		s.renewed <- struct{}{}
	}
	if s.renewErr != nil {
		return s.renewErr
	}

	return nil
}

func (s *stubLockHandle) Release(context.Context) error {
	s.releaseCalls++
	return s.releaseErr
}
