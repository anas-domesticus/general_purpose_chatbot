package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewLogger(t *testing.T) {
	config := Config{
		Level:   DebugLevel,
		Format:  "json",
		Service: "test-service",
	}

	logger := NewLogger(config)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestLoggerWithFields(t *testing.T) {
	config := Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "test-service",
	}

	logger := NewLogger(config)

	// Create logger with fields
	loggerWithFields := logger.WithFields(
		StringField("key1", "value1"),
		StringField("key2", "value2"),
	)

	// Original logger should not be affected (immutable)
	if logger == loggerWithFields {
		t.Error("WithFields should return a new logger instance")
	}
}

func TestLoggerWithCorrelationID(t *testing.T) {
	config := Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "test-service",
	}

	logger := NewLogger(config)
	correlationID := "test-correlation-id"

	loggerWithCorrelation := logger.WithCorrelationID(correlationID)

	// Should return a new logger instance
	if logger == loggerWithCorrelation {
		t.Error("WithCorrelationID should return a new logger instance")
	}
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer

	// Create a logger with custom output
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	logger := &logger{
		logrus:  logrusLogger,
		fields:  []LogField{{Key: "service", Value: "test-service"}},
		service: "test-service",
	}

	// Test info logging
	logger.Info("test message", StringField("test_key", "test_value"))

	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Check expected fields
	if logEntry["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got %v", logEntry["msg"])
	}

	if logEntry["service"] != "test-service" {
		t.Errorf("Expected service='test-service', got %v", logEntry["service"])
	}

	if logEntry["test_key"] != "test_value" {
		t.Errorf("Expected test_key='test_value', got %v", logEntry["test_key"])
	}

	if logEntry["level"] != "info" {
		t.Errorf("Expected level='info', got %v", logEntry["level"])
	}
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer

	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})
	logrusLogger.SetLevel(logrus.DebugLevel)

	logger := &logger{
		logrus:  logrusLogger,
		fields:  []LogField{},
		service: "test-service",
	}

	// Test all log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	if !bytes.Contains(buf.Bytes(), []byte("debug message")) {
		t.Error("Debug message not found in output")
	}

	if !bytes.Contains(buf.Bytes(), []byte("info message")) {
		t.Error("Info message not found in output")
	}

	if !bytes.Contains(buf.Bytes(), []byte("warn message")) {
		t.Error("Warn message not found in output")
	}

	if !bytes.Contains(buf.Bytes(), []byte("error message")) {
		t.Error("Error message not found in output")
	}

	_ = output // Avoid unused variable warning
}

func TestFieldHelpers(t *testing.T) {
	tests := []struct {
		name     string
		field    LogField
		expected LogField
	}{
		{
			name:     "StringField",
			field:    StringField("test", "value"),
			expected: LogField{Key: "test", Value: "value"},
		},
		{
			name:     "IntField",
			field:    IntField("count", 42),
			expected: LogField{Key: "count", Value: "42"},
		},
		{
			name:     "DurationField",
			field:    DurationField("duration", 5*time.Second),
			expected: LogField{Key: "duration", Value: "5s"},
		},
		{
			name:     "CorrelationIDField",
			field:    CorrelationIDField("test-id"),
			expected: LogField{Key: "correlation_id", Value: "test-id"},
		},
		{
			name:     "HTTPMethodField",
			field:    HTTPMethodField("GET"),
			expected: LogField{Key: "http_method", Value: "GET"},
		},
		{
			name:     "HTTPPathField",
			field:    HTTPPathField("/api/test"),
			expected: LogField{Key: "http_path", Value: "/api/test"},
		},
		{
			name:     "HTTPStatusField",
			field:    HTTPStatusField(200),
			expected: LogField{Key: "http_status", Value: "200"},
		},
		{
			name:     "ClientIPField",
			field:    ClientIPField("192.168.1.1"),
			expected: LogField{Key: "client_ip", Value: "192.168.1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Key != tt.expected.Key {
				t.Errorf("Expected key=%s, got %s", tt.expected.Key, tt.field.Key)
			}
			if tt.field.Value != tt.expected.Value {
				t.Errorf("Expected value=%s, got %s", tt.expected.Value, tt.field.Value)
			}
		})
	}
}

