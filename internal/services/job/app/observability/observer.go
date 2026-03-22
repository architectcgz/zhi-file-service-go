package observability

import (
	"context"
	"time"
)

type Fields map[string]any

type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped"
)

type Metrics interface {
	AddCounter(name string, value int64, fields Fields)
	ObserveDuration(name string, duration time.Duration, fields Fields)
}

type Logger interface {
	Log(ctx context.Context, level Level, message string, fields Fields)
}

type Tracer interface {
	Start(ctx context.Context, name string, fields Fields) (context.Context, Span)
}

type Span interface {
	AddFields(fields Fields)
	RecordError(err error)
	End()
}

type HealthReporter interface {
	RecordJob(health JobHealth)
	RecordOutbox(health OutboxHealth)
}

type Options struct {
	Metrics Metrics
	Logger  Logger
	Tracer  Tracer
	Health  HealthReporter
}

type Observer struct {
	metrics Metrics
	logger  Logger
	tracer  Tracer
	health  HealthReporter
}

type JobRun struct {
	Status         Status
	LockAcquired   bool
	ItemsProcessed int
	RetryCount     int
	Error          error
	Duration       time.Duration
}

type OutboxRun struct {
	Status     Status
	Claimed    int
	Published  int
	Failed     int
	RetryCount int
	Error      error
	Duration   time.Duration
}

func NewObserver(opts Options) *Observer {
	return &Observer{
		metrics: fallbackMetrics(opts.Metrics),
		logger:  fallbackLogger(opts.Logger),
		tracer:  fallbackTracer(opts.Tracer),
		health:  fallbackHealth(opts.Health),
	}
}

func (o *Observer) RecordJobLockAcquireFailure(ctx context.Context, jobName string, err error) {
	if o == nil {
		return
	}

	now := time.Now()
	status := StatusSkipped
	level := LevelInfo
	message := "job_lock_not_acquired"
	if err != nil {
		status = StatusFailed
		level = LevelWarn
		message = "job_lock_acquire_failed"
	}

	fields := Fields{
		"job_name":      jobName,
		"status":        status,
		"lock_acquired": false,
	}
	if err != nil {
		fields["error"] = err.Error()
	}

	o.metrics.AddCounter("job_lock_acquire_failed_total", 1, Fields{
		"job_name": jobName,
		"status":   status,
	})
	o.logger.Log(ctx, level, message, fields)
	o.health.RecordJob(JobHealth{
		JobName:        jobName,
		Status:         status,
		LastStartedAt:  now,
		LastFinishedAt: now,
		LastDuration:   0,
		LastError:      errorString(err),
		LockAcquired:   false,
	})
}

func (o *Observer) StartJobRun(ctx context.Context, jobName string) (context.Context, func(JobRun)) {
	if o == nil {
		return ctx, func(JobRun) {}
	}

	startedAt := time.Now()
	baseFields := Fields{"job_name": jobName}
	ctx, span := o.tracer.Start(ctx, "job."+jobName, baseFields)
	o.logger.Log(ctx, LevelInfo, "job_run_started", baseFields)

	return ctx, func(run JobRun) {
		duration := run.Duration
		if duration <= 0 {
			duration = time.Since(startedAt)
		}
		finishedAt := startedAt.Add(duration)
		if run.Status == "" {
			run.Status = statusFromError(run.Error)
		}

		fields := mergeFields(baseFields, Fields{
			"status":          run.Status,
			"lock_acquired":   run.LockAcquired,
			"items_processed": run.ItemsProcessed,
			"retry_count":     run.RetryCount,
		})
		if run.Error != nil {
			fields["error"] = run.Error.Error()
		}

		o.metrics.AddCounter("job_run_total", 1, Fields{
			"job_name": jobName,
			"status":   run.Status,
		})
		if run.Error != nil {
			o.metrics.AddCounter("job_run_failed_total", 1, Fields{
				"job_name": jobName,
			})
		}
		o.metrics.ObserveDuration("job_duration_seconds", duration, Fields{
			"job_name": jobName,
			"status":   run.Status,
		})
		if run.ItemsProcessed > 0 {
			o.metrics.AddCounter("job_items_processed_total", int64(run.ItemsProcessed), Fields{
				"job_name": jobName,
			})
		}
		if run.RetryCount > 0 {
			o.metrics.AddCounter("job_retry_total", int64(run.RetryCount), Fields{
				"job_name": jobName,
			})
		}

		span.AddFields(fields)
		if run.Error != nil {
			span.RecordError(run.Error)
		}
		span.End()

		level := LevelInfo
		message := "job_run_completed"
		if run.Error != nil {
			level = LevelError
			message = "job_run_failed"
		}
		o.logger.Log(ctx, level, message, fields)
		o.health.RecordJob(JobHealth{
			JobName:        jobName,
			Status:         run.Status,
			LastStartedAt:  startedAt,
			LastFinishedAt: finishedAt,
			LastDuration:   duration,
			LastError:      errorString(run.Error),
			ItemsProcessed: run.ItemsProcessed,
			RetryCount:     run.RetryCount,
			LockAcquired:   run.LockAcquired,
		})
	}
}

