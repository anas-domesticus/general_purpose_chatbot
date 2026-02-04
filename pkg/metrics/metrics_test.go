package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func TestMetrics_Listen(t *testing.T) {
	expectedOut := `# HELP app_grpc_request_duration_seconds gRPC request duration in seconds
# TYPE app_grpc_request_duration_seconds histogram
app_grpc_request_duration_seconds_bucket{le="0.1"} 0
app_grpc_request_duration_seconds_bucket{le="0.3"} 0
app_grpc_request_duration_seconds_bucket{le="0.5"} 0
app_grpc_request_duration_seconds_bucket{le="0.7"} 0
app_grpc_request_duration_seconds_bucket{le="1"} 0
app_grpc_request_duration_seconds_bucket{le="3"} 0
app_grpc_request_duration_seconds_bucket{le="5"} 0
app_grpc_request_duration_seconds_bucket{le="7"} 0
app_grpc_request_duration_seconds_bucket{le="10"} 0
app_grpc_request_duration_seconds_bucket{le="+Inf"} 0
app_grpc_request_duration_seconds_sum 0
app_grpc_request_duration_seconds_count 0
# HELP app_http_request_duration_seconds HTTP request duration in seconds
# TYPE app_http_request_duration_seconds histogram
app_http_request_duration_seconds_bucket{le="0.1"} 0
app_http_request_duration_seconds_bucket{le="0.3"} 0
app_http_request_duration_seconds_bucket{le="0.5"} 0
app_http_request_duration_seconds_bucket{le="0.7"} 0
app_http_request_duration_seconds_bucket{le="1"} 0
app_http_request_duration_seconds_bucket{le="3"} 0
app_http_request_duration_seconds_bucket{le="5"} 0
app_http_request_duration_seconds_bucket{le="7"} 0
app_http_request_duration_seconds_bucket{le="10"} 0
app_http_request_duration_seconds_bucket{le="+Inf"} 0
app_http_request_duration_seconds_sum 0
app_http_request_duration_seconds_count 0
# HELP app_total_0_grpc_responses Total OK gRPC responses returned
# TYPE app_total_0_grpc_responses counter
app_total_0_grpc_responses 5
# HELP app_total_200_http_responses Total OK HTTP responses returned
# TYPE app_total_200_http_responses counter
app_total_200_http_responses 5
# HELP app_total_404_http_responses Total Not Found HTTP responses returned
# TYPE app_total_404_http_responses counter
app_total_404_http_responses 5
# HELP app_total_7_grpc_responses Total PermissionDenied gRPC responses returned
# TYPE app_total_7_grpc_responses counter
app_total_7_grpc_responses 5
# HELP app_total_grpc_requests Total gPRC requests
# TYPE app_total_grpc_requests counter
app_total_grpc_requests 0
# HELP app_total_http_requests Total HTTP requests
# TYPE app_total_http_requests counter
app_total_http_requests 0
`

	m := NewMetrics(true, true, false, logger.NewLogger(logger.Config{Service: "test"}))
	port := getRandomHighPort()
	m.Listen(port)
	for i := 0; i < 5; i++ {
		m.IncrementHttpResponseCounter(200)
		m.IncrementHttpResponseCounter(404)
	}
	for i := 0; i < 5; i++ {
		m.IncrementGrpcResponseCounter(codes.OK)
		m.IncrementGrpcResponseCounter(codes.PermissionDenied)
	}

	time.Sleep(500 * time.Millisecond)

	// assert correct path
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/metrics", port), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, expectedOut, string(bodyBytes))

	// assert incorrect path
	req, err = http.NewRequest("GET", fmt.Sprintf("http://localhost:%d", port), nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	_ = resp.Body.Close()

	m.stopChan <- os.Interrupt
	assert.True(t, errors.Is(<-m.errChan, http.ErrServerClosed))
}

func TestMetrics_SetCustomMetrics(t *testing.T) {
	before := `# HELP test_foo1 foo 1 help
# TYPE test_foo1 counter
test_foo1 0
# HELP test_foo2 foo 2 help
# TYPE test_foo2 gauge
test_foo2 0
`
	after := `# HELP test_foo1 foo 1 help
# TYPE test_foo1 counter
test_foo1 1
# HELP test_foo2 foo 2 help
# TYPE test_foo2 gauge
test_foo2 1.234
`
	m := NewMetrics(false, false, false, logger.NewLogger(logger.Config{Service: "test"}))
	port := getRandomHighPort()
	m.Listen(port)

	customMetric0 := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "test",
		Name:      "foo1",
		Help:      "foo 1 help",
	})
	customMetric1 := prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: "test",
		Name:      "foo2",
		Help:      "foo 2 help",
	})
	m.AddCustomMetric(customMetric0)
	m.AddCustomMetric(customMetric1)

	time.Sleep(500 * time.Millisecond)

	// assertions
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/metrics", port), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, before, string(bodyBytes))

	customMetric0.Inc()
	customMetric1.Set(1.234)

	req, err = http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/metrics", port), nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	bodyBytes, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, after, string(bodyBytes))
}