func TestTimeField(t *testing.T) {
	testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	field := TimeField("timestamp", testTime)

	expected := LogField{Key: "timestamp", Value: "2023-01-01T12:00:00Z"}

	if field.Key != expected.Key {
		t.Errorf("Expected key=%s, got %s", expected.Key, field.Key)
	}
	if field.Value != expected.Value {
		t.Errorf("Expected value=%s, got %s", expected.Value, field.Value)
	}
}

func TestLoggerImmutability(t *testing.T) {
	config := Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "test-service",
	}

	logger1 := NewLogger(config)
	logger2 := logger1.WithFields(StringField("key1", "value1"))
	logger3 := logger2.WithFields(StringField("key2", "value2"))

	// Each logger should be independent
	if logger1 == logger2 || logger2 == logger3 || logger1 == logger3 {
		t.Error("Logger instances should be independent")
	}
}

func TestLoggerCustomOutput(t *testing.T) {
	var buf bytes.Buffer

	config := Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "test-service",
		Output:  &buf,
	}

	logger := NewLogger(config)
	logger.Info("test message", StringField("test_key", "test_value"))

	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Check expected fields
	if logEntry["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got %v", logEntry["msg"])
	}

	if logEntry["service"] != "test-service" {
		t.Errorf("Expected service='test-service', got %v", logEntry["service"])
	}

	if logEntry["test_key"] != "test_value" {
		t.Errorf("Expected test_key='test_value', got %v", logEntry["test_key"])
	}
}

func TestGrpcRequestsInterceptor(t *testing.T) {
	var buf bytes.Buffer

	config := Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "test-service",
		Output:  &buf,
	}

	logger := NewLogger(config)

	// Mock handler that returns successfully
	successHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	// Mock handler that returns an error
	errorHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.Internal, "test error")
	}

	// Mock gRPC server info
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	t.Run("successful request", func(t *testing.T) {
		buf.Reset()

		resp, err := logger.GrpcRequestsInterceptor(context.Background(), "test-req", info, successHandler)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resp != "success" {
			t.Errorf("Expected response='success', got %v", resp)
		}

		// Check that two log entries were created (start and completion)
		output := buf.String()
		logLines := strings.Split(strings.TrimSpace(output), "\n")

		if len(logLines) < 2 {
			t.Errorf("Expected at least 2 log entries, got %d", len(logLines))
		}

		// Parse the first log entry (start)
		var startEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLines[0]), &startEntry); err != nil {
			t.Fatalf("Failed to parse start log entry: %v", err)
		}

		if startEntry["msg"] != "gRPC request started" {
			t.Errorf("Expected start msg='gRPC request started', got %v", startEntry["msg"])
		}

		// Parse the second log entry (completion)
		var completionEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLines[1]), &completionEntry); err != nil {
			t.Fatalf("Failed to parse completion log entry: %v", err)
		}

		if completionEntry["msg"] != "gRPC request completed successfully" {
			t.Errorf("Expected completion msg='gRPC request completed successfully', got %v", completionEntry["msg"])
		}

		if completionEntry["grpc_method"] != "/test.Service/TestMethod" {
			t.Errorf("Expected grpc_method='/test.Service/TestMethod', got %v", completionEntry["grpc_method"])
		}

		if completionEntry["grpc_code"] != "OK" {
			t.Errorf("Expected grpc_code='OK', got %v", completionEntry["grpc_code"])
		}
	})

	t.Run("error request", func(t *testing.T) {
		buf.Reset()

		resp, err := logger.GrpcRequestsInterceptor(context.Background(), "test-req", info, errorHandler)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if resp != nil {
			t.Errorf("Expected nil response, got %v", resp)
		}

		// Check that error was logged
		output := buf.String()
		logLines := strings.Split(strings.TrimSpace(output), "\n")

		if len(logLines) < 2 {
			t.Errorf("Expected at least 2 log entries, got %d", len(logLines))
		}

		// Parse the completion log entry
		var completionEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLines[1]), &completionEntry); err != nil {
			t.Fatalf("Failed to parse completion log entry: %v", err)
		}

		if completionEntry["msg"] != "gRPC request completed with error" {
			t.Errorf("Expected completion msg='gRPC request completed with error', got %v", completionEntry["msg"])
		}

		if completionEntry["grpc_code"] != "Internal" {
			t.Errorf("Expected grpc_code='Internal', got %v", completionEntry["grpc_code"])
		}
	})
}

