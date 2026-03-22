package scheduler

import (
	"context"
	"errors"
	"sync"
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
		run: func(ctx context.Context, job jobs.Job) (jobs.Result, error) {
			<-handle.renewed
			<-handle.renewed
			return jobs.Result{}, nil
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
		run: func(ctx context.Context, job jobs.Job) (jobs.Result, error) {
			<-ctx.Done()
			return jobs.Result{}, ctx.Err()
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

func TestRunOnceReturnsAcquireError(t *testing.T) {
	t.Parallel()

	locker := &stubLocker{err: errors.New("lock backend down")}
	worker := &stubWorker{}
	s := New(locker, worker)

	result, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupOrphanBlobs}, Config{
		LockTTL: 30 * time.Second,
	})
	if err == nil || err.Error() != "lock backend down" {
		t.Fatalf("RunOnce() error = %v, want %q", err, "lock backend down")
	}
	if result.Acquired || result.Executed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if worker.calls != 0 {
		t.Fatalf("worker calls = %d, want 0", worker.calls)
	}
}

func TestRunOnceSkipsWhenAcquireReturnsNilHandle(t *testing.T) {
	t.Parallel()

	locker := &stubLocker{acquired: true}
	worker := &stubWorker{}
	s := New(locker, worker)

	result, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupMultipart}, Config{
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
}

func TestRunOnceUsesDefaultRenewInterval(t *testing.T) {
	t.Parallel()

	handle := &stubLockHandle{
		key:     "job:repair_stuck_completing",
		owner:   "node-a",
		renewed: make(chan struct{}, 1),
	}
	locker := &stubLocker{
		acquired: true,
		handle:   handle,
	}
	worker := &stubWorker{
		run: func(ctx context.Context, job jobs.Job) (jobs.Result, error) {
			select {
			case <-handle.renewed:
				return jobs.Result{}, nil
			case <-time.After(200 * time.Millisecond):
				t.Fatal("timed out waiting for default renew interval")
				return jobs.Result{}, nil
			}
		},
	}
	s := New(locker, worker)

	result, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameRepairStuckCompleting}, Config{
		LockTTL: 40 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if !result.Acquired || !result.Executed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if handle.renewCalls == 0 {
		t.Fatalf("renew calls = %d, want > 0", handle.renewCalls)
	}
}

func TestRunOnceReturnsReleaseError(t *testing.T) {
	t.Parallel()

	handle := &stubLockHandle{
		key:        "job:finalize_file_delete",
		owner:      "node-a",
		releaseErr: errors.New("release failed"),
	}
	locker := &stubLocker{
		acquired: true,
		handle:   handle,
	}
	s := New(locker, &stubWorker{})

	_, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameFinalizeFileDelete}, Config{
		LockTTL:       30 * time.Second,
		RenewInterval: time.Hour,
	})
	if err == nil || err.Error() != "release failed" {
		t.Fatalf("RunOnce() error = %v, want %q", err, "release failed")
	}
	if handle.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", handle.releaseCalls)
	}
}

func TestRunOnceWrapsReleaseErrorAfterWorkerFailure(t *testing.T) {
	t.Parallel()

	handle := &stubLockHandle{
		key:        "job:cleanup_orphan_blobs",
		owner:      "node-a",
		releaseErr: errors.New("release failed"),
	}
	locker := &stubLocker{
		acquired: true,
		handle:   handle,
	}
	s := New(locker, &stubWorker{
		run: func(context.Context, jobs.Job) (jobs.Result, error) {
			return jobs.Result{}, errors.New("worker failed")
		},
	})

	_, err := s.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupOrphanBlobs}, Config{
		LockTTL:       30 * time.Second,
		RenewInterval: time.Hour,
	})
	if err == nil || err.Error() != "worker failed; release lock: release failed" {
		t.Fatalf("RunOnce() error = %v, want %q", err, "worker failed; release lock: release failed")
	}
}

func TestRunOnceAllowsTakeoverAfterLeaseLoss(t *testing.T) {
	t.Parallel()

	locker := newTakeoverLocker("cleanup_multipart")
	workerDone := make(chan struct{})
	firstWorker := &stubWorker{
		run: func(ctx context.Context, job jobs.Job) (jobs.Result, error) {
			defer close(workerDone)
			<-ctx.Done()
			return jobs.Result{}, ctx.Err()
		},
	}
	firstScheduler := New(locker, firstWorker)

	firstErrCh := make(chan error, 1)
	go func() {
		_, err := firstScheduler.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupMultipart}, Config{
			LockTTL:       40 * time.Millisecond,
			RenewInterval: 10 * time.Millisecond,
		})
		firstErrCh <- err
	}()

	select {
	case err := <-firstErrCh:
		if err == nil || err.Error() != "lease lost" {
			t.Fatalf("first RunOnce() error = %v, want %q", err, "lease lost")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for first scheduler to lose lease")
	}

	select {
	case <-workerDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for first worker shutdown")
	}

	secondWorker := &stubWorker{}
	secondScheduler := New(locker, secondWorker)
	result, err := secondScheduler.RunOnce(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupMultipart}, Config{
		LockTTL:       40 * time.Millisecond,
		RenewInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("second RunOnce() error = %v", err)
	}
	if !result.Acquired || !result.Executed {
		t.Fatalf("unexpected second result: %#v", result)
	}
	if secondWorker.calls != 1 {
		t.Fatalf("second worker calls = %d, want 1", secondWorker.calls)
	}
}

type stubWorker struct {
	calls  int
	result jobs.Result
	err    error
	run    func(context.Context, jobs.Job) (jobs.Result, error)
}

func (s *stubWorker) Execute(ctx context.Context, job jobs.Job) (jobs.Result, error) {
	s.calls++
	if s.run != nil {
		return s.run(ctx, job)
	}

	return s.result, s.err
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

type takeoverLocker struct {
	mu        sync.Mutex
	key       string
	active    *takeoverHandle
	acquires  int
	releaseCh chan struct{}
}

func newTakeoverLocker(jobName string) *takeoverLocker {
	return &takeoverLocker{
		key:       lockKey(jobName),
		releaseCh: make(chan struct{}, 1),
	}
}

func (l *takeoverLocker) Acquire(_ context.Context, key string, _ time.Duration) (ports.LockHandle, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if key != l.key || l.active != nil {
		return nil, false, nil
	}

	l.acquires++
	handle := &takeoverHandle{
		locker: l,
		key:    key,
		owner:  "node",
	}
	if l.acquires == 1 {
		handle.renewErr = errors.New("lease lost")
	}
	l.active = handle
	return handle, true, nil
}

type takeoverHandle struct {
	locker       *takeoverLocker
	key          string
	owner        string
	renewErr     error
	releasedOnce bool
}

func (h *takeoverHandle) Key() string {
	return h.key
}

func (h *takeoverHandle) Owner() string {
	return h.owner
}

func (h *takeoverHandle) Renew(context.Context, time.Duration) error {
	if h.renewErr == nil {
		return nil
	}
	h.releaseActive()
	return h.renewErr
}

func (h *takeoverHandle) Release(context.Context) error {
	h.releaseActive()
	return nil
}

func (h *takeoverHandle) releaseActive() {
	h.locker.mu.Lock()
	defer h.locker.mu.Unlock()
	if h.releasedOnce {
		return
	}
	if h.locker.active == h {
		h.locker.active = nil
	}
	h.releasedOnce = true
}