func (o *Observer) RecordOutboxRetry(ctx context.Context, eventID string, eventType string, retryCount int, err error) {
	if o == nil {
		return
	}

	fields := Fields{
		"event_id":    eventID,
		"event_type":  eventType,
		"retry_count": retryCount,
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	o.logger.Log(ctx, LevelWarn, "outbox_event_retry_scheduled", fields)
}

func (o *Observer) StartOutboxRun(ctx context.Context) (context.Context, func(OutboxRun)) {
	if o == nil {
		return ctx, func(OutboxRun) {}
	}

	startedAt := time.Now()
	ctx, span := o.tracer.Start(ctx, "job.process_outbox_events.consume_batch", nil)
	o.logger.Log(ctx, LevelInfo, "outbox_batch_started", nil)

	return ctx, func(run OutboxRun) {
		duration := run.Duration
		if duration <= 0 {
			duration = time.Since(startedAt)
		}
		finishedAt := startedAt.Add(duration)
		if run.Status == "" {
			switch {
			case run.Error != nil || run.Failed > 0:
				run.Status = StatusFailed
			default:
				run.Status = StatusSuccess
			}
		}

		fields := Fields{
			"status":      run.Status,
			"claimed":     run.Claimed,
			"published":   run.Published,
			"failed":      run.Failed,
			"retry_count": run.RetryCount,
		}
		if run.Error != nil {
			fields["error"] = run.Error.Error()
		}

		span.AddFields(fields)
		if run.Error != nil {
			span.RecordError(run.Error)
		}
		span.End()

		level := LevelInfo
		message := "outbox_batch_completed"
		if run.Error != nil || run.Failed > 0 {
			level = LevelError
			message = "outbox_batch_failed"
		}
		o.logger.Log(ctx, level, message, fields)
		o.health.RecordOutbox(OutboxHealth{
			Status:         run.Status,
			LastStartedAt:  startedAt,
			LastFinishedAt: finishedAt,
			LastDuration:   duration,
			LastError:      errorString(run.Error),
			Claimed:        run.Claimed,
			Published:      run.Published,
			Failed:         run.Failed,
			RetryCount:     run.RetryCount,
		})
	}
}

func statusFromError(err error) Status {
	if err != nil {
		return StatusFailed
	}
	return StatusSuccess
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func mergeFields(left Fields, right Fields) Fields {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}

	merged := make(Fields, len(left)+len(right))
	for key, value := range left {
		merged[key] = value
	}
	for key, value := range right {
		merged[key] = value
	}

	return merged
}

type noopMetrics struct{}

func (noopMetrics) AddCounter(string, int64, Fields) {}

func (noopMetrics) ObserveDuration(string, time.Duration, Fields) {}

func fallbackMetrics(metrics Metrics) Metrics {
	if metrics == nil {
		return noopMetrics{}
	}
	return metrics
}

type noopLogger struct{}

func (noopLogger) Log(context.Context, Level, string, Fields) {}

func fallbackLogger(logger Logger) Logger {
	if logger == nil {
		return noopLogger{}
	}
	return logger
}

type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _ string, _ Fields) (context.Context, Span) {
	return ctx, noopSpan{}
}

func fallbackTracer(tracer Tracer) Tracer {
	if tracer == nil {
		return noopTracer{}
	}
	return tracer
}

type noopSpan struct{}

func (noopSpan) AddFields(Fields) {}

func (noopSpan) RecordError(error) {}

func (noopSpan) End() {}

type noopHealth struct{}

func (noopHealth) RecordJob(JobHealth) {}

func (noopHealth) RecordOutbox(OutboxHealth) {}

func fallbackHealth(health HealthReporter) HealthReporter {
	if health == nil {
		return noopHealth{}
	}
	return health
}