func TestGrpcFieldHelpers(t *testing.T) {
	tests := []struct {
		name     string
		field    LogField
		expected LogField
	}{
		{
			name:     "GrpcMethodField",
			field:    GrpcMethodField("/test.Service/TestMethod"),
			expected: LogField{Key: "grpc_method", Value: "/test.Service/TestMethod"},
		},
		{
			name:     "GrpcServiceField",
			field:    GrpcServiceField("test.Service"),
			expected: LogField{Key: "grpc_service", Value: "test.Service"},
		},
		{
			name:     "GrpcCodeField",
			field:    GrpcCodeField(codes.OK),
			expected: LogField{Key: "grpc_code", Value: "OK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Key != tt.expected.Key {
				t.Errorf("Expected key=%s, got %s", tt.expected.Key, tt.field.Key)
			}
			if tt.field.Value != tt.expected.Value {
				t.Errorf("Expected value=%s, got %s", tt.expected.Value, tt.field.Value)
			}
		})
	}
}

func TestHTTPMiddleware(t *testing.T) {
	var buf bytes.Buffer

	config := Config{
		Level:   InfoLevel,
		Format:  "json",
		Service: "test-service",
		Output:  &buf,
	}

	logger := NewLogger(config)

	t.Run("successful request", func(t *testing.T) {
		buf.Reset()

		// Create a test handler
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		})

		// Wrap with middleware
		middleware := logger.HTTPMiddleware(testHandler)

		// Create test request with valid UUID
		testCorrelationID := uuid.New().String()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("X-Correlation-ID", testCorrelationID)

		// Create response recorder
		rr := httptest.NewRecorder()

		// Execute request
		middleware.ServeHTTP(rr, req)

		// Check response
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		// Parse log entries
		output := buf.String()
		logLines := strings.Split(strings.TrimSpace(output), "\n")

		if len(logLines) < 2 {
			t.Errorf("Expected at least 2 log entries, got %d", len(logLines))
		}

		// Parse first log entry (request received)
		var requestEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLines[0]), &requestEntry); err != nil {
			t.Fatalf("Failed to parse request log entry: %v", err)
		}

		if requestEntry["msg"] != "HTTP request received" {
			t.Errorf("Expected msg='HTTP request received', got %v", requestEntry["msg"])
		}

		if requestEntry["http_method"] != "GET" {
			t.Errorf("Expected http_method='GET', got %v", requestEntry["http_method"])
		}

		if requestEntry["http_path"] != "/test" {
			t.Errorf("Expected http_path='/test', got %v", requestEntry["http_path"])
		}

		if requestEntry["correlation_id"] != testCorrelationID {
			t.Errorf("Expected correlation_id='%s', got %v", testCorrelationID, requestEntry["correlation_id"])
		}

		// Parse second log entry (response sent)
		var responseEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLines[1]), &responseEntry); err != nil {
			t.Fatalf("Failed to parse response log entry: %v", err)
		}

		if responseEntry["msg"] != "HTTP response sent" {
			t.Errorf("Expected msg='HTTP response sent', got %v", responseEntry["msg"])
		}

		if responseEntry["http_status"] != "200" {
			t.Errorf("Expected http_status='200', got %v", responseEntry["http_status"])
		}

		if responseEntry["response_bytes"] != "13" {
			t.Errorf("Expected response_bytes='13', got %v", responseEntry["response_bytes"])
		}

		if responseEntry["duration"] == nil {
			t.Error("Expected duration field to be present")
		}
	})

	t.Run("error status code", func(t *testing.T) {
		buf.Reset()

		// Create a test handler that returns error
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		})

		// Wrap with middleware
		middleware := logger.HTTPMiddleware(testHandler)

		// Create test request with valid UUID
		testCorrelationID := uuid.New().String()
		req := httptest.NewRequest("POST", "/api/error", nil)
		req.RemoteAddr = "192.168.1.1:5678"
		req.Header.Set("X-Correlation-ID", testCorrelationID)

		// Create response recorder
		rr := httptest.NewRecorder()

		// Execute request
		middleware.ServeHTTP(rr, req)

		// Check response
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}

		// Parse log entries
		output := buf.String()
		logLines := strings.Split(strings.TrimSpace(output), "\n")

		if len(logLines) < 2 {
			t.Errorf("Expected at least 2 log entries, got %d", len(logLines))
		}

		// Parse response log entry
		var responseEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLines[1]), &responseEntry); err != nil {
			t.Fatalf("Failed to parse response log entry: %v", err)
		}

		if responseEntry["http_status"] != "500" {
			t.Errorf("Expected http_status='500', got %v", responseEntry["http_status"])
		}

		if responseEntry["response_bytes"] != "5" {
			t.Errorf("Expected response_bytes='5', got %v", responseEntry["response_bytes"])
		}
	})
}

