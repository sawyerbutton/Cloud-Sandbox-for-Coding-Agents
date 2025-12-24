package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the cloud sandbox system
type Metrics struct {
	// Sandbox metrics
	SandboxesTotal     prometheus.Gauge
	SandboxesActive    prometheus.Gauge
	SandboxesIdle      prometheus.Gauge
	SandboxAcquireTime prometheus.Histogram
	SandboxReleaseTime prometheus.Histogram

	// Execution metrics
	ExecutionsTotal   *prometheus.CounterVec
	ExecutionDuration prometheus.Histogram
	ExecutionErrors   *prometheus.CounterVec

	// Session metrics
	SessionsTotal   prometheus.Gauge
	SessionsActive  prometheus.Gauge
	SessionsPaused  prometheus.Gauge
	SessionCreated  prometheus.Counter
	SessionDeleted  prometheus.Counter
	SessionDuration prometheus.Histogram

	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPResponseSize    *prometheus.HistogramVec

	// File operation metrics
	FileOperationsTotal *prometheus.CounterVec
	FileOperationErrors *prometheus.CounterVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics(namespace string) *Metrics {
	m := &Metrics{
		// Sandbox metrics
		SandboxesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sandboxes_total",
			Help:      "Total number of sandboxes in the pool",
		}),
		SandboxesActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sandboxes_active",
			Help:      "Number of sandboxes currently in use",
		}),
		SandboxesIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sandboxes_idle",
			Help:      "Number of sandboxes available in the pool",
		}),
		SandboxAcquireTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "sandbox_acquire_duration_seconds",
			Help:      "Time to acquire a sandbox from the pool",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}),
		SandboxReleaseTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "sandbox_release_duration_seconds",
			Help:      "Time to release a sandbox back to the pool",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
		}),

		// Execution metrics
		ExecutionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "executions_total",
			Help:      "Total number of code executions",
		}, []string{"language", "status"}),
		ExecutionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "execution_duration_seconds",
			Help:      "Duration of code executions",
			Buckets:   []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		}),
		ExecutionErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "execution_errors_total",
			Help:      "Total number of execution errors",
		}, []string{"error_type"}),

		// Session metrics
		SessionsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sessions_total",
			Help:      "Total number of sessions",
		}),
		SessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sessions_active",
			Help:      "Number of active sessions",
		}),
		SessionsPaused: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sessions_paused",
			Help:      "Number of paused sessions",
		}),
		SessionCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sessions_created_total",
			Help:      "Total number of sessions created",
		}),
		SessionDeleted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sessions_deleted_total",
			Help:      "Total number of sessions deleted",
		}),
		SessionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "session_duration_seconds",
			Help:      "Duration of sessions from creation to deletion",
			Buckets:   []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800, 86400},
		}),

		// HTTP metrics
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),
		HTTPResponseSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "Size of HTTP responses",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
		}, []string{"method", "path"}),

		// File operation metrics
		FileOperationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "file_operations_total",
			Help:      "Total number of file operations",
		}, []string{"operation"}),
		FileOperationErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "file_operation_errors_total",
			Help:      "Total number of file operation errors",
		}, []string{"operation", "error_type"}),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.SandboxesTotal,
		m.SandboxesActive,
		m.SandboxesIdle,
		m.SandboxAcquireTime,
		m.SandboxReleaseTime,
		m.ExecutionsTotal,
		m.ExecutionDuration,
		m.ExecutionErrors,
		m.SessionsTotal,
		m.SessionsActive,
		m.SessionsPaused,
		m.SessionCreated,
		m.SessionDeleted,
		m.SessionDuration,
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPResponseSize,
		m.FileOperationsTotal,
		m.FileOperationErrors,
	)

	return m
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordExecution records an execution metric
func (m *Metrics) RecordExecution(language string, success bool, durationSeconds float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.ExecutionsTotal.WithLabelValues(language, status).Inc()
	m.ExecutionDuration.Observe(durationSeconds)
}

// RecordHTTPRequest records an HTTP request metric
func (m *Metrics) RecordHTTPRequest(method, path, status string, durationSeconds float64, responseSize int) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(durationSeconds)
	m.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
}

// RecordFileOperation records a file operation metric
func (m *Metrics) RecordFileOperation(operation string, success bool, errType string) {
	m.FileOperationsTotal.WithLabelValues(operation).Inc()
	if !success {
		m.FileOperationErrors.WithLabelValues(operation, errType).Inc()
	}
}

// UpdateSandboxStats updates sandbox pool metrics
func (m *Metrics) UpdateSandboxStats(total, active, idle int) {
	m.SandboxesTotal.Set(float64(total))
	m.SandboxesActive.Set(float64(active))
	m.SandboxesIdle.Set(float64(idle))
}

// UpdateSessionStats updates session metrics
func (m *Metrics) UpdateSessionStats(total, active, paused int) {
	m.SessionsTotal.Set(float64(total))
	m.SessionsActive.Set(float64(active))
	m.SessionsPaused.Set(float64(paused))
}
