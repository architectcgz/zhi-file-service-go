package runtime

import (
	"context"
	"fmt"
	"sort"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	jobjobs "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/jobs"
	jobobs "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/observability"
	joboutbox "github.com/architectcgz/zhi-file-service-go/internal/services/job/app/outbox"
	jobrunner "github.com/architectcgz/zhi-file-service-go/internal/services/job/infra/runner"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

const (
	eventTypeUploadSessionFailed = "upload.session.failed.v1"
	eventTypeFileDeleteRequested = "file.asset.delete_requested.v1"
	payloadFieldProducer         = "producer"
	payloadFieldUploadSessionID  = "uploadSessionId"
	payloadFieldFailureCode      = "failureCode"
	payloadFieldFileID           = "fileId"
	payloadFieldBlobObjectID     = "blobObjectId"
	expectedUploadProducer       = "upload-service"
	expectedAdminDeleteProducer  = "admin-service"
)

type outboxRunner interface {
	RunOnce(ctx context.Context) (joboutbox.Result, error)
}

type scheduledJobDependencies struct {
	UploadSessions ports.UploadSessionRepository
	FileCleanup    ports.FileCleanupRepository
	BlobRepo       ports.BlobRepository
	MultipartRepo  ports.MultipartRepository
	TenantUsage    ports.TenantUsageRepository
	OutboxConsumer outboxRunner
}

func buildScheduledJobs(cfg config.JobConfig, deps scheduledJobDependencies, clk clock.Clock) []jobrunner.ScheduledJob {
	return []jobrunner.ScheduledJob{
		{
			Job: jobjobs.NewExpireUploadSessionsJob(deps.UploadSessions, clk, jobjobs.ExpireUploadSessionsConfig{
				BatchSize: cfg.DefaultBatchSize,
			}),
			Interval: cfg.ExpireUploadSessionsInterval,
		},
		{
			Job: jobjobs.NewRepairStuckCompletingJob(deps.UploadSessions, clk, jobjobs.RepairStuckCompletingConfig{
				BatchSize: cfg.DefaultBatchSize,
			}),
			Interval: cfg.RepairStuckCompletingInterval,
		},
		{
			Job:      jobjobs.NewProcessOutboxEventsJob(deps.OutboxConsumer),
			Interval: cfg.ProcessOutboxEventsInterval,
		},
		{
			Job: jobjobs.NewFinalizeFileDeleteJob(deps.FileCleanup, clk, jobjobs.FinalizeFileDeleteConfig{
				BatchSize: cfg.DefaultBatchSize,
				Retention: cfg.FileDeleteRetention,
			}),
			Interval: cfg.FinalizeFileDeleteInterval,
		},
		{
			Job: jobjobs.NewCleanupMultipartJob(deps.MultipartRepo, clk, jobjobs.CleanupMultipartConfig{
				BatchSize: cfg.DefaultBatchSize,
			}),
			Interval: cfg.CleanupMultipartInterval,
		},
		{
			Job: jobjobs.NewCleanupOrphanBlobsJob(deps.BlobRepo, clk, jobjobs.CleanupOrphanBlobsConfig{
				BatchSize: cfg.DefaultBatchSize,
			}),
			Interval: cfg.CleanupOrphanBlobsInterval,
		},
		{
			Job: jobjobs.NewReconcileTenantUsageJob(deps.TenantUsage, jobjobs.ReconcileTenantUsageConfig{
				BatchSize: cfg.DefaultBatchSize,
			}),
			Interval: cfg.ReconcileTenantUsageInterval,
		},
	}
}

func newOutboxConsumer(
	reader ports.OutboxReader,
	observer *jobobs.Observer,
	clk clock.Clock,
	batchSize int,
) joboutbox.Consumer {
	handlers := newOutboxHandlers()
	return joboutbox.NewConsumer(reader, handlers, clk, joboutbox.Config{
		BatchSize:  batchSize,
		EventTypes: supportedOutboxEventTypes(handlers),
		Observer:   observer,
	})
}

func newOutboxHandlers() map[string]joboutbox.Handler {
	return map[string]joboutbox.Handler{
		eventTypeFileDeleteRequested: joboutbox.HandlerFunc(handleFileDeleteRequested),
		eventTypeUploadSessionFailed: joboutbox.HandlerFunc(handleUploadSessionFailed),
	}
}

func supportedOutboxEventTypes(handlers map[string]joboutbox.Handler) []string {
	result := make([]string, 0, len(handlers))
	for eventType, handler := range handlers {
		if handler == nil {
			continue
		}
		result = append(result, eventType)
	}
	sort.Strings(result)
	return result
}

func handleFileDeleteRequested(_ context.Context, event ports.OutboxEvent) error {
	if err := requirePayloadString(event.Payload, payloadFieldProducer, expectedAdminDeleteProducer); err != nil {
		return err
	}
	if err := requirePayloadString(event.Payload, payloadFieldFileID, ""); err != nil {
		return err
	}
	return requirePayloadString(event.Payload, payloadFieldBlobObjectID, "")
}

func handleUploadSessionFailed(_ context.Context, event ports.OutboxEvent) error {
	if err := requirePayloadString(event.Payload, payloadFieldProducer, expectedUploadProducer); err != nil {
		return err
	}
	if err := requirePayloadString(event.Payload, payloadFieldUploadSessionID, ""); err != nil {
		return err
	}
	return requirePayloadString(event.Payload, payloadFieldFailureCode, "")
}

func requirePayloadString(payload map[string]any, key string, expected string) error {
	if payload == nil {
		return fmt.Errorf("outbox payload is required")
	}
	raw, ok := payload[key]
	if !ok {
		return fmt.Errorf("outbox payload %q is required", key)
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return fmt.Errorf("outbox payload %q must be a non-empty string", key)
	}
	if expected != "" && value != expected {
		return fmt.Errorf("outbox payload %q must be %q", key, expected)
	}
	return nil
}
