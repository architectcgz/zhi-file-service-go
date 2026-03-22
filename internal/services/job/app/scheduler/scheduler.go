package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/observability"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
)

type Worker interface {
	Execute(ctx context.Context, job jobs.Job) (jobs.Result, error)
}

type InlineWorker struct{}

func (InlineWorker) Execute(ctx context.Context, job jobs.Job) (jobs.Result, error) {
	return jobs.Execute(ctx, job)
}

type Config struct {
	LockTTL       time.Duration
	RenewInterval time.Duration
	Observer      *observability.Observer
}

type RunResult struct {
	Acquired bool
	Executed bool
}

type Scheduler struct {
	locker ports.DistributedLocker
	worker Worker
}

func New(locker ports.DistributedLocker, worker Worker) Scheduler {
	if worker == nil {
		worker = InlineWorker{}
	}

	return Scheduler{
		locker: locker,
		worker: worker,
	}
}

func (s Scheduler) RunOnce(ctx context.Context, job jobs.Job, cfg Config) (RunResult, error) {
	if s.locker == nil {
		return RunResult{}, fmt.Errorf("distributed locker is required")
	}
	if job == nil || strings.TrimSpace(job.Name()) == "" {
		return RunResult{}, fmt.Errorf("job name is required")
	}
	if cfg.LockTTL <= 0 {
		return RunResult{}, fmt.Errorf("lock ttl must be > 0")
	}
	if cfg.RenewInterval <= 0 {
		cfg.RenewInterval = cfg.LockTTL / 2
		if cfg.RenewInterval <= 0 {
			cfg.RenewInterval = time.Second
		}
	}

	handle, acquired, err := s.locker.Acquire(ctx, lockKey(job.Name()), cfg.LockTTL)
	if err != nil {
		cfg.Observer.RecordJobLockAcquireFailure(ctx, job.Name(), err)
		return RunResult{}, err
	}
	if !acquired || handle == nil {
		cfg.Observer.RecordJobLockAcquireFailure(ctx, job.Name(), nil)
		return RunResult{}, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runCtx, finishObservation := cfg.Observer.StartJobRun(runCtx, job.Name())

	done := make(chan struct{})
	renewErrCh := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(cfg.RenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := handle.Renew(context.WithoutCancel(ctx), cfg.LockTTL); err != nil {
					select {
					case renewErrCh <- err:
					default:
					}
					cancel()
					return
				}
			}
		}
	}()

	jobResult, execErr := s.worker.Execute(runCtx, job)
	close(done)

	select {
	case renewErr := <-renewErrCh:
		if execErr == nil || errors.Is(execErr, context.Canceled) {
			execErr = renewErr
		}
	default:
	}

	if releaseErr := handle.Release(context.WithoutCancel(ctx)); releaseErr != nil {
		if execErr == nil {
			execErr = releaseErr
		} else {
			execErr = fmt.Errorf("%w; release lock: %v", execErr, releaseErr)
		}
	}

	finishObservation(observability.JobRun{
		LockAcquired:   true,
		ItemsProcessed: jobResult.ItemsProcessed,
		RetryCount:     jobResult.RetryCount,
		Error:          execErr,
	})

	return RunResult{
		Acquired: true,
		Executed: true,
	}, execErr
}

func lockKey(jobName string) string {
	return "job:" + strings.TrimSpace(jobName)
}
