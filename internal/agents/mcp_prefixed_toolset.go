// Package agents provides AI agent creation and management.
package agents

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// MCPToolPrefix is the prefix added to all MCP tool names.
// The format is: mcp__{serverName}__{toolName}
const MCPToolPrefix = "mcp__"

// prefixedMCPToolset wraps an MCP toolset and prefixes all tool names
// to avoid conflicts when multiple MCP servers expose tools with the same name.
type prefixedMCPToolset struct {
	serverName string
	inner      tool.Toolset
}

// newPrefixedMCPToolset creates a new toolset wrapper that prefixes all tools
// from the given toolset with the server name.
func newPrefixedMCPToolset(serverName string, inner tool.Toolset) tool.Toolset {
	return &prefixedMCPToolset{
		serverName: serverName,
		inner:      inner,
	}
}

// Name returns the name of the toolset.
func (p *prefixedMCPToolset) Name() string {
	return MCPToolPrefix + p.serverName
}

// Tools returns the list of tools with prefixed names.
func (p *prefixedMCPToolset) Tools(ctx agent.ReadonlyContext) ([]tool.Tool, error) {
	tools, err := p.inner.Tools(ctx)
	if err != nil {
		return nil, err
	}

	prefixedTools := make([]tool.Tool, len(tools))
	for i, t := range tools {
		prefixedTools[i] = newPrefixedTool(p.serverName, t)
	}
	return prefixedTools, nil
}

// prefixedTool wraps an MCP tool and exposes it with a prefixed name.
// This implements the same interfaces as the underlying MCP tool:
// - tool.Tool (Name, Description, IsLongRunning)
// - toolinternal.FunctionTool (Declaration, Run)
// - toolinternal.RequestProcessor (ProcessRequest)
type prefixedTool struct {
	serverName   string
	prefixedName string
	inner        tool.Tool
}

// newPrefixedTool creates a new tool wrapper with a prefixed name.
func newPrefixedTool(serverName string, inner tool.Tool) *prefixedTool {
	return &prefixedTool{
		serverName:   serverName,
		prefixedName: MCPToolPrefix + serverName + "__" + inner.Name(),
		inner:        inner,
	}
}

// Name returns the prefixed tool name (mcp__{serverName}__{toolName}).
func (t *prefixedTool) Name() string {
	return t.prefixedName
}

// Description returns the tool's description.
func (t *prefixedTool) Description() string {
	return t.inner.Description()
}

// IsLongRunning returns whether the tool is long-running.
func (t *prefixedTool) IsLongRunning() bool {
	return t.inner.IsLongRunning()
}

// Declaration returns the function declaration with the prefixed name.
// This is called when building LLM requests to expose the tool to the model.
func (t *prefixedTool) Declaration() *genai.FunctionDeclaration {
	// Get the inner tool's declaration using type assertion
	// The MCP tool implements this interface internally
	type declarator interface {
		Declaration() *genai.FunctionDeclaration
	}

	d, ok := t.inner.(declarator)
	if !ok {
		return nil
	}

	innerDecl := d.Declaration()
	if innerDecl == nil {
		return nil
	}

	// Create a copy of the declaration with the prefixed name
	return &genai.FunctionDeclaration{
		Name:                 t.prefixedName,
		Description:          innerDecl.Description,
		Parameters:           innerDecl.Parameters,
		ParametersJsonSchema: innerDecl.ParametersJsonSchema,
		Response:             innerDecl.Response,
		ResponseJsonSchema:   innerDecl.ResponseJsonSchema,
		Behavior:             innerDecl.Behavior,
	}
}

// Run executes the tool. The underlying MCP tool uses its original name
// when communicating with the MCP server.
func (t *prefixedTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	// Get the inner tool's Run method using type assertion
	type runner interface {
		Run(ctx tool.Context, args any) (map[string]any, error)
	}

	r, ok := t.inner.(runner)
	if !ok {
		return nil, nil
	}

	return r.Run(ctx, args)
}

// ProcessRequest processes the LLM request by adding this tool's declaration.
// This implements the toolinternal.RequestProcessor interface.
func (t *prefixedTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	// Pack the tool into the request using the prefixed name
	return packTool(req, t)
}

// packTool adds a tool to the LLM request.
// This is based on toolutils.PackTool from the ADK but works with our prefixed tool.
func packTool(req *model.LLMRequest, t *prefixedTool) error {
	if req.Tools == nil {
		req.Tools = make(map[string]any)
	}

	name := t.Name()
	if _, ok := req.Tools[name]; ok {
		// Tool already registered (shouldn't happen with prefixed names)
		return nil
	}
	req.Tools[name] = t

	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}

	decl := t.Declaration()
	if decl == nil {
		return nil
	}

	// Find an existing genai.Tool with FunctionDeclarations
	var funcTool *genai.Tool
	for _, gt := range req.Config.Tools {
		if gt != nil && gt.FunctionDeclarations != nil {
			funcTool = gt
			break
		}
	}

	if funcTool == nil {
		req.Config.Tools = append(req.Config.Tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{decl},
		})
	} else {
		funcTool.FunctionDeclarations = append(funcTool.FunctionDeclarations, decl)
	}

	return nil
}
