// Package prefixed_uuid provides UUID generation with customisable prefixes.
package prefixed_uuid

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// PrefixedUUID represents a UUID with a prefix string.
type PrefixedUUID struct {
	Prefix string
	UUID   uuid.UUID
}

// New creates a new PrefixedUUID with the given prefix and a generated UUID.
func New(prefix string) PrefixedUUID {
	return PrefixedUUID{
		Prefix: prefix,
		UUID:   uuid.New(),
	}
}

// FromUUID creates a PrefixedUUID from an existing UUID and prefix.
func FromUUID(prefix string, id uuid.UUID) PrefixedUUID {
	return PrefixedUUID{
		Prefix: prefix,
		UUID:   id,
	}
}

// FromString parses a prefixed UUID string in the format "prefix-uuid".
func FromString(s string) (PrefixedUUID, error) {
	idx := strings.Index(s, "-")
	if idx == -1 {
		return PrefixedUUID{}, fmt.Errorf("invalid prefixed UUID format: %s", s)
	}

	prefix := s[:idx]
	uuidStr := s[idx+1:]

	parsedUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		return PrefixedUUID{}, fmt.Errorf("invalid UUID: %w", err)
	}

	return PrefixedUUID{
		Prefix: prefix,
		UUID:   parsedUUID,
	}, nil
}

// RawUUID returns the underlying UUID without the prefix.
func (p PrefixedUUID) RawUUID() uuid.UUID {
	return p.UUID
}

// String implements the fmt.Stringer interface.
// It returns the prefixed UUID in the format "prefix-uuid".
func (p PrefixedUUID) String() string {
	return fmt.Sprintf("%s-%s", p.Prefix, p.UUID.String())
}

// IsZero returns true if the PrefixedUUID is uninitialized (zero value).
func (p PrefixedUUID) IsZero() bool {
	return p.Prefix == "" && p.UUID == uuid.Nil
}

// Equal returns true if two PrefixedUUIDs are equal.
func (p PrefixedUUID) Equal(other PrefixedUUID) bool {
	return p.Prefix == other.Prefix && p.UUID == other.UUID
}

// MarshalJSON implements json.Marshaler.
// Serialises the PrefixedUUID as a JSON string.
func (p PrefixedUUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshalJSON implements json.Unmarshaler.
// Deserializes a PrefixedUUID from a JSON string.
func (p *PrefixedUUID) UnmarshalJSON(data []byte) error {
	// Remove quotes from JSON string
	s := string(data)
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return fmt.Errorf("invalid JSON string format")
	}
	s = s[1 : len(s)-1]

	parsed, err := FromString(s)
	if err != nil {
		return err
	}

	*p = parsed
	return nil
}
