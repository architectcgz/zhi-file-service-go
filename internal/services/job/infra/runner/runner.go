package runner

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	jobobs "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/observability"
	jobscheduler "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/scheduler"
)

var ErrSchedulerRequired = errors.New("scheduler is required")

type Scheduler interface {
	RunOnce(context.Context, jobs.Job, jobscheduler.Config) (jobscheduler.RunResult, error)
}

type ScheduledJob struct {
	Job      jobs.Job
	Interval time.Duration
}

type RuntimeConfig struct {
	Scheduler     Scheduler
	Jobs          []ScheduledJob
	LockTTL       time.Duration
	RenewInterval time.Duration
	Observer      *jobobs.Observer
}

type Runtime struct {
	cfg RuntimeConfig

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func New(cfg RuntimeConfig) *Runtime {
	return &Runtime{cfg: cfg}
}

func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return errors.New("runtime already started")
	}
	if len(r.cfg.Jobs) > 0 && r.cfg.Scheduler == nil {
		return ErrSchedulerRequired
	}

	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.started = true

	for _, scheduled := range r.cfg.Jobs {
		if scheduled.Job == nil || scheduled.Interval <= 0 {
			continue
		}

		r.wg.Add(1)
		go r.loop(runCtx, scheduled)
	}

	return nil
}

func (r *Runtime) Ready(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return errors.New("job runtime not started")
	}
	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}

	cancel := r.cancel
	r.cancel = nil
	r.started = false
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runtime) loop(ctx context.Context, scheduled ScheduledJob) {
	defer r.wg.Done()

	run := func() {
		_, _ = r.cfg.Scheduler.RunOnce(ctx, scheduled.Job, jobscheduler.Config{
			LockTTL:       r.cfg.LockTTL,
			RenewInterval: r.cfg.RenewInterval,
			Observer:      r.cfg.Observer,
		})
	}

	run()

	ticker := time.NewTicker(scheduled.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
