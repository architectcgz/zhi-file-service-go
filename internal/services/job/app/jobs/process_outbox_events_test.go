package jobs_test

import (
	"context"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/outbox"
)

func TestProcessOutboxEventsJobDelegatesToConsumer(t *testing.T) {
	t.Parallel()

	consumer := &stubJobConsumer{}
	job := jobs.NewProcessOutboxEventsJob(consumer)

	if err := job.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if consumer.calls != 1 {
		t.Fatalf("consumer calls = %d, want 1", consumer.calls)
	}
	if job.Name() != jobs.JobNameProcessOutboxEvents {
		t.Fatalf("Name() = %q, want %q", job.Name(), jobs.JobNameProcessOutboxEvents)
	}
}

func TestProcessOutboxEventsJobReturnsExecutionResult(t *testing.T) {
	t.Parallel()

	consumer := &stubJobConsumer{
		result: outbox.Result{
			Claimed:   4,
			Published: 3,
			Failed:    1,
		},
	}
	job := jobs.NewProcessOutboxEventsJob(consumer)

	result, err := jobs.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ItemsProcessed != 4 {
		t.Fatalf("ItemsProcessed = %d, want 4", result.ItemsProcessed)
	}
	if result.RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", result.RetryCount)
	}
}

type stubJobConsumer struct {
	calls  int
	result outbox.Result
}

func (s *stubJobConsumer) RunOnce(context.Context) (outbox.Result, error) {
	s.calls++
	return s.result, nil
}
