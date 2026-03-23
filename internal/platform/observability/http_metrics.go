package observability

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var httpResponseSizeBuckets = []float64{
	128,
	512,
	1024,
	4 * 1024,
	16 * 1024,
	64 * 1024,
	256 * 1024,
	1024 * 1024,
	4 * 1024 * 1024,
	16 * 1024 * 1024,
}

type httpMetricsRecorder struct {
	requests     *prometheus.CounterVec
	duration     *prometheus.HistogramVec
	responseSize *prometheus.HistogramVec
}

func WrapHTTPMetrics(serviceName string, metrics *Metrics, next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}

	recorder := newHTTPMetricsRecorder(serviceName, metrics)
	if recorder == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		writer := newMetricsResponseWriter(w)

		next.ServeHTTP(writer, r)

		labels := prometheus.Labels{
			"method":      strings.ToUpper(strings.TrimSpace(r.Method)),
			"route":       normalizeHTTPRoute(r),
			"status_code": strconv.Itoa(writer.StatusCode()),
		}
		recorder.requests.With(labels).Inc()
		recorder.duration.With(labels).Observe(time.Since(startedAt).Seconds())
		recorder.responseSize.With(labels).Observe(float64(writer.BytesWritten()))
	})
}

func newHTTPMetricsRecorder(serviceName string, metrics *Metrics) *httpMetricsRecorder {
	if metrics == nil || !metrics.Enabled() || metrics.Registry() == nil {
		return nil
	}

	constLabels := prometheus.Labels{}
	if name := strings.TrimSpace(serviceName); name != "" {
		constLabels["service"] = name
	}

	return &httpMetricsRecorder{
		requests: registerCounterVec(metrics.Registry(), prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "http_requests_total",
				Help:        "Total number of HTTP requests processed.",
				ConstLabels: constLabels,
			},
			[]string{"method", "route", "status_code"},
		)),
		duration: registerHistogramVec(metrics.Registry(), prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "http_request_duration_seconds",
				Help:        "HTTP request duration in seconds.",
				ConstLabels: constLabels,
				Buckets:     prometheus.DefBuckets,
			},
			[]string{"method", "route", "status_code"},
		)),
		responseSize: registerHistogramVec(metrics.Registry(), prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "http_response_size_bytes",
				Help:        "HTTP response size in bytes.",
				ConstLabels: constLabels,
				Buckets:     httpResponseSizeBuckets,
			},
			[]string{"method", "route", "status_code"},
		)),
	}
}

func normalizeHTTPRoute(r *http.Request) string {
	if r == nil {
		return "unmatched"
	}

	pattern := strings.TrimSpace(r.Pattern)
	if pattern == "" {
		return "unmatched"
	}

	if method, route, ok := strings.Cut(pattern, " "); ok {
		if strings.HasPrefix(route, "/") {
			return route
		}
		if strings.HasPrefix(method, "/") {
			return method
		}
	}
	if strings.HasPrefix(pattern, "/") {
		return pattern
	}
	return "unmatched"
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

func registerHistogramVec(registry *prometheus.Registry, collector *prometheus.HistogramVec) *prometheus.HistogramVec {
	if err := registry.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			if existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.HistogramVec); ok {
				return existing
			}
		}
		panic(err)
	}
	return collector
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (w *metricsResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *metricsResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *metricsResponseWriter) Write(body []byte) (int, error) {
	written, err := w.ResponseWriter.Write(body)
	w.bytesWritten += written
	return written, err
}

func (w *metricsResponseWriter) ReadFrom(src io.Reader) (int64, error) {
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		written, err := readerFrom.ReadFrom(src)
		w.bytesWritten += int(written)
		return written, err
	}

	written, err := io.Copy(w.ResponseWriter, src)
	w.bytesWritten += int(written)
	return written, err
}

func (w *metricsResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *metricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *metricsResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *metricsResponseWriter) StatusCode() int {
	if w == nil {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *metricsResponseWriter) BytesWritten() int {
	if w == nil {
		return 0
	}
	return w.bytesWritten
}
