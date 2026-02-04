package logger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// CorrelationIDMetadataKey is the key used for correlation ID in gRPC metadata
	CorrelationIDMetadataKey = "x-correlation-id"
	// CorrelationIDFieldKey is the field key used for correlation ID in log entries
	CorrelationIDFieldKey = "correlation_id"
)

// Context key for correlation ID
type contextKey string

const correlationIDContextKey contextKey = "correlation_id"

// LogField represents a structured log field with concrete types
type LogField struct {
	Key   string
	Value string
}

// Logger interface with simplified, focused methods
type Logger interface {
	Info(msg string, fields ...LogField)
	Error(msg string, fields ...LogField)
	Debug(msg string, fields ...LogField)
	Warn(msg string, fields ...LogField)
	WithFields(fields ...LogField) Logger
	WithCorrelationID(id string) Logger
	GrpcRequestsInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
	HTTPMiddleware(next http.Handler) http.Handler
}

// Config represents logger configuration
type Config struct {
	Level   Level
	Format  string
	Service string
	Output  io.Writer // Optional: defaults to os.Stdout if nil
}

// logger implements the Logger interface
type logger struct {
	logrus  *logrus.Logger
	fields  []LogField
	service string
}

// NewLogger creates a new logger instance with the given configuration
func NewLogger(config Config) Logger {
	logrusLogger := logrus.New()

	// Set formatter
	if config.Format == "text" {
		logrusLogger.SetFormatter(&logrus.TextFormatter{})
	} else {
		logrusLogger.SetFormatter(&logrus.JSONFormatter{})
	}

	// Set output (default to stdout if not specified)
	if config.Output != nil {
		logrusLogger.SetOutput(config.Output)
	} else {
		logrusLogger.SetOutput(os.Stdout)
	}

	// Set level
	switch config.Level {
	case DebugLevel:
		logrusLogger.SetLevel(logrus.DebugLevel)
	case WarnLevel:
		logrusLogger.SetLevel(logrus.WarnLevel)
	case ErrorLevel:
		logrusLogger.SetLevel(logrus.ErrorLevel)
	default:
		logrusLogger.SetLevel(logrus.InfoLevel)
	}

	// Add service field if provided
	var serviceFields []LogField
	if config.Service != "" {
		serviceFields = []LogField{{Key: "service", Value: config.Service}}
	}

	return &logger{
		logrus:  logrusLogger,
		fields:  serviceFields,
		service: config.Service,
	}
}

// WithFields returns a new logger with additional fields (immutable)
func (l *logger) WithFields(fields ...LogField) Logger {
	newFields := make([]LogField, 0, len(l.fields)+len(fields))
	newFields = append(newFields, l.fields...)
	newFields = append(newFields, fields...)

	return &logger{
		logrus:  l.logrus,
		fields:  newFields,
		service: l.service,
	}
}

// WithCorrelationID returns a new logger with correlation ID field
func (l *logger) WithCorrelationID(id string) Logger {
	return l.WithFields(LogField{Key: CorrelationIDFieldKey, Value: id})
}

// Info logs an info message with optional fields
func (l *logger) Info(msg string, fields ...LogField) {
	l.log(logrus.InfoLevel, msg, fields...)
}

// Error logs an error message with optional fields
func (l *logger) Error(msg string, fields ...LogField) {
	l.log(logrus.ErrorLevel, msg, fields...)
}

// Debug logs a debug message with optional fields
func (l *logger) Debug(msg string, fields ...LogField) {
	l.log(logrus.DebugLevel, msg, fields...)
}

// Warn logs a warning message with optional fields
func (l *logger) Warn(msg string, fields ...LogField) {
	l.log(logrus.WarnLevel, msg, fields...)
}

