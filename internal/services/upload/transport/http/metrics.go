package httptransport

import (
	"errors"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
	"github.com/prometheus/client_golang/prometheus"
)

var uploadCompleteDurationBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10}

type MetricsRecorder interface {
	RecordSessionCreate()
	RecordSessionComplete(time.Duration)
	RecordSessionCompleteFailure(xerrors.Code)
	RecordSessionAbort()
	RecordDedupHit()
	RecordDedupMiss()
}

type prometheusMetricsRecorder struct {
	sessionCreate       prometheus.Counter
	sessionComplete     prometheus.Counter
	sessionCompleteFail *prometheus.CounterVec
	sessionAbort        prometheus.Counter
	completeDuration    prometheus.Histogram
	dedupHit            prometheus.Counter
	dedupMiss           prometheus.Counter
}

type noopMetricsRecorder struct{}

func NewMetricsRecorder(registry *prometheus.Registry, serviceName string) MetricsRecorder {
	if registry == nil {
		return noopMetricsRecorder{}
	}

	constLabels := prometheus.Labels{}
	if name := strings.TrimSpace(serviceName); name != "" {
		constLabels["service"] = name
	}

	return &prometheusMetricsRecorder{
		sessionCreate: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "upload_session_create_total",
			Help:        "Total number of successful upload session creations.",
			ConstLabels: constLabels,
		})),
		sessionComplete: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "upload_session_complete_total",
			Help:        "Total number of successful upload session completions.",
			ConstLabels: constLabels,
		})),
		sessionCompleteFail: registerCounterVec(registry, prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "upload_session_complete_failed_total",
				Help:        "Total number of failed upload session completions.",
				ConstLabels: constLabels,
			},
			[]string{"error_code"},
		)),
		sessionAbort: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "upload_session_abort_total",
			Help:        "Total number of successful upload session aborts.",
			ConstLabels: constLabels,
		})),
		completeDuration: registerHistogram(registry, prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:        "upload_complete_duration_seconds",
			Help:        "Duration of successful upload completion requests in seconds.",
			ConstLabels: constLabels,
			Buckets:     uploadCompleteDurationBuckets,
		})),
		dedupHit: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "upload_dedup_hit_total",
			Help:        "Total number of dedup hits during upload completion.",
			ConstLabels: constLabels,
		})),
		dedupMiss: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "upload_dedup_miss_total",
			Help:        "Total number of dedup misses during upload completion.",
			ConstLabels: constLabels,
		})),
	}
}

func (r *prometheusMetricsRecorder) RecordSessionCreate() {
	if r == nil {
		return
	}
	r.sessionCreate.Inc()
}

func (r *prometheusMetricsRecorder) RecordSessionComplete(duration time.Duration) {
	if r == nil {
		return
	}
	r.sessionComplete.Inc()
	r.completeDuration.Observe(duration.Seconds())
}

func (r *prometheusMetricsRecorder) RecordSessionCompleteFailure(code xerrors.Code) {
	if r == nil {
		return
	}
	r.sessionCompleteFail.WithLabelValues(metricErrorCode(code)).Inc()
}

func (r *prometheusMetricsRecorder) RecordSessionAbort() {
	if r == nil {
		return
	}
	r.sessionAbort.Inc()
}

func (r *prometheusMetricsRecorder) RecordDedupHit() {
	if r == nil {
		return
	}
	r.dedupHit.Inc()
}

func (r *prometheusMetricsRecorder) RecordDedupMiss() {
	if r == nil {
		return
	}
	r.dedupMiss.Inc()
}

func (noopMetricsRecorder) RecordSessionCreate()                      {}
func (noopMetricsRecorder) RecordSessionComplete(time.Duration)       {}
func (noopMetricsRecorder) RecordSessionCompleteFailure(xerrors.Code) {}
func (noopMetricsRecorder) RecordSessionAbort()                       {}
func (noopMetricsRecorder) RecordDedupHit()                           {}
func (noopMetricsRecorder) RecordDedupMiss()                          {}

func registerCounter(registry *prometheus.Registry, collector prometheus.Counter) prometheus.Counter {
	if err := registry.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(prometheus.Counter); ok {
				return existing
			}
		}
		panic(err)
	}
	return collector
}

func registerCounterVec(registry *prometheus.Registry, collector *prometheus.CounterVec) *prometheus.CounterVec {
	if err := registry.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
		panic(err)
	}
	return collector
}

func registerHistogram(registry *prometheus.Registry, collector prometheus.Histogram) prometheus.Histogram {
	if err := registry.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(prometheus.Histogram); ok {
				return existing
			}
		}
		panic(err)
	}
	return collector
}

func metricErrorCode(code xerrors.Code) string {
	if value := strings.TrimSpace(string(code)); value != "" {
		return value
	}
	return string(xerrors.CodeInternalError)
}
