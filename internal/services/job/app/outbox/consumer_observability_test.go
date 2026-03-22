package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/observability"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/outbox"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

func TestConsumerRecordsObservedFailureAndHealth(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 18, 0, 0, 0, time.UTC)
	logger := &stubOutboxLogger{}
	tracer := &stubOutboxTracer{}
	health := observability.NewMemoryHealthStore()
	observer := observability.NewObserver(observability.Options{
		Logger: logger,
		Tracer: tracer,
		Health: health,
	})

	reader := &stubOutboxReader{
		events: []ports.OutboxEvent{
			{EventID: "evt-9", EventType: "upload.session.failed.v1", RetryCount: 1},
		},
	}
	consumer := outbox.NewConsumer(reader, map[string]outbox.Handler{
		"upload.session.failed.v1": outbox.HandlerFunc(func(context.Context, ports.OutboxEvent) error {
			return errors.New("repair failed")
		}),
	}, clock.NewFixed(now), outbox.Config{
		RetryBackoff: time.Minute,
		Observer:     observer,
	})

	result, err := consumer.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Failed != 1 {
		t.Fatalf("result.Failed = %d, want 1", result.Failed)
	}
	if tracer.started[0] != "job.process_outbox_events.consume_batch" {
		t.Fatalf("unexpected span names: %#v", tracer.started)
	}
	if !logger.hasMessage("outbox_event_retry_scheduled") {
		t.Fatalf("expected retry log, got %#v", logger.entries)
	}

	outboxHealth := health.Snapshot().Outbox
	if outboxHealth.Status != observability.StatusFailed {
		t.Fatalf("outbox status = %q, want %q", outboxHealth.Status, observability.StatusFailed)
	}
	if outboxHealth.Failed != 1 || outboxHealth.RetryCount != 1 || outboxHealth.LastError != "repair failed" {
		t.Fatalf("unexpected outbox health: %#v", outboxHealth)
	}
}

type stubOutboxLogger struct {
	entries []string
}

func (s *stubOutboxLogger) Log(_ context.Context, _ observability.Level, message string, _ observability.Fields) {
	s.entries = append(s.entries, message)
}

func (s *stubOutboxLogger) hasMessage(message string) bool {
	for _, entry := range s.entries {
		if entry == message {
			return true
		}
	}
	return false
}

type stubOutboxTracer struct {
	started []string
}

func (s *stubOutboxTracer) Start(ctx context.Context, name string, _ observability.Fields) (context.Context, observability.Span) {
	s.started = append(s.started, name)
	return ctx, &stubOutboxSpan{}
}

type stubOutboxSpan struct{}

func (s *stubOutboxSpan) AddFields(observability.Fields) {}

func (s *stubOutboxSpan) RecordError(error) {}

func (s *stubOutboxSpan) End() {}