// log is the internal logging method
func (l *logger) log(level logrus.Level, msg string, fields ...LogField) {
	// Combine existing fields with new fields
	allFields := make([]LogField, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	// Convert to logrus.Fields
	logrusFields := l.convertToLogrusFields(allFields)

	// Log with appropriate level
	entry := l.logrus.WithFields(logrusFields)
	switch level {
	case logrus.InfoLevel:
		entry.Info(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	}
}

// convertToLogrusFields converts LogField slice to logrus.Fields
func (l *logger) convertToLogrusFields(fields []LogField) logrus.Fields {
	logrusFields := make(logrus.Fields, len(fields))
	for _, field := range fields {
		logrusFields[field.Key] = field.Value
	}
	return logrusFields
}

// Helper functions for common field types

// StringField returns a LogField for a string value.
func StringField(key, value string) LogField {
	return LogField{Key: key, Value: value}
}

// IntField returns a LogField for an integer value.
func IntField(key string, value int) LogField {
	return LogField{Key: key, Value: strconv.Itoa(value)}
}

// Int64Field returns a LogField for an int64 value.
func Int64Field(key string, value int64) LogField {
	return LogField{Key: key, Value: strconv.FormatInt(value, 10)}
}

// BoolField returns a LogField for a boolean value.
func BoolField(key string, value bool) LogField {
	return LogField{Key: key, Value: strconv.FormatBool(value)}
}

// Field creates a log field with automatic type conversion for less common types
func Field[T any](key string, value T) LogField {
	return LogField{Key: key, Value: convertValue(value)}
}

// convertValue converts various types to string representation
func convertValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case time.Duration:
		return v.String()
	case error:
		if v == nil {
			return "<nil>"
		}
		return v.Error()
	default:
		// Use fmt.Sprintf for any other type
		return fmt.Sprintf("%v", v)
	}
}

// DurationField returns a LogField for a time.Duration value.
func DurationField(key string, value time.Duration) LogField {
	return LogField{Key: key, Value: value.String()}
}

// TimeField returns a LogField for a time.Time value formatted as RFC3339.
func TimeField(key string, value time.Time) LogField {
	return LogField{Key: key, Value: value.Format(time.RFC3339)}
}

// ErrorField returns a LogField for an error value.
func ErrorField(err error) LogField {
	if err == nil {
		return LogField{Key: "error", Value: "<nil>"}
	}
	return LogField{Key: "error", Value: err.Error()}
}

// Common field constructors

// CorrelationIDField returns a LogField for a correlation ID.
func CorrelationIDField(id string) LogField {
	return StringField(CorrelationIDFieldKey, id)
}

// HTTPMethodField returns a LogField for an HTTP method.
func HTTPMethodField(method string) LogField {
	return StringField("http_method", method)
}

// HTTPPathField returns a LogField for an HTTP path.
func HTTPPathField(path string) LogField {
	return StringField("http_path", path)
}

// HTTPStatusField returns a LogField for an HTTP status code.
func HTTPStatusField(status int) LogField {
	return IntField("http_status", status)
}

// ClientIPField returns a LogField for a client IP address.
func ClientIPField(ip string) LogField {
	return StringField("client_ip", ip)
}

// GrpcRequestsInterceptor implements gRPC unary interceptor interface for logging
// Note: interface{} usage required by gRPC library signature
func (l *logger) GrpcRequestsInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	start := time.Now()

	// Ensure correlation ID is in context
	ctx, correlationID := EnsureCorrelationID(ctx)

	// Create logger with gRPC context and correlation ID
	grpcLogger := l.WithFields(
		StringField("grpc_method", info.FullMethod),
		CorrelationIDField(correlationID),
	)

	grpcLogger.Info("gRPC request started")

	// Call the handler with enriched context
	resp, err = handler(ctx, req)

	// Calculate duration
	duration := time.Since(start)

	// Log the result
	fields := []LogField{
		DurationField("duration", duration),
		StringField("grpc_code", status.Code(err).String()),
		IntField("grpc_code_int", int(status.Code(err))),
	}

	if err != nil {
		grpcLogger.Error("gRPC request completed with error", fields...)
	} else {
		grpcLogger.Info("gRPC request completed successfully", fields...)
	}

	return resp, err
}

// Helper functions for gRPC fields

// GrpcMethodField returns a LogField for a gRPC method name.
func GrpcMethodField(method string) LogField {
	return StringField("grpc_method", method)
}

// GrpcServiceField returns a LogField for a gRPC service name.
func GrpcServiceField(service string) LogField {
	return StringField("grpc_service", service)
}