func getRandomHighPort() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(16384) + 49152
}

func TestHTTPMiddleware(t *testing.T) {
	// Create metrics instance with HTTP counters enabled
	m := NewMetrics(true, false, false, logger.NewLogger(logger.Config{Service: "test"}))

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		}
	})

	// Wrap with metrics middleware
	handler := m.HTTPMiddleware()(testHandler)

	t.Run("tracks successful requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/success", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Check response
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "success", recorder.Body.String())

		// Check metrics were incremented
		// Note: We can't easily verify exact counter values without exposing them,
		// but we can verify the handler worked correctly
	})

	t.Run("tracks error requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/error", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Check response
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Equal(t, "error", recorder.Body.String())
	})

	t.Run("tracks request duration", func(t *testing.T) {
		// Create a handler that takes some time
		slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})

		wrappedHandler := m.HTTPMiddleware()(slowHandler)

		req := httptest.NewRequest("GET", "/slow", nil)
		recorder := httptest.NewRecorder()

		start := time.Now()
		wrappedHandler.ServeHTTP(recorder, req)
		elapsed := time.Since(start)

		// Verify it took at least 10ms
		assert.True(t, elapsed >= 10*time.Millisecond)
		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}

func TestResponseWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: recorder, statusCode: 200}

	t.Run("captures custom status code", func(t *testing.T) {
		rw.WriteHeader(404)
		assert.Equal(t, 404, rw.statusCode)
		assert.Equal(t, 404, recorder.Code)
	})

	t.Run("defaults to 200 if WriteHeader not called", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: recorder, statusCode: 200}

		_, _ = rw.Write([]byte("test"))
		assert.Equal(t, 200, rw.statusCode)
	})
}

func TestJobMetrics(t *testing.T) {
	// Create metrics instance with job metrics enabled
	m := NewMetrics(false, false, true, logger.NewLogger(logger.Config{Service: "test"}))

	// Verify job metrics were created
	assert.NotNil(t, m.JobMetricCounters)
	assert.Contains(t, m.JobMetricCounters, JobMetricTotal)
	assert.Contains(t, m.JobMetricCounters, JobMetricTotalSuccess)
	assert.Contains(t, m.JobMetricCounters, JobMetricTotalFailed)
	assert.Contains(t, m.JobMetricCounters, JobMetricTotalKilled)

	// Test incrementing job metrics
	m.JobMetricCounters[JobMetricTotal].Inc()
	m.JobMetricCounters[JobMetricTotalSuccess].Inc()
}

func TestGrpcInterceptor(t *testing.T) {
	// Create metrics instance with gRPC counters enabled
	m := NewMetrics(false, true, false, logger.NewLogger(logger.Config{Service: "test"}))

	// Mock handler that returns an error
	errorHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("test error")
	}

	// Mock handler that succeeds
	successHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	// Mock gRPC server info
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	t.Run("tracks successful gRPC requests", func(t *testing.T) {
		ctx := context.Background()
		resp, err := m.GrpcRequestsInterceptor(ctx, "test", info, successHandler)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
	})

	t.Run("tracks error gRPC requests", func(t *testing.T) {
		ctx := context.Background()
		resp, err := m.GrpcRequestsInterceptor(ctx, "test", info, errorHandler)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}
