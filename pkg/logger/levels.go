// Package logger provides structured logging utilities.
package logger

// Level string constants
const (
	levelDebug = "debug"
	levelInfo  = "info"
	levelWarn  = "warn"
	levelError = "error"
)

// Level represents log levels
type Level int

const (
	// DebugLevel represents the debug log level for detailed diagnostic information.
	DebugLevel Level = iota
	// InfoLevel represents the info log level for general informational messages.
	InfoLevel
	// WarnLevel represents the warn log level for warning messages.
	WarnLevel
	// ErrorLevel represents the error log level for error messages.
	ErrorLevel
)

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return levelDebug
	case InfoLevel:
		return levelInfo
	case WarnLevel:
		return levelWarn
	case ErrorLevel:
		return levelError
	default:
		return levelInfo
	}
}

// ParseLevel parses a string level into a Level enum
func ParseLevel(levelStr string) Level {
	switch levelStr {
	case levelDebug:
		return DebugLevel
	case levelInfo:
		return InfoLevel
	case levelWarn:
		return WarnLevel
	case levelError:
		return ErrorLevel
	default:
		return InfoLevel
	}
}
