package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/app/outbox"
	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

func TestConsumerPublishesHandledEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 17, 0, 0, 0, time.UTC)
	reader := &stubOutboxReader{
		events: []ports.OutboxEvent{
			{EventID: "evt-1", EventType: "file.asset.delete_requested.v1"},
		},
	}
	consumer := outbox.NewConsumer(reader, map[string]outbox.Handler{
		"file.asset.delete_requested.v1": outbox.HandlerFunc(func(context.Context, ports.OutboxEvent) error {
			return nil
		}),
	}, clock.NewFixed(now), outbox.Config{
		BatchSize:  20,
		EventTypes: []string{"file.asset.delete_requested.v1"},
	})

	result, err := consumer.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Claimed != 1 || result.Published != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(reader.published) != 1 || reader.published[0] != "evt-1" {
		t.Fatalf("unexpected published events: %#v", reader.published)
	}
	if len(reader.claimQuery.EventTypes) != 1 || reader.claimQuery.EventTypes[0] != "file.asset.delete_requested.v1" {
		t.Fatalf("unexpected claim query: %#v", reader.claimQuery)
	}
}

func TestConsumerMarksFailedWhenHandlerErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 17, 10, 0, 0, time.UTC)
	reader := &stubOutboxReader{
		events: []ports.OutboxEvent{
			{EventID: "evt-2", EventType: "upload.session.failed.v1", RetryCount: 2},
		},
	}
	consumer := outbox.NewConsumer(reader, map[string]outbox.Handler{
		"upload.session.failed.v1": outbox.HandlerFunc(func(context.Context, ports.OutboxEvent) error {
			return errors.New("repair failed")
		}),
	}, clock.NewFixed(now), outbox.Config{
		RetryBackoff:    time.Minute,
		MaxRetryBackoff: 10 * time.Minute,
	})

	result, err := consumer.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Failed != 1 || len(reader.failed) != 1 {
		t.Fatalf("unexpected result or failures: result=%#v failed=%#v", result, reader.failed)
	}
	if !reader.failed[0].nextAttemptAt.Equal(now.Add(4 * time.Minute)) {
		t.Fatalf("unexpected next attempt at: %s", reader.failed[0].nextAttemptAt)
	}
}

func TestConsumerMarksFailedWhenNoHandlerIsRegistered(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 22, 17, 20, 0, 0, time.UTC)
	reader := &stubOutboxReader{
		events: []ports.OutboxEvent{
			{EventID: "evt-3", EventType: "unknown.event.v1"},
		},
	}
	consumer := outbox.NewConsumer(reader, nil, clock.NewFixed(now), outbox.Config{
		RetryBackoff:    30 * time.Second,
		MaxRetryBackoff: 10 * time.Minute,
	})

	result, err := consumer.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Failed != 1 || len(reader.failed) != 1 {
		t.Fatalf("unexpected result or failures: result=%#v failed=%#v", result, reader.failed)
	}
	if reader.failed[0].message != "no handler registered" {
		t.Fatalf("unexpected failure message: %#v", reader.failed[0])
	}
}

type stubOutboxReader struct {
	events      []ports.OutboxEvent
	claimQuery  ports.ClaimOutboxEventsQuery
	published   []string
	failed      []failedOutboxEvent
}

type failedOutboxEvent struct {
	eventID       string
	nextAttemptAt time.Time
	message       string
}

func (s *stubOutboxReader) ClaimPending(_ context.Context, query ports.ClaimOutboxEventsQuery) ([]ports.OutboxEvent, error) {
	s.claimQuery = query
	return s.events, nil
}

func (s *stubOutboxReader) MarkPublished(_ context.Context, eventID string, _ time.Time) error {
	s.published = append(s.published, eventID)
	return nil
}

func (s *stubOutboxReader) MarkFailed(_ context.Context, eventID string, nextAttemptAt time.Time, lastError string) error {
	s.failed = append(s.failed, failedOutboxEvent{
		eventID:       eventID,
		nextAttemptAt: nextAttemptAt,
		message:       lastError,
	})
	return nil
}
