package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testConfig struct {
	CommonConfig `yaml:",inline"`

	APIKey   string   `env:"API_KEY" yaml:"api_key" required:"true"`
	Debug    bool     `env:"DEBUG" yaml:"debug" default:"false"`
	Features []string `env:"FEATURES" yaml:"features"`
}

// Validate implements the Validator interface.
func (c testConfig) Validate() error {
	return c.CommonConfig.Validate()
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
				APIKey:       "test-key",
				Debug:        false,
			},
			wantErr: false,
		},
		{
			name: "Override with environment variables",
			envVars: map[string]string{
				"LOG_LEVEL": "debug",
				"API_KEY":   "env-key",
				"DEBUG":     "true",
				"FEATURES":  "feature1,feature2,feature3",
			},
			want: testConfig{
				CommonConfig: CommonConfig{LogLevel: "debug"},
				APIKey:       "env-key",
				Debug:        true,
				Features:     []string{"feature1", "feature2", "feature3"},
			},
			wantErr: false,
		},
		{
			name:    "Missing required field",
			envVars: map[string]string{},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Clearenv()
			for k, v := range tc.envVars {
				_ = os.Setenv(k, v)
			}

			var got testConfig
			err := GetConfigFromEnvVars(&got)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}

			os.Clearenv()
		})
	}
}

func TestGetConfigWithEnvInterpolation(t *testing.T) {
	yamlContent := `
log_level: info
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

	os.Clearenv()
	os.Setenv("TEST_API_KEY", "secret-from-env")
	os.Setenv("TEST_DEBUG", "true")
	os.Setenv("TEST_FEATURE_1", "dynamic-feature")

	var cfg testConfig
	err = GetConfig(&cfg, tmpFile.Name(), false)
	assert.NoError(t, err)

	assert.Equal(t, "secret-from-env", cfg.APIKey)
	assert.Equal(t, true, cfg.Debug)
	assert.Equal(t, []string{"dynamic-feature", "feature2"}, cfg.Features)

	os.Clearenv()
}

func TestGetConfigWithEnvInterpolationUnsetVar(t *testing.T) {
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
