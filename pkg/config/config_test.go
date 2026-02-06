package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testConfig struct {
	CommonConfig `yaml:",inline"`
	Http         HTTPServerConfig `yaml:"http,inline"`
	Database     DatabaseConfig   `yaml:"database,inline"`
	Metrics      MetricsConfig    `yaml:"metrics,inline"`

	APIKey   string   `env:"API_KEY" yaml:"api_key" required:"true"`
	Debug    bool     `env:"DEBUG" yaml:"debug" default:"false"`
	Features []string `env:"FEATURES" yaml:"features"`
}

// Validate implements the Validator interface to validate embedded structs
func (c testConfig) Validate() error {
	if err := c.CommonConfig.Validate(); err != nil {
		return err
	}
	if err := c.Http.Validate(); err != nil {
		return err
	}
	if err := c.Metrics.Validate(); err != nil {
		return err
	}
	return nil
}

func TestGetConfigFromEnvVars(t *testing.T) {
	testCases := []struct {
		name    string
		envVars map[string]string
		want    testConfig
		wantErr bool
	}{
		{
			name: "All defaults, except required field",
			envVars: map[string]string{
				"API_KEY": "test-key",
			},
			want: testConfig{
				CommonConfig: CommonConfig{LogLevel: "info"},
				Http:         HTTPServerConfig{Port: 8080, ReadTimeoutSeconds: 15, WriteTimeoutSeconds: 15, IdleTimeoutSeconds: 60, MaxHeaderBytes: 1048576},
				Database: DatabaseConfig{
					URL:              "",
					Host:             "localhost",
					Port:             5432,
					Database:         "chatbot",
					Username:         "postgres",
					Password:         "postgres",
					SSLMode:          "disable",
					MaxConnections:   25,
					MinConnections:   5,
					MaxIdleTime:      "5m",
					MaxLifetime:      "30m",
					ConnectTimeout:   "10s",
					StatementTimeout: "30s",
				},
				Metrics: MetricsConfig{Port: 9090, ExposeMetrics: false, EnableHTTPMetrics: false, EnableJobMetrics: false},
				APIKey:  "test-key",
				Debug:   false,
			},
			wantErr: false,
		},
		{
			name: "Override with environment variables",
			envVars: map[string]string{
				"LOG_LEVEL":    "debug",
				"HTTP_PORT":    "3000",
				"DATABASE_URL": "postgres://test:test@testhost:5432/testdb",
				"API_KEY":      "env-key",
				"DEBUG":        "true",
				"FEATURES":     "feature1,feature2,feature3",
			},
			want: testConfig{
				CommonConfig: CommonConfig{LogLevel: "debug"},
				Http:         HTTPServerConfig{Port: 3000, ReadTimeoutSeconds: 15, WriteTimeoutSeconds: 15, IdleTimeoutSeconds: 60, MaxHeaderBytes: 1048576},
				Database: DatabaseConfig{
					URL:              "postgres://test:test@testhost:5432/testdb",
					Host:             "localhost",
					Port:             5432,
					Database:         "chatbot",
					Username:         "postgres",
					Password:         "postgres",
					SSLMode:          "disable",
					MaxConnections:   25,
					MinConnections:   5,
					MaxIdleTime:      "5m",
					MaxLifetime:      "30m",
					ConnectTimeout:   "10s",
					StatementTimeout: "30s",
				},
				Metrics:  MetricsConfig{Port: 9090, ExposeMetrics: false, EnableHTTPMetrics: false, EnableJobMetrics: false},
				APIKey:   "env-key",
				Debug:    true,
				Features: []string{"feature1", "feature2", "feature3"},
			},
			wantErr: false,
		},
		{
			name:    "Missing required field",
			envVars: map[string]string{},
			wantErr: true,
		},
		{
			name: "Invalid port number",
			envVars: map[string]string{
				"API_KEY":   "test-key",
				"HTTP_PORT": "99999",
			},
			wantErr: true, // Should fail validation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tc.envVars {
				_ = os.Setenv(k, v)
			}

			// Test the function
			var got testConfig
			err := GetConfigFromEnvVars(&got)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}

			// Cleanup
			os.Clearenv()
		})
	}
}

func TestHTTPServerConfigHelpers(t *testing.T) {
	cfg := HTTPServerConfig{
		ReadTimeoutSeconds:  30,
		WriteTimeoutSeconds: 60,
		IdleTimeoutSeconds:  120,
	}

	assert.Equal(t, "30s", cfg.ReadTimeout().String())
	assert.Equal(t, "1m0s", cfg.WriteTimeout().String())
	assert.Equal(t, "2m0s", cfg.IdleTimeout().String())
}

func TestGetConfigWithEnvInterpolation(t *testing.T) {
	// Create a temporary YAML file with environment variable placeholders
	yamlContent := `
log_level: info
http:
  port: 8080
api_key: ${TEST_API_KEY}
debug: ${TEST_DEBUG}
features:
  - ${TEST_FEATURE_1}
  - feature2
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	assert.NoError(t, err)
	tmpFile.Close()

	// Clear environment and set test values
	os.Clearenv()
	os.Setenv("TEST_API_KEY", "secret-from-env")
	os.Setenv("TEST_DEBUG", "true")
	os.Setenv("TEST_FEATURE_1", "dynamic-feature")

	// Load config
	var cfg testConfig
	err = GetConfig(&cfg, tmpFile.Name(), false)
	assert.NoError(t, err)

	// Verify environment variables were interpolated
	assert.Equal(t, "secret-from-env", cfg.APIKey)
	assert.Equal(t, true, cfg.Debug)
	assert.Equal(t, []string{"dynamic-feature", "feature2"}, cfg.Features)

	// Cleanup
	os.Clearenv()
}

func TestGetConfigWithEnvInterpolationUnsetVar(t *testing.T) {
	// Test that unset env vars become empty strings
	yamlContent := `
log_level: info
api_key: ${UNSET_VAR}
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	assert.NoError(t, err)
	tmpFile.Close()

	os.Clearenv()

	var cfg testConfig
	err = GetConfig(&cfg, tmpFile.Name(), false)
	// Should fail because api_key is required and will be empty
	assert.Error(t, err)

	os.Clearenv()
}

func TestCommonConfigValidation(t *testing.T) {
	testCases := []struct {
		name     string
		logLevel string
		wantErr  bool
	}{
		{"Valid debug", "debug", false},
		{"Valid info", "info", false},
		{"Valid warn", "warn", false},
		{"Valid error", "error", false},
		{"Case insensitive", "DEBUG", false},
		{"Invalid level", "invalid", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := CommonConfig{LogLevel: tc.logLevel}
			err := cfg.Validate()

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