func TestEnsureHTTPCorrelationID(t *testing.T) {
	t.Run("generates correlation ID when missing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		updatedReq, correlationID := EnsureHTTPCorrelationID(req)

		// Should have generated a valid UUID
		if _, err := uuid.Parse(correlationID); err != nil {
			t.Errorf("Expected valid UUID, got %s: %v", correlationID, err)
		}

		// Should be in header
		if updatedReq.Header.Get("X-Correlation-ID") != correlationID {
			t.Errorf("Expected correlation ID in header to match generated ID")
		}

		// Should be in context
		if GetCorrelationIDFromContext(updatedReq.Context()) != correlationID {
			t.Errorf("Expected correlation ID in context to match generated ID")
		}
	})

	t.Run("preserves existing valid correlation ID", func(t *testing.T) {
		existingID := uuid.New().String()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", existingID)

		updatedReq, correlationID := EnsureHTTPCorrelationID(req)

		// Should preserve the existing ID
		if correlationID != existingID {
			t.Errorf("Expected correlation ID %s, got %s", existingID, correlationID)
		}

		// Should be in header
		if updatedReq.Header.Get("X-Correlation-ID") != existingID {
			t.Errorf("Expected correlation ID in header to match existing ID")
		}

		// Should be in context
		if GetCorrelationIDFromContext(updatedReq.Context()) != existingID {
			t.Errorf("Expected correlation ID in context to match existing ID")
		}
	})

	t.Run("replaces invalid correlation ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", "invalid-uuid")

		updatedReq, correlationID := EnsureHTTPCorrelationID(req)

		// Should have generated a new valid UUID
		if _, err := uuid.Parse(correlationID); err != nil {
			t.Errorf("Expected valid UUID, got %s: %v", correlationID, err)
		}

		// Should not be the invalid ID
		if correlationID == "invalid-uuid" {
			t.Error("Expected invalid correlation ID to be replaced")
		}

		// Should be in header
		if updatedReq.Header.Get("X-Correlation-ID") != correlationID {
			t.Errorf("Expected correlation ID in header to match new generated ID")
		}

		// Should be in context
		if GetCorrelationIDFromContext(updatedReq.Context()) != correlationID {
			t.Errorf("Expected correlation ID in context to match new generated ID")
		}
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := newResponseWriter(rr)

		rw.WriteHeader(http.StatusNotFound)

		if rw.statusCode != http.StatusNotFound {
			t.Errorf("Expected statusCode=%d, got %d", http.StatusNotFound, rw.statusCode)
		}
	})

	t.Run("captures bytes written", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := newResponseWriter(rr)

		data := []byte("test data")
		n, err := rw.Write(data)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if n != len(data) {
			t.Errorf("Expected n=%d, got %d", len(data), n)
		}

		if rw.bytesWritten != len(data) {
			t.Errorf("Expected bytesWritten=%d, got %d", len(data), rw.bytesWritten)
		}
	})

	t.Run("defaults to 200 OK", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := newResponseWriter(rr)

		if rw.statusCode != http.StatusOK {
			t.Errorf("Expected default statusCode=%d, got %d", http.StatusOK, rw.statusCode)
		}
	})
}