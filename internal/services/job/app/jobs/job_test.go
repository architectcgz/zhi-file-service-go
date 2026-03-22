package jobs_test

import (
	"context"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
)

func TestExecuteReturnsZeroResultForPlainJob(t *testing.T) {
	t.Parallel()

	calls := 0
	job := jobs.Func{
		JobName: " plain_job ",
		Run: func(context.Context) error {
			calls++
			return nil
		},
	}

	result, err := jobs.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if result.ItemsProcessed != 0 || result.RetryCount != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if job.Name() != "plain_job" {
		t.Fatalf("Name() = %q, want %q", job.Name(), "plain_job")
	}
}

func TestFuncNilRunIsNoop(t *testing.T) {
	t.Parallel()

	result, err := jobs.Execute(context.Background(), jobs.Func{JobName: jobs.JobNameCleanupMultipart})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ItemsProcessed != 0 || result.RetryCount != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}
