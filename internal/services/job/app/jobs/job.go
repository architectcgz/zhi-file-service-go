package jobs

import (
	"context"
	"strings"
)

type Job interface {
	Name() string
	Execute(ctx context.Context) error
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
