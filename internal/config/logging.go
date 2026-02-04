package config

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL" yaml:"level" default:"info"`
	Format string `env:"LOG_FORMAT" yaml:"format" default:"json"`
}
