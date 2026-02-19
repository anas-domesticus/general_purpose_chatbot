package agents

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExtractAllContent_TextOnly(t *testing.T) {
	content := []mcp.Content{
		&mcp.TextContent{Text: "hello world"},
	}
	got := extractAllContent(content)
	if got != "hello world" {
		t.Errorf("extractAllContent() = %q, want %q", got, "hello world")
	}
}

func TestExtractAllContent_EmbeddedResourceText(t *testing.T) {
	content := []mcp.Content{
		&mcp.TextContent{Text: "successfully downloaded text file (SHA: abc123)"},
		&mcp.EmbeddedResource{
			Resource: &mcp.ResourceContents{
				URI:      "repo://owner/repo/contents/main.go",
				MIMEType: "text/x-go; charset=utf-8",
				Text:     "package main\n\nfunc main() {}\n",
			},
		},
	}
	got := extractAllContent(content)
	want := "successfully downloaded text file (SHA: abc123)package main\n\nfunc main() {}\n"
	if got != want {
		t.Errorf("extractAllContent() = %q, want %q", got, want)
	}
}

func TestExtractAllContent_EmbeddedResourceBlobText(t *testing.T) {
	content := []mcp.Content{
		&mcp.EmbeddedResource{
			Resource: &mcp.ResourceContents{
				URI:      "repo://owner/repo/contents/data.json",
				MIMEType: "application/json",
				Blob:     []byte(`{"key": "value"}`),
			},
		},
	}
	got := extractAllContent(content)
	want := `{"key": "value"}`
	if got != want {
		t.Errorf("extractAllContent() = %q, want %q", got, want)
	}
}

func TestExtractAllContent_EmbeddedResourceBlobBinary(t *testing.T) {
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47}
	content := []mcp.Content{
		&mcp.EmbeddedResource{
			Resource: &mcp.ResourceContents{
				URI:      "repo://owner/repo/contents/image.png",
				MIMEType: "image/png",
				Blob:     binaryData,
			},
		},
	}
	got := extractAllContent(content)
	if got == "" {
		t.Error("extractAllContent() returned empty string for binary resource")
	}
	if !contains(got, "image/png") {
		t.Errorf("extractAllContent() = %q, should contain MIME type", got)
	}
	if !contains(got, "base64:") {
		t.Errorf("extractAllContent() = %q, should contain base64 data", got)
	}
}

func TestExtractAllContent_NilResource(t *testing.T) {
	content := []mcp.Content{
		&mcp.EmbeddedResource{Resource: nil},
	}
	got := extractAllContent(content)
	if got != "" {
		t.Errorf("extractAllContent() = %q, want empty string for nil resource", got)
	}
}

func TestExtractAllContent_ImageContent(t *testing.T) {
	content := []mcp.Content{
		&mcp.ImageContent{MIMEType: "image/jpeg", Data: make([]byte, 1024)},
	}
	got := extractAllContent(content)
	if got == "" {
		t.Error("extractAllContent() returned empty string for ImageContent")
	}
	if !contains(got, "image/jpeg") {
		t.Errorf("extractAllContent() = %q, should mention MIME type", got)
	}
}

func TestExtractAllContent_ResourceLink(t *testing.T) {
	content := []mcp.Content{
		&mcp.ResourceLink{URI: "repo://owner/repo/contents/big.bin", Name: "big.bin"},
	}
	got := extractAllContent(content)
	if got == "" {
		t.Error("extractAllContent() returned empty string for ResourceLink")
	}
	if !contains(got, "big.bin") {
		t.Errorf("extractAllContent() = %q, should contain resource name", got)
	}
}

func TestExtractAllContent_MixedContent(t *testing.T) {
	content := []mcp.Content{
		&mcp.TextContent{Text: "summary: "},
		&mcp.EmbeddedResource{
			Resource: &mcp.ResourceContents{
				URI:      "file://test.txt",
				MIMEType: "text/plain",
				Text:     "file content here",
			},
		},
	}
	got := extractAllContent(content)
	want := "summary: file content here"
	if got != want {
		t.Errorf("extractAllContent() = %q, want %q", got, want)
	}
}

func TestExtractTextFromContent_ErrorDetails(t *testing.T) {
	content := []mcp.Content{
		&mcp.TextContent{Text: "file not found"},
		&mcp.ImageContent{MIMEType: "image/png", Data: []byte("ignored")},
	}
	got := extractTextFromContent(content)
	if got != "file not found" {
		t.Errorf("extractTextFromContent() = %q, want %q", got, "file not found")
	}
}

func TestExtractTextFromContent_WithEmbeddedResource(t *testing.T) {
	content := []mcp.Content{
		&mcp.TextContent{Text: "error: "},
		&mcp.EmbeddedResource{
			Resource: &mcp.ResourceContents{
				URI:  "file://error.log",
				Text: "detailed error info",
			},
		},
	}
	got := extractTextFromContent(content)
	want := "error: detailed error info"
	if got != want {
		t.Errorf("extractTextFromContent() = %q, want %q", got, want)
	}
}

func TestIsTextMIMEType(t *testing.T) {
	tests := []struct {
		mimeType string
		want     bool
	}{
		{"text/plain", true},
		{"text/html", true},
		{"text/x-go", true},
		{"text/x-python; charset=utf-8", true},
		{"application/json", true},
		{"application/xml", true},
		{"application/javascript", true},
		{"application/yaml", true},
		{"application/x-yaml", true},
		{"application/toml", true},
		{"application/xhtml+xml", true},
		{"image/png", false},
		{"image/jpeg", false},
		{"audio/wav", false},
		{"application/octet-stream", false},
		{"application/pdf", false},
	}
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			if got := isTextMIMEType(tt.mimeType); got != tt.want {
				t.Errorf("isTextMIMEType(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
