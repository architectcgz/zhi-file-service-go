package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	jobjobs "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	joboutbox "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/outbox"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

func TestBuildScheduledJobsIncludesPhase6Registrations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	jobs := buildScheduledJobs(config.JobConfig{
		DefaultBatchSize:              64,
		ExpireUploadSessionsInterval:  5 * time.Minute,
		RepairStuckCompletingInterval: 2 * time.Minute,
		ProcessOutboxEventsInterval:   20 * time.Second,
		FinalizeFileDeleteInterval:    time.Minute,
		CleanupMultipartInterval:      12 * time.Minute,
		CleanupOrphanBlobsInterval:    10 * time.Minute,
		ReconcileTenantUsageInterval:  30 * time.Minute,
		FileDeleteRetention:           24 * time.Hour,
	}, scheduledJobDependencies{
		UploadSessions: mockUploadSessionRepository{},
		FileCleanup:    mockFileCleanupRepository{},
		BlobRepo:       mockBlobRepository{},
		MultipartRepo:  mockMultipartRepository{},
		TenantUsage:    mockTenantUsageRepository{},
		OutboxConsumer: mockOutboxRunner{},
	}, clock.NewFixed(now))

	if len(jobs) != 7 {
		t.Fatalf("len(jobs) = %d, want 7", len(jobs))
	}

	wantIntervals := map[string]time.Duration{
		jobjobs.JobNameExpireUploadSessions:  5 * time.Minute,
		jobjobs.JobNameRepairStuckCompleting: 2 * time.Minute,
		jobjobs.JobNameProcessOutboxEvents:   20 * time.Second,
		jobjobs.JobNameFinalizeFileDelete:    time.Minute,
		jobjobs.JobNameCleanupMultipart:      12 * time.Minute,
		jobjobs.JobNameCleanupOrphanBlobs:    10 * time.Minute,
		jobjobs.JobNameReconcileTenantUsage:  30 * time.Minute,
	}

	for _, scheduled := range jobs {
		if scheduled.Job == nil {
			t.Fatal("scheduled job = nil, want non-nil")
		}
		wantInterval, ok := wantIntervals[scheduled.Job.Name()]
		if !ok {
			t.Fatalf("unexpected job registered: %q", scheduled.Job.Name())
		}
		if scheduled.Interval != wantInterval {
			t.Fatalf("%s interval = %s, want %s", scheduled.Job.Name(), scheduled.Interval, wantInterval)
		}
		delete(wantIntervals, scheduled.Job.Name())
	}

	if len(wantIntervals) != 0 {
		t.Fatalf("missing scheduled jobs: %#v", wantIntervals)
	}
}

func TestNewOutboxHandlersSupportCurrentEventTypes(t *testing.T) {
	t.Parallel()

	handlers := newOutboxHandlers()
	if len(handlers) != 2 {
		t.Fatalf("len(handlers) = %d, want 2", len(handlers))
	}

	tests := []struct {
		name      string
		eventType string
		payload   map[string]any
	}{
		{
			name:      "file delete request",
			eventType: "file.asset.delete_requested.v1",
			payload: map[string]any{
				"fileId":       "file-1",
				"blobObjectId": "blob-1",
				"producer":     "admin-service",
			},
		},
		{
			name:      "upload session failed",
			eventType: "upload.session.failed.v1",
			payload: map[string]any{
				"uploadSessionId": "session-1",
				"failureCode":     "UPLOAD_HASH_MISMATCH",
				"producer":        "upload-service",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, ok := handlers[tt.eventType]
			if !ok {
				t.Fatalf("handler for %q not registered", tt.eventType)
			}
			if err := handler.Handle(context.Background(), ports.OutboxEvent{
				EventID:   "evt-1",
				EventType: tt.eventType,
				Payload:   tt.payload,
			}); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
		})
	}
}

func TestNewOutboxHandlersRejectUnexpectedProducer(t *testing.T) {
	t.Parallel()

	handlers := newOutboxHandlers()
	handler, ok := handlers[eventTypeUploadSessionFailed]
	if !ok {
		t.Fatalf("handler for %q not registered", eventTypeUploadSessionFailed)
	}

	err := handler.Handle(context.Background(), ports.OutboxEvent{
		EventID:   "evt-invalid-1",
		EventType: eventTypeUploadSessionFailed,
		Payload: map[string]any{
			"uploadSessionId": "session-1",
			"failureCode":     "UPLOAD_HASH_MISMATCH",
			"producer":        "other-service",
		},
	})
	if err == nil {
		t.Fatal("expected handler error for unexpected producer")
	}
}

type mockUploadSessionRepository struct{}

func (mockUploadSessionRepository) ExpirePendingSessions(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (mockUploadSessionRepository) RepairStuckCompleting(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

type mockFileCleanupRepository struct{}

func (mockFileCleanupRepository) FinalizeDeletedFiles(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

type mockBlobRepository struct{}

func (mockBlobRepository) CleanupOrphanBlobs(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

type mockMultipartRepository struct{}

func (mockMultipartRepository) CleanupMultipartUploads(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

type mockTenantUsageRepository struct{}

func (mockTenantUsageRepository) ReconcileTenantUsage(context.Context, int) (int, error) {
	return 0, nil
}

type mockOutboxRunner struct{}

func (mockOutboxRunner) RunOnce(context.Context) (joboutbox.Result, error) {
	return joboutbox.Result{}, nil
}
