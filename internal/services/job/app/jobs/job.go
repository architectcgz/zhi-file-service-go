package jobs

import (
	"context"
	"strings"
)

type Job interface {
	Name() string
	Execute(ctx context.Context) error
}

type Result struct {
	ItemsProcessed int
	RetryCount     int
}

type ResultJob interface {
	Job
	ExecuteWithResult(ctx context.Context) (Result, error)
}

func Execute(ctx context.Context, job Job) (Result, error) {
	if resultJob, ok := job.(ResultJob); ok {
		return resultJob.ExecuteWithResult(ctx)
	}

	err := job.Execute(ctx)
	return Result{}, err
}

type Func struct {
	JobName string
	Run     func(context.Context) error
}

func (f Func) Name() string {
	return strings.TrimSpace(f.JobName)
}

func (f Func) Execute(ctx context.Context) error {
	if f.Run == nil {
		return nil
	}

	return f.Run(ctx)
}
