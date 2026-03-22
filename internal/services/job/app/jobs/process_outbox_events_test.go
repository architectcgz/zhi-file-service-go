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

type stubJobConsumer struct {
	calls int
}

func (s *stubJobConsumer) RunOnce(context.Context) (outbox.Result, error) {
	s.calls++
	return outbox.Result{}, nil
}
