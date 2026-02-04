package logger

// Level represents log levels
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	default:
		return "info"
	}
}

// ParseLevel parses a string level into a Level enum
func ParseLevel(levelStr string) Level {
	switch levelStr {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}
