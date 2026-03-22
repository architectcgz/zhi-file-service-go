package jobs_test

import (
	"context"
	"errors"
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

func TestProcessOutboxEventsJobReturnsZeroResultWhenNoEventsClaimed(t *testing.T) {
	t.Parallel()

	job := jobs.NewProcessOutboxEventsJob(&stubJobConsumer{})

	result, err := jobs.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ItemsProcessed != 0 || result.RetryCount != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestProcessOutboxEventsJobPropagatesConsumerErrorAndCounts(t *testing.T) {
	t.Parallel()

	job := jobs.NewProcessOutboxEventsJob(&stubJobConsumer{
		result: outbox.Result{
			Claimed: 2,
			Failed:  2,
		},
		err: errors.New("consumer failed"),
	})

	result, err := jobs.Execute(context.Background(), job)
	if err == nil || err.Error() != "consumer failed" {
		t.Fatalf("Execute() error = %v, want %q", err, "consumer failed")
	}
	if result.ItemsProcessed != 2 {
		t.Fatalf("ItemsProcessed = %d, want 2", result.ItemsProcessed)
	}
	if result.RetryCount != 2 {
		t.Fatalf("RetryCount = %d, want 2", result.RetryCount)
	}
}

type stubJobConsumer struct {
	calls  int
	result outbox.Result
	err    error
}

func (s *stubJobConsumer) RunOnce(context.Context) (outbox.Result, error) {
	s.calls++
	return s.result, s.err
}
