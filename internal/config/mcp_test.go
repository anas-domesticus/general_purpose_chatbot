package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMCPConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      MCPConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with stdio transport",
			config: MCPConfig{
				Enabled: true,
				Timeout: 30 * time.Second,
				Servers: map[string]MCPServerConfig{
					"filesystem": {
						Name:      "filesystem",
						Transport: "stdio",
						Command:   "npx",
						Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
						Enabled:   true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid transport type",
			config: MCPConfig{
				Enabled: true,
				Timeout: 30 * time.Second,
				Servers: map[string]MCPServerConfig{
					"invalid": {
						Name:      "invalid",
						Transport: "invalid_transport",
						Enabled:   true,
					},
				},
			},
			expectError: true,
			errorMsg:    "transport must be one of [stdio, websocket, sse, http]",
		},
		{
			name: "stdio missing command",
			config: MCPConfig{
				Enabled: true,
				Timeout: 30 * time.Second,
				Servers: map[string]MCPServerConfig{
					"filesystem": {
						Name:      "filesystem",
						Transport: "stdio",
						Command:   "", // Missing command
						Enabled:   true,
					},
				},
			},
			expectError: true,
			errorMsg:    "command is required for stdio transport",
		},
		{
			name: "websocket missing URL",
			config: MCPConfig{
				Enabled: true,
				Timeout: 30 * time.Second,
				Servers: map[string]MCPServerConfig{
					"websocket": {
						Name:      "websocket",
						Transport: "websocket",
						URL:       "", // Missing URL
						Enabled:   true,
					},
				},
			},
			expectError: true,
			errorMsg:    "url is required for websocket transport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appConfig := &AppConfig{
				MCP:            tt.config,
				RequestTimeout: 30 * time.Second,
				LLM:            LLMConfig{Provider: "claude"},
				Anthropic: AnthropicConfig{
					APIKey:         "test-api-key",
					Timeout:        30 * time.Second,
					InitialBackoff: 1 * time.Second,
					MaxBackoff:     10 * time.Second,
				},
				Security:   SecurityConfig{MaxRequestSize: 1024, RateLimitRPS: 1},
				Logging:    LoggingConfig{Level: "info", Format: "json"},
				Monitoring: MonitoringConfig{},
			}

			err := appConfig.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMCPConfigYAMLParsing(t *testing.T) {
	yamlConfig := `
mcp:
  enabled: true
  timeout: "30s"
  servers:
    filesystem:
      name: "filesystem"
      enabled: true
      transport: "stdio"
      command: "npx"
      args:
        - "-y"
        - "@modelcontextprotocol/server-filesystem"
        - "/tmp"
      tool_filter:
        - "list_directory"
        - "read_file"
      environment:
        PATH: "/usr/bin"
    database:
      name: "database"
      enabled: false
      transport: "websocket"
      url: "ws://localhost:8080/mcp"
      auth:
        type: "bearer"
        token: "test-token"
`

	var config struct {
		MCP MCPConfig `yaml:"mcp"`
	}

	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	require.NoError(t, err)

	mcp := config.MCP
	assert.True(t, mcp.Enabled)
	assert.Equal(t, 30*time.Second, mcp.Timeout)
	assert.Len(t, mcp.Servers, 2)

	// Test filesystem server
	fs := mcp.Servers["filesystem"]
	assert.Equal(t, "filesystem", fs.Name)
	assert.True(t, fs.Enabled)
	assert.Equal(t, "stdio", fs.Transport)
	assert.Equal(t, "npx", fs.Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}, fs.Args)
	assert.Equal(t, []string{"list_directory", "read_file"}, fs.ToolFilter)
	assert.Equal(t, map[string]string{"PATH": "/usr/bin"}, fs.Environment)

	// Test database server
	db := mcp.Servers["database"]
	assert.Equal(t, "database", db.Name)
	assert.False(t, db.Enabled)
	assert.Equal(t, "websocket", db.Transport)
	assert.Equal(t, "ws://localhost:8080/mcp", db.URL)
	assert.NotNil(t, db.Auth)
	assert.Equal(t, "bearer", db.Auth.Type)
	assert.Equal(t, "test-token", db.Auth.Token)
}

func TestMCPAuthValidation(t *testing.T) {
	tests := []struct {
		name        string
		authConfig  *MCPAuthConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid bearer auth",
			authConfig: &MCPAuthConfig{
				Type:  "bearer",
				Token: "test-token",
			},
			expectError: false,
		},
		{
			name: "bearer auth missing token",
			authConfig: &MCPAuthConfig{
				Type:  "bearer",
				Token: "",
			},
			expectError: true,
			errorMsg:    "token is required for bearer auth",
		},
		{
			name: "valid basic auth",
			authConfig: &MCPAuthConfig{
				Type: "basic",
				User: "user",
				Pass: "pass",
			},
			expectError: false,
		},
		{
			name: "basic auth missing user",
			authConfig: &MCPAuthConfig{
				Type: "basic",
				User: "",
				Pass: "pass",
			},
			expectError: true,
			errorMsg:    "user and pass are required for basic auth",
		},
		{
			name: "valid api_key auth",
			authConfig: &MCPAuthConfig{
				Type:   "api_key",
				APIKey: "key",
				Header: "X-API-Key",
			},
			expectError: false,
		},
		{
			name: "api_key auth missing key",
			authConfig: &MCPAuthConfig{
				Type:   "api_key",
				APIKey: "",
				Header: "X-API-Key",
			},
			expectError: true,
			errorMsg:    "api_key and header are required for api_key auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appConfig := &AppConfig{
				MCP: MCPConfig{
					Enabled: true,
					Timeout: 30 * time.Second,
					Servers: map[string]MCPServerConfig{
						"test": {
							Name:      "test",
							Transport: "websocket",
							URL:       "ws://localhost:8080/mcp",
							Auth:      tt.authConfig,
							Enabled:   true,
						},
					},
				},
				RequestTimeout: 30 * time.Second,
				LLM:            LLMConfig{Provider: "claude"},
				Anthropic: AnthropicConfig{
					APIKey:         "test-api-key",
					Timeout:        30 * time.Second,
					InitialBackoff: 1 * time.Second,
					MaxBackoff:     10 * time.Second,
				},
				Security:   SecurityConfig{MaxRequestSize: 1024, RateLimitRPS: 1},
				Logging:    LoggingConfig{Level: "info", Format: "json"},
				Monitoring: MonitoringConfig{},
			}

			err := appConfig.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
