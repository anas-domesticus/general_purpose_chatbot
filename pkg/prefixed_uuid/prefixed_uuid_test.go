package prefixed_uuid

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	p := New("user")
	if p.Prefix != "user" {
		t.Errorf("expected prefix 'user', got '%s'", p.Prefix)
	}
	if p.UUID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
}

func TestFromUUID(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	p := FromUUID("order", id)

	if p.Prefix != "order" {
		t.Errorf("expected prefix 'order', got '%s'", p.Prefix)
	}
	if p.UUID != id {
		t.Errorf("expected UUID %s, got %s", id, p.UUID)
	}
}

func TestString(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	p := FromUUID("book", id)

	expected := "book-123e4567-e89b-12d3-a456-426614174000"
	got := p.String()

	if got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}
}

func TestFromString(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPrefix string
		wantUUID   string
		wantErr    bool
	}{
		{
			name:       "valid simple prefix",
			input:      "user-123e4567-e89b-12d3-a456-426614174000",
			wantPrefix: "user",
			wantUUID:   "123e4567-e89b-12d3-a456-426614174000",
			wantErr:    false,
		},
		{
			name:       "valid multi-word prefix",
			input:      "content_book-123e4567-e89b-12d3-a456-426614174000",
			wantPrefix: "content_book",
			wantUUID:   "123e4567-e89b-12d3-a456-426614174000",
			wantErr:    false,
		},
		{
			name:    "missing dash separator",
			input:   "user123e4567e89b12d3a456426614174000",
			wantErr: true,
		},
		{
			name:    "invalid UUID",
			input:   "user-not-a-valid-uuid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only UUID",
			input:   "123e4567-e89b-12d3-a456-426614174000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromString(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got.Prefix != tt.wantPrefix {
				t.Errorf("expected prefix '%s', got '%s'", tt.wantPrefix, got.Prefix)
			}

			if got.UUID.String() != tt.wantUUID {
				t.Errorf("expected UUID '%s', got '%s'", tt.wantUUID, got.UUID.String())
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"simple prefix", "user"},
		{"underscore prefix", "content_book"},
		{"number prefix", "order123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new UUID
			original := New(tt.prefix)

			// Convert to string
			s := original.String()

			// Parse back
			parsed, err := FromString(s)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Should be equal
			if !original.Equal(parsed) {
				t.Errorf("round trip failed: original=%s, parsed=%s", original, parsed)
			}
		})
	}
}

func TestRawUUID(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	p := FromUUID("test", id)

	raw := p.RawUUID()
	if raw != id {
		t.Errorf("expected %s, got %s", id, raw)
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name     string
		p        PrefixedUUID
		wantZero bool
	}{
		{
			name:     "zero value",
			p:        PrefixedUUID{},
			wantZero: true,
		},
		{
			name:     "with prefix only",
			p:        PrefixedUUID{Prefix: "user", UUID: uuid.Nil},
			wantZero: false,
		},
		{
			name:     "with UUID only",
			p:        PrefixedUUID{UUID: uuid.New()},
			wantZero: false,
		},
		{
			name:     "fully initialized",
			p:        New("user"),
			wantZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.IsZero()
			if got != tt.wantZero {
				t.Errorf("IsZero() = %v, want %v", got, tt.wantZero)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	id1 := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	id2 := uuid.MustParse("987e6543-e21b-12d3-a456-426614174000")

	tests := []struct {
		name      string
		p1        PrefixedUUID
		p2        PrefixedUUID
		wantEqual bool
	}{
		{
			name:      "identical",
			p1:        FromUUID("user", id1),
			p2:        FromUUID("user", id1),
			wantEqual: true,
		},
		{
			name:      "different prefix",
			p1:        FromUUID("user", id1),
			p2:        FromUUID("order", id1),
			wantEqual: false,
		},
		{
			name:      "different UUID",
			p1:        FromUUID("user", id1),
			p2:        FromUUID("user", id2),
			wantEqual: false,
		},
		{
			name:      "both different",
			p1:        FromUUID("user", id1),
			p2:        FromUUID("order", id2),
			wantEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p1.Equal(tt.p2)
			if got != tt.wantEqual {
				t.Errorf("Equal() = %v, want %v", got, tt.wantEqual)
			}
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	p := FromUUID("user", id)

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	expected := `"user-123e4567-e89b-12d3-a456-426614174000"`
	got := string(data)

	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestMarshalJSONWithSpecialCharacters(t *testing.T) {
	// Test that special characters in prefix are properly escaped
	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	p := FromUUID(`user"quoted`, id)

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Should be valid JSON - unmarshal to verify
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		t.Errorf("produced invalid JSON: %v, data=%s", err, string(data))
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPrefix string
		wantUUID   string
		wantErr    bool
	}{
		{
			name:       "valid",
			input:      `"user-123e4567-e89b-12d3-a456-426614174000"`,
			wantPrefix: "user",
			wantUUID:   "123e4567-e89b-12d3-a456-426614174000",
			wantErr:    false,
		},
		{
			name:    "invalid JSON",
			input:   `user-123e4567-e89b-12d3-a456-426614174000`,
			wantErr: true,
		},
		{
			name:    "invalid UUID",
			input:   `"user-not-a-uuid"`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   `""`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p PrefixedUUID
			err := json.Unmarshal([]byte(tt.input), &p)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if p.Prefix != tt.wantPrefix {
				t.Errorf("expected prefix '%s', got '%s'", tt.wantPrefix, p.Prefix)
			}

			if p.UUID.String() != tt.wantUUID {
				t.Errorf("expected UUID '%s', got '%s'", tt.wantUUID, p.UUID.String())
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := New("user")

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var parsed PrefixedUUID
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Should be equal
	if !original.Equal(parsed) {
		t.Errorf("JSON round trip failed: original=%s, parsed=%s", original, parsed)
	}
}

func TestJSONInStruct(t *testing.T) {
	type TestStruct struct {
		ID   PrefixedUUID `json:"id"`
		Name string       `json:"name"`
	}

	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	original := TestStruct{
		ID:   FromUUID("user", id),
		Name: "test",
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var parsed TestStruct
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify
	if !original.ID.Equal(parsed.ID) {
		t.Errorf("ID mismatch: original=%s, parsed=%s", original.ID, parsed.ID)
	}
	if original.Name != parsed.Name {
		t.Errorf("Name mismatch: original=%s, parsed=%s", original.Name, parsed.Name)
	}
}