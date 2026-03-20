package acpclient

import (
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertMCPServers(t *testing.T) {
	tests := []struct {
		name   string
		input  []config.ACPMCPServer
		verify func(t *testing.T, result []acp.McpServer)
	}{
		{
			name:  "empty input returns empty output",
			input: nil,
			verify: func(t *testing.T, result []acp.McpServer) {
				assert.Empty(t, result)
			},
		},
		{
			name: "single stdio server",
			input: []config.ACPMCPServer{
				{Name: "my-tool", Transport: "stdio", Command: "npx", Args: []string{"-y", "server"}},
			},
			verify: func(t *testing.T, result []acp.McpServer) {
				require.Len(t, result, 1)
				require.NotNil(t, result[0].Stdio)
				assert.Equal(t, "my-tool", result[0].Stdio.Name)
				assert.Equal(t, "npx", result[0].Stdio.Command)
				assert.Equal(t, []string{"-y", "server"}, result[0].Stdio.Args)
				assert.Nil(t, result[0].Http)
				assert.Nil(t, result[0].Sse)
			},
		},
		{
			name: "single HTTP server with headers",
			input: []config.ACPMCPServer{
				{Name: "api", Transport: "http", URL: "https://example.com", Headers: map[string]string{"Authorization": "Bearer tok"}},
			},
			verify: func(t *testing.T, result []acp.McpServer) {
				require.Len(t, result, 1)
				require.NotNil(t, result[0].Http)
				assert.Equal(t, "api", result[0].Http.Name)
				assert.Equal(t, "http", result[0].Http.Type)
				assert.Equal(t, "https://example.com", result[0].Http.Url)
				require.Len(t, result[0].Http.Headers, 1)
				assert.Equal(t, "Authorization", result[0].Http.Headers[0].Name)
				assert.Equal(t, "Bearer tok", result[0].Http.Headers[0].Value)
			},
		},
		{
			name: "single SSE server",
			input: []config.ACPMCPServer{
				{Name: "events", Transport: "sse", URL: "https://sse.example.com"},
			},
			verify: func(t *testing.T, result []acp.McpServer) {
				require.Len(t, result, 1)
				require.NotNil(t, result[0].Sse)
				assert.Equal(t, "events", result[0].Sse.Name)
				assert.Equal(t, "sse", result[0].Sse.Type)
				assert.Equal(t, "https://sse.example.com", result[0].Sse.Url)
			},
		},
		{
			name: "unspecified transport defaults to stdio",
			input: []config.ACPMCPServer{
				{Name: "default", Command: "my-cmd"},
			},
			verify: func(t *testing.T, result []acp.McpServer) {
				require.Len(t, result, 1)
				require.NotNil(t, result[0].Stdio)
				assert.Equal(t, "default", result[0].Stdio.Name)
				assert.Equal(t, "my-cmd", result[0].Stdio.Command)
			},
		},
		{
			name: "env vars on stdio converted to EnvVariable slice",
			input: []config.ACPMCPServer{
				{Name: "tool", Transport: "stdio", Command: "cmd", Env: map[string]string{"KEY": "val"}},
			},
			verify: func(t *testing.T, result []acp.McpServer) {
				require.Len(t, result, 1)
				require.NotNil(t, result[0].Stdio)
				require.Len(t, result[0].Stdio.Env, 1)
				assert.Equal(t, "KEY", result[0].Stdio.Env[0].Name)
				assert.Equal(t, "val", result[0].Stdio.Env[0].Value)
			},
		},
		{
			name: "headers on HTTP converted to HttpHeader slice",
			input: []config.ACPMCPServer{
				{Name: "api", Transport: "http", URL: "https://x.com", Headers: map[string]string{"X-A": "1", "X-B": "2"}},
			},
			verify: func(t *testing.T, result []acp.McpServer) {
				require.Len(t, result, 1)
				require.NotNil(t, result[0].Http)
				assert.Len(t, result[0].Http.Headers, 2)
				assert.ElementsMatch(t,
					[]acp.HttpHeader{{Name: "X-A", Value: "1"}, {Name: "X-B", Value: "2"}},
					result[0].Http.Headers,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertMCPServers(tt.input)
			tt.verify(t, result)
		})
	}
}
