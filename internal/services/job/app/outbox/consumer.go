package outbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/job/ports"
	"github.com/architectcgz/zhi-file-service-go/pkg/clock"
)

type Handler interface {
	Handle(ctx context.Context, event ports.OutboxEvent) error
}

type HandlerFunc func(context.Context, ports.OutboxEvent) error

func (f HandlerFunc) Handle(ctx context.Context, event ports.OutboxEvent) error {
	return f(ctx, event)
}

type Config struct {
	BatchSize       int
	EventTypes      []string
	RetryBackoff    time.Duration
	MaxRetryBackoff time.Duration
}

type Result struct {
	Claimed   int
	Published int
	Failed    int
}

type Consumer struct {
	reader          ports.OutboxReader
	handlers        map[string]Handler
	clock           clock.Clock
	batchSize       int
	eventTypes      []string
	retryBackoff    time.Duration
	maxRetryBackoff time.Duration
}

func NewConsumer(
	reader ports.OutboxReader,
	handlers map[string]Handler,
	clk clock.Clock,
	cfg Config,
) Consumer {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = time.Minute
	}
	if cfg.MaxRetryBackoff <= 0 {
		cfg.MaxRetryBackoff = time.Hour
	}

	normalizedHandlers := make(map[string]Handler, len(handlers))
	for eventType, handler := range handlers {
		eventType = strings.TrimSpace(eventType)
		if eventType == "" || handler == nil {
			continue
		}
		normalizedHandlers[eventType] = handler
	}

	return Consumer{
		reader:          reader,
		handlers:        normalizedHandlers,
		clock:           clk,
		batchSize:       cfg.BatchSize,
		eventTypes:      normalizeEventTypes(cfg.EventTypes),
		retryBackoff:    cfg.RetryBackoff,
		maxRetryBackoff: cfg.MaxRetryBackoff,
	}
}

func (c Consumer) RunOnce(ctx context.Context) (Result, error) {
	events, err := c.reader.ClaimPending(ctx, ports.ClaimOutboxEventsQuery{
		EventTypes: c.eventTypes,
		DueBefore:  c.clock.Now(),
		Limit:      c.batchSize,
	})
	if err != nil {
		return Result{}, err
	}

	now := c.clock.Now()
	result := Result{Claimed: len(events)}
	for _, event := range events {
		handler, ok := c.handlers[event.EventType]
		if !ok {
			if err := c.reader.MarkFailed(ctx, event.EventID, now.Add(c.retryDelay(event.RetryCount)), "no handler registered"); err != nil {
				return result, err
			}
			result.Failed++
			continue
		}

		if err := handler.Handle(ctx, event); err != nil {
			if markErr := c.reader.MarkFailed(ctx, event.EventID, now.Add(c.retryDelay(event.RetryCount)), err.Error()); markErr != nil {
				return result, fmt.Errorf("handle outbox event: %w; mark failed: %v", err, markErr)
			}
			result.Failed++
			continue
		}

		if err := c.reader.MarkPublished(ctx, event.EventID, now); err != nil {
			return result, err
		}
		result.Published++
	}

	return result, nil
}

func (c Consumer) retryDelay(retryCount int) time.Duration {
	delay := c.retryBackoff
	for i := 0; i < retryCount; i++ {
		if delay >= c.maxRetryBackoff/2 {
			return c.maxRetryBackoff
		}
		delay *= 2
	}
	if delay > c.maxRetryBackoff {
		return c.maxRetryBackoff
	}

	return delay
}

func normalizeEventTypes(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}
