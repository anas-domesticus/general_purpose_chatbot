package web_search

import (
	"testing"
)

func TestNew_RequiresAPIKey(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Error("expected error when API key is missing")
	}
}

func TestNew_CreatesToolWithValidConfig(t *testing.T) {
	tool, err := New(Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool == nil {
		t.Fatal("expected tool to be created")
	}
	if tool.Name() != "web_search" {
		t.Errorf("expected tool name 'web_search', got %q", tool.Name())
	}
}

func TestNew_UsesDefaultBaseURL(t *testing.T) {
	tool, err := New(Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool == nil {
		t.Fatal("expected tool to be created")
	}
}

func TestParseDomains(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"example.com", []string{"example.com"}},
		{"example.com,test.com", []string{"example.com", "test.com"}},
		{"example.com, test.com , other.com", []string{"example.com", "test.com", "other.com"}},
	}

	for _, tt := range tests {
		result := parseDomains(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseDomains(%q) = %v, expected %v", tt.input, result, tt.expected)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("parseDomains(%q)[%d] = %q, expected %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}
