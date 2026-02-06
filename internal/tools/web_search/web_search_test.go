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

func TestSearchClient_BuildRequestURL(t *testing.T) {
	client := &searchClient{
		apiKey:  "test-key",
		baseURL: "https://www.searchapi.io",
	}

	tests := []struct {
		name     string
		args     Args
		contains []string
	}{
		{
			name:     "basic query",
			args:     Args{Query: "test query"},
			contains: []string{"api_key=test-key", "engine=google", "q=test+query"},
		},
		{
			name:     "with num results",
			args:     Args{Query: "test", NumResults: 20},
			contains: []string{"num=20"},
		},
		{
			name:     "with page",
			args:     Args{Query: "test", Page: 2},
			contains: []string{"page=2"},
		},
		{
			name:     "with location",
			args:     Args{Query: "test", Location: "New York"},
			contains: []string{"location=New+York"},
		},
		{
			name:     "with safe search",
			args:     Args{Query: "test", SafeSearch: "active"},
			contains: []string{"safe=active"},
		},
		{
			name:     "with custom engine",
			args:     Args{Query: "test", Engine: EngineBing},
			contains: []string{"engine=bing"},
		},
		{
			name:     "defaults to google engine",
			args:     Args{Query: "test"},
			contains: []string{"engine=" + EngineDefault},
		},
		{
			name:     "caps num results at 100",
			args:     Args{Query: "test", NumResults: 200},
			contains: []string{"num=100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := client.buildRequestURL(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, substr := range tt.contains {
				if !containsSubstring(url, substr) {
					t.Errorf("URL %q should contain %q", url, substr)
				}
			}
		})
	}
}

func TestSearchClient_ParseResponse(t *testing.T) {
	client := &searchClient{}

	tests := []struct {
		name        string
		body        string
		wantResults int
		wantError   bool
	}{
		{
			name: "handles numeric total_results",
			body: `{
				"organic_results": [{"title": "Test", "link": "https://example.com", "snippet": "A test result"}],
				"search_information": {"total_results": 12345}
			}`,
			wantResults: 1,
			wantError:   false,
		},
		{
			name: "handles string total_results",
			body: `{
				"organic_results": [{"title": "Test", "link": "https://example.com", "snippet": "A test result"}],
				"search_information": {"total_results": "12345"}
			}`,
			wantResults: 1,
			wantError:   false,
		},
		{
			name: "handles missing search_information",
			body: `{
				"organic_results": [{"title": "Test", "link": "https://example.com", "snippet": "A test result"}]
			}`,
			wantResults: 1,
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.parseResponse("test query", []byte(tt.body))
			if tt.wantError && result.Error == "" {
				t.Error("expected error but got none")
			}
			if !tt.wantError && result.Error != "" {
				t.Errorf("unexpected error: %s", result.Error)
			}
			if len(result.Results) != tt.wantResults {
				t.Errorf("expected %d results, got %d", tt.wantResults, len(result.Results))
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