// GrpcCodeField returns a LogField for a gRPC status code.
func GrpcCodeField(code codes.Code) LogField {
	return StringField("grpc_code", code.String())
}

// Correlation ID context helpers

// WithCorrelationIDContext adds correlation ID to context
func WithCorrelationIDContext(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDContextKey, correlationID)
}

// GetCorrelationIDFromContext retrieves correlation ID from context
func GetCorrelationIDFromContext(ctx context.Context) string {
	if correlationID, ok := ctx.Value(correlationIDContextKey).(string); ok {
		return correlationID
	}
	return ""
}

// ExtractCorrelationIDFromMetadata extracts correlation ID from gRPC incoming metadata
func ExtractCorrelationIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get(CorrelationIDMetadataKey)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// InjectCorrelationIDIntoMetadata injects correlation ID into outgoing gRPC metadata
func InjectCorrelationIDIntoMetadata(ctx context.Context, correlationID string) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(map[string]string{})
	} else {
		md = md.Copy()
	}

	md.Set(CorrelationIDMetadataKey, correlationID)
	return metadata.NewOutgoingContext(ctx, md)
}

// EnsureCorrelationID ensures context has a correlation ID, generating one if needed
func EnsureCorrelationID(ctx context.Context) (context.Context, string) {
	// First check if context already has one
	if correlationID := GetCorrelationIDFromContext(ctx); correlationID != "" {
		return ctx, correlationID
	}

	// Try to extract from gRPC metadata
	if correlationID := ExtractCorrelationIDFromMetadata(ctx); correlationID != "" {
		// Validate it's a proper UUID
		if _, err := uuid.Parse(correlationID); err == nil {
			ctx = WithCorrelationIDContext(ctx, correlationID)
			return ctx, correlationID
		}
	}

	// Generate new correlation ID
	correlationID := uuid.New().String()
	ctx = WithCorrelationIDContext(ctx, correlationID)
	return ctx, correlationID
}

// EnsureHTTPCorrelationID ensures HTTP request has a correlation ID, generating one if needed
func EnsureHTTPCorrelationID(r *http.Request) (*http.Request, string) {
	correlationID := r.Header.Get("X-Correlation-ID")
	if correlationID == "" {
		// Generate new correlation ID
		correlationID = uuid.New().String()
		r.Header.Set("X-Correlation-ID", correlationID)
	} else {
		// Validate existing correlation ID is a proper UUID
		if _, err := uuid.Parse(correlationID); err != nil {
			// Invalid UUID, generate a new one
			correlationID = uuid.New().String()
			r.Header.Set("X-Correlation-ID", correlationID)
		}
	}

	// Add to context
	ctx := WithCorrelationIDContext(r.Context(), correlationID)
	return r.WithContext(ctx), correlationID
}

// GetLoggerFromContext returns a logger with correlation ID from context automatically injected
func GetLoggerFromContext(ctx context.Context, baseLogger Logger) Logger {
	correlationID := GetCorrelationIDFromContext(ctx)
	if correlationID != "" {
		return baseLogger.WithCorrelationID(correlationID)
	}
	return baseLogger
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// HTTPMiddleware implements chi-compatible HTTP middleware for request logging
func (l *logger) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Ensure correlation ID is present in header and context
		r, correlationID := EnsureHTTPCorrelationID(r)

		// Create logger with request fields
		requestLogger := l.WithFields(
			ClientIPField(r.RemoteAddr),
			HTTPMethodField(r.Method),
			HTTPPathField(r.URL.Path),
			CorrelationIDField(correlationID),
		)

		requestLogger.Info("HTTP request received")

		// Wrap response writer to capture status and bytes
		wrappedWriter := newResponseWriter(w)

		// Process request
		next.ServeHTTP(wrappedWriter, r)

		// Calculate duration
		duration := time.Since(start)

		// Log response
		requestLogger.WithFields(
			HTTPStatusField(wrappedWriter.statusCode),
			IntField("response_bytes", wrappedWriter.bytesWritten),
			DurationField("duration", duration),
		).Info("HTTP response sent")
	})
}
