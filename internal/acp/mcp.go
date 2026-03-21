package acpclient

import (
	acp "github.com/coder/acp-go-sdk"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
)

// ConvertMCPServers converts config MCP server definitions to ACP SDK types.
// Always returns a non-nil slice (empty slice when no servers configured).
func ConvertMCPServers(servers []config.ACPMCPServer) []acp.McpServer {
	if len(servers) == 0 {
		return []acp.McpServer{}
	}
	result := make([]acp.McpServer, 0, len(servers))
	for _, s := range servers {
		var ms acp.McpServer
		switch s.Transport {
		case "http":
			ms.Http = &acp.McpServerHttp{
				Name:    s.Name,
				Type:    "http",
				Url:     s.URL,
				Headers: convertHeaders(s.Headers),
			}
		case "sse":
			ms.Sse = &acp.McpServerSse{
				Name:    s.Name,
				Type:    "sse",
				Url:     s.URL,
				Headers: convertHeaders(s.Headers),
			}
		default: // "stdio" or unspecified
			ms.Stdio = &acp.McpServerStdio{
				Name:    s.Name,
				Command: s.Command,
				Args:    s.Args,
				Env:     convertEnv(s.Env),
			}
		}
		result = append(result, ms)
	}
	return result
}

func convertHeaders(h map[string]string) []acp.HttpHeader {
	headers := make([]acp.HttpHeader, 0, len(h))
	for k, v := range h {
		headers = append(headers, acp.HttpHeader{Name: k, Value: v})
	}
	return headers
}

func convertEnv(e map[string]string) []acp.EnvVariable {
	env := make([]acp.EnvVariable, 0, len(e))
	for k, v := range e {
		env = append(env, acp.EnvVariable{Name: k, Value: v})
	}
	return env
}
