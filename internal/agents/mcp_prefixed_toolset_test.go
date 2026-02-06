package agents

import (
	"testing"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// mockTool implements the tool.Tool interface for testing
type mockTool struct {
	name        string
	description string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return t.description }
func (t *mockTool) IsLongRunning() bool { return false }

// mockToolWithDeclaration implements tool.Tool plus Declaration method
type mockToolWithDeclaration struct {
	mockTool
	declaration *genai.FunctionDeclaration
}

func (t *mockToolWithDeclaration) Declaration() *genai.FunctionDeclaration {
	return t.declaration
}

func (t *mockToolWithDeclaration) Run(_ tool.Context, _ any) (map[string]any, error) {
	return map[string]any{"result": "success"}, nil
}

func (t *mockToolWithDeclaration) ProcessRequest(_ tool.Context, _ *model.LLMRequest) error {
	return nil
}

// mockToolset implements tool.Toolset for testing
type mockToolset struct {
	name  string
	tools []tool.Tool
}

func (ts *mockToolset) Name() string { return ts.name }
func (ts *mockToolset) Tools(_ agent.ReadonlyContext) ([]tool.Tool, error) {
	return ts.tools, nil
}

func TestPrefixedMCPToolset_Name(t *testing.T) {
	inner := &mockToolset{name: "inner_toolset"}
	prefixed := newPrefixedMCPToolset("my_server", inner)

	want := "mcp__my_server"
	got := prefixed.Name()
	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestPrefixedMCPToolset_Tools(t *testing.T) {
	innerTools := []tool.Tool{
		&mockTool{name: "tool_a", description: "Tool A description"},
		&mockTool{name: "tool_b", description: "Tool B description"},
	}
	inner := &mockToolset{name: "inner", tools: innerTools}
	prefixed := newPrefixedMCPToolset("my_server", inner)

	tools, err := prefixed.Tools(nil)
	if err != nil {
		t.Fatalf("Tools() error = %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("Tools() returned %d tools, want 2", len(tools))
	}

	// Check first tool
	if got, want := tools[0].Name(), "mcp__my_server__tool_a"; got != want {
		t.Errorf("tools[0].Name() = %q, want %q", got, want)
	}
	if got, want := tools[0].Description(), "Tool A description"; got != want {
		t.Errorf("tools[0].Description() = %q, want %q", got, want)
	}

	// Check second tool
	if got, want := tools[1].Name(), "mcp__my_server__tool_b"; got != want {
		t.Errorf("tools[1].Name() = %q, want %q", got, want)
	}
}

func TestPrefixedTool_Declaration(t *testing.T) {
	innerDecl := &genai.FunctionDeclaration{
		Name:        "original_tool",
		Description: "Original description",
		ParametersJsonSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param1": map[string]any{"type": "string"},
			},
		},
	}
	inner := &mockToolWithDeclaration{
		mockTool:    mockTool{name: "original_tool", description: "Original description"},
		declaration: innerDecl,
	}

	prefixed := newPrefixedTool("test_server", inner)

	// Check the prefixed name
	if got, want := prefixed.Name(), "mcp__test_server__original_tool"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}

	// Check the declaration has the prefixed name
	decl := prefixed.Declaration()
	if decl == nil {
		t.Fatal("Declaration() returned nil")
	}
	if got, want := decl.Name, "mcp__test_server__original_tool"; got != want {
		t.Errorf("Declaration().Name = %q, want %q", got, want)
	}
	if got, want := decl.Description, "Original description"; got != want {
		t.Errorf("Declaration().Description = %q, want %q", got, want)
	}
	if decl.ParametersJsonSchema == nil {
		t.Error("Declaration().ParametersJsonSchema is nil, expected to be preserved")
	}
}

func TestPrefixedTool_Run(t *testing.T) {
	inner := &mockToolWithDeclaration{
		mockTool:    mockTool{name: "runnable_tool", description: "Runnable"},
		declaration: &genai.FunctionDeclaration{Name: "runnable_tool"},
	}

	prefixed := newPrefixedTool("server", inner)

	result, err := prefixed.Run(nil, map[string]any{"input": "test"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil")
	}
	if got, want := result["result"], "success"; got != want {
		t.Errorf("Run() result[\"result\"] = %v, want %v", got, want)
	}
}

func TestPrefixedTool_NilDeclaration(t *testing.T) {
	// Test with a tool that doesn't implement Declaration
	inner := &mockTool{name: "simple_tool", description: "Simple"}
	prefixed := newPrefixedTool("server", inner)

	decl := prefixed.Declaration()
	if decl != nil {
		t.Errorf("Declaration() = %v, want nil for tool without Declaration method", decl)
	}
}

func TestPrefixedToolset_PreventsDuplicateNames(t *testing.T) {
	// Simulate two servers with the same tool name
	server1Tools := []tool.Tool{
		&mockTool{name: "read_file", description: "Read file from server 1"},
	}
	server2Tools := []tool.Tool{
		&mockTool{name: "read_file", description: "Read file from server 2"},
	}

	inner1 := &mockToolset{name: "server1", tools: server1Tools}
	inner2 := &mockToolset{name: "server2", tools: server2Tools}

	prefixed1 := newPrefixedMCPToolset("filesystem", inner1)
	prefixed2 := newPrefixedMCPToolset("github", inner2)

	tools1, _ := prefixed1.Tools(nil)
	tools2, _ := prefixed2.Tools(nil)

	// Both tools should have unique names now
	name1 := tools1[0].Name()
	name2 := tools2[0].Name()

	if name1 == name2 {
		t.Errorf("Tool names should be different, both got %q", name1)
	}

	if name1 != "mcp__filesystem__read_file" {
		t.Errorf("tools1[0].Name() = %q, want %q", name1, "mcp__filesystem__read_file")
	}
	if name2 != "mcp__github__read_file" {
		t.Errorf("tools2[0].Name() = %q, want %q", name2, "mcp__github__read_file")
	}
}
