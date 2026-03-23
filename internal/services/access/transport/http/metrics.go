package httptransport

import (
	"errors"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsRecorder interface {
	RecordFileGet()
	RecordAccessTicketIssue()
	RecordDownloadRedirect()
	RecordDownloadRedirectFailure(xerrors.Code)
	RecordAccessTicketVerifyFailure(xerrors.Code)
	RecordStoragePresignDuration(time.Duration)
}

type prometheusMetricsRecorder struct {
	fileGet             prometheus.Counter
	accessTicketIssue   prometheus.Counter
	downloadRedirect    prometheus.Counter
	downloadRedirectErr *prometheus.CounterVec
	ticketVerifyErr     *prometheus.CounterVec
	presignDuration     prometheus.Histogram
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
		fileGet: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "file_get_total",
			Help:        "Total number of successful file metadata fetches.",
			ConstLabels: constLabels,
		})),
		accessTicketIssue: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "access_ticket_issue_total",
			Help:        "Total number of successful access ticket issuances.",
			ConstLabels: constLabels,
		})),
		downloadRedirect: registerCounter(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "download_redirect_total",
			Help:        "Total number of successful download redirects.",
			ConstLabels: constLabels,
		})),
		downloadRedirectErr: registerCounterVec(registry, prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "download_redirect_failed_total",
				Help:        "Total number of failed download redirects.",
				ConstLabels: constLabels,
			},
			[]string{"error_code"},
		)),
		ticketVerifyErr: registerCounterVec(registry, prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "access_ticket_verify_failed_total",
				Help:        "Total number of access ticket verification failures.",
				ConstLabels: constLabels,
			},
			[]string{"error_code"},
		)),
		presignDuration: registerHistogram(registry, prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:        "access_storage_presign_duration_seconds",
			Help:        "Duration of private object storage presign operations in seconds.",
			ConstLabels: constLabels,
			Buckets:     prometheus.DefBuckets,
		})),
	}
}

func (r *prometheusMetricsRecorder) RecordFileGet() {
	if r == nil {
		return
	}
	r.fileGet.Inc()
}

func (r *prometheusMetricsRecorder) RecordAccessTicketIssue() {
	if r == nil {
		return
	}
	r.accessTicketIssue.Inc()
}

func (r *prometheusMetricsRecorder) RecordDownloadRedirect() {
	if r == nil {
		return
	}
	r.downloadRedirect.Inc()
}

func (r *prometheusMetricsRecorder) RecordDownloadRedirectFailure(code xerrors.Code) {
	if r == nil {
		return
	}
	r.downloadRedirectErr.WithLabelValues(metricErrorCode(code)).Inc()
}

func (r *prometheusMetricsRecorder) RecordAccessTicketVerifyFailure(code xerrors.Code) {
	if r == nil {
		return
	}
	r.ticketVerifyErr.WithLabelValues(metricErrorCode(code)).Inc()
}

func (r *prometheusMetricsRecorder) RecordStoragePresignDuration(duration time.Duration) {
	if r == nil {
		return
	}
	r.presignDuration.Observe(duration.Seconds())
}

func (noopMetricsRecorder) RecordFileGet()                               {}
func (noopMetricsRecorder) RecordAccessTicketIssue()                     {}
func (noopMetricsRecorder) RecordDownloadRedirect()                      {}
func (noopMetricsRecorder) RecordDownloadRedirectFailure(xerrors.Code)   {}
func (noopMetricsRecorder) RecordAccessTicketVerifyFailure(xerrors.Code) {}
func (noopMetricsRecorder) RecordStoragePresignDuration(time.Duration)   {}

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
