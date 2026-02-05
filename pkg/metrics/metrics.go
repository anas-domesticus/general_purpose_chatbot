// Package metrics provides Prometheus metrics collection for HTTP and gRPC requests.
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	subsystem = "app"
)

// Metrics provides Prometheus metrics collection for HTTP and gRPC requests.
type Metrics struct {
	reg *prometheus.Registry

	TotalHTTPRequestsCounter prometheus.Counter
	HTTPRequestsCounters     map[int]prometheus.Counter
	HTTPDurationHistogram    prometheus.Histogram

	TotalGrpcRequestsCounter prometheus.Counter
	GrpcRequestsCounters     map[int]prometheus.Counter
	GrpcDurationHistogram    prometheus.Histogram

	JobMetricCounters map[int]prometheus.Counter

	customMetrics []prometheus.Collector

	stopChan chan os.Signal
	errChan  chan error
	log      logger.Logger
}

// NewMetrics creates a new Metrics instance with the specified collectors enabled.
func NewMetrics(httpCounters, grpcCounters, jobMetrics bool, l logger.Logger) Metrics {
	m := Metrics{
		reg: prometheus.NewRegistry(),
		log: l,
	}
	if httpCounters {
		m.TotalHTTPRequestsCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "total_http_requests",
			Help:      "Total HTTP requests",
		})
		m.reg.MustRegister(m.TotalHTTPRequestsCounter)
		m.HTTPRequestsCounters = make(map[int]prometheus.Counter)

		m.HTTPDurationHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.1, 0.3, 0.5, 0.7, 1.0, 3.0, 5.0, 7.0, 10.0},
		})
		m.reg.MustRegister(m.HTTPDurationHistogram)
	}
	if grpcCounters {
		m.TotalGrpcRequestsCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "total_grpc_requests",
			Help:      "Total gPRC requests",
		})
		m.reg.MustRegister(m.TotalGrpcRequestsCounter)
		m.GrpcRequestsCounters = make(map[int]prometheus.Counter)

		m.GrpcDurationHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "grpc_request_duration_seconds",
			Help:      "gRPC request duration in seconds",
			Buckets:   []float64{0.1, 0.3, 0.5, 0.7, 1.0, 3.0, 5.0, 7.0, 10.0},
		})
		m.reg.MustRegister(m.GrpcDurationHistogram)
	}
	if jobMetrics {
		m.JobMetricCounters = getJobMetricCounters()
		for k := range m.JobMetricCounters {
			m.reg.MustRegister(m.JobMetricCounters[k])
		}
	}
	return m
}

// Listen starts the metrics HTTP server on the specified port.
func (m *Metrics) Listen(port int) {
	m.log.Info("Starting metrics listener", logger.IntField("port", port))
	mux := http.NewServeMux()
	mux.Handle("/", http.NotFoundHandler())
	mux.Handle("/metrics", promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}))
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	sigChan := make(chan os.Signal)
	errChan := make(chan error)
	go func() {
		errChan <- server.ListenAndServe()
	}()
	go func() {
		for {
			sig := <-sigChan
			if sig == os.Interrupt {
				m.log.Info("Stopping metrics listener")
				_ = server.Shutdown(context.Background())
				return
			}
		}
	}()
	m.errChan = errChan
	m.stopChan = sigChan
}

// Job metric counter indices.
const (
	JobMetricTotal = iota
	JobMetricTotalSuccess
	JobMetricTotalFailed
	JobMetricTotalKilled
)

func getJobMetricCounters() map[int]prometheus.Counter {
	m := make(map[int]prometheus.Counter)
	m[JobMetricTotal] = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      "total_jobs_handled",
		Help:      "Total jobs handled",
	})
	m[JobMetricTotalSuccess] = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      "total_jobs_successful",
		Help:      "Total jobs handled successfully",
	})
	m[JobMetricTotalFailed] = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      "total_jobs_failed",
		Help:      "Total jobs handled unsuccessfully",
	})
	m[JobMetricTotalKilled] = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      "total_jobs_killed",
		Help:      "Total jobs killed",
	})
	return m
}

// AddCustomMetric registers a custom Prometheus collector.
func (m *Metrics) AddCustomMetric(c prometheus.Collector) {
	m.customMetrics = append(m.customMetrics, c)
	m.reg.MustRegister(m.customMetrics[len(m.customMetrics)-1])
}

// GrpcRequestsInterceptor implements gRPC unary interceptor interface
// Note: interface{} usage required by gRPC library signature
func (m *Metrics) GrpcRequestsInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	start := time.Now()

	m.TotalGrpcRequestsCounter.Inc()
	h, err := handler(ctx, req)

	// Record metrics
	duration := time.Since(start)
	m.GrpcDurationHistogram.Observe(duration.Seconds())
	m.IncrementGrpcResponseCounter(status.Code(err))

	return h, err
}

// IncrementHTTPResponseCounter increments the counter for the given HTTP status code.
func (m *Metrics) IncrementHTTPResponseCounter(code int) {
	_, ok := m.HTTPRequestsCounters[code]
	if !ok {
		m.HTTPRequestsCounters[code] = newTotalHTTPReqMetric(code)
		m.reg.MustRegister(m.HTTPRequestsCounters[code])
	}
	m.HTTPRequestsCounters[code].Inc()
}

// IncrementGrpcResponseCounter increments the counter for the given gRPC status code.
func (m *Metrics) IncrementGrpcResponseCounter(code codes.Code) {
	_, ok := m.GrpcRequestsCounters[int(code)]
	if !ok {
		m.GrpcRequestsCounters[int(code)] = newTotalGrpcReqMetric(code)
		m.reg.MustRegister(m.GrpcRequestsCounters[int(code)])
	}
	m.GrpcRequestsCounters[int(code)].Inc()
}

func newTotalHTTPReqMetric(code int) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      fmt.Sprintf("total_%d_http_responses", code),
		Help:      fmt.Sprintf("Total %s HTTP responses returned", http.StatusText(code)),
	})
}

func newTotalGrpcReqMetric(c codes.Code) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      fmt.Sprintf("total_%d_grpc_responses", c),
		Help:      fmt.Sprintf("Total %s gRPC responses returned", c.String()),
	})
}

// HTTPMiddleware returns a Chi-compatible middleware that tracks HTTP metrics
func (m *Metrics) HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Increment total requests counter
			m.TotalHTTPRequestsCounter.Inc()

			// Create a response writer that captures the status code
			rw := &responseWriter{ResponseWriter: w, statusCode: 200}

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			m.HTTPDurationHistogram.Observe(duration.Seconds())
			m.IncrementHTTPResponseCounter(rw.statusCode)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
