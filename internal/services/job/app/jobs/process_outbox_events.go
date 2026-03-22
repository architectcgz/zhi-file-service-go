package jobs

import (
	"context"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/outbox"
)

const JobNameProcessOutboxEvents = "process_outbox_events"

type outboxRunner interface {
	RunOnce(ctx context.Context) (outbox.Result, error)
}

type ProcessOutboxEventsJob struct {
	consumer outboxRunner
}

func NewProcessOutboxEventsJob(consumer outboxRunner) ProcessOutboxEventsJob {
	return ProcessOutboxEventsJob{consumer: consumer}
}

func (j ProcessOutboxEventsJob) Name() string {
	return JobNameProcessOutboxEvents
}

func (j ProcessOutboxEventsJob) Execute(ctx context.Context) error {
	_, err := j.ExecuteWithResult(ctx)
	return err
}

func (j ProcessOutboxEventsJob) ExecuteWithResult(ctx context.Context) (Result, error) {
	result, err := j.consumer.RunOnce(ctx)
	return Result{
		ItemsProcessed: result.Claimed,
		RetryCount:     result.Failed,
	}, err
}
