# Prefixed UUID

Type-safe UUIDs with human-readable prefixes for better debugging and type identification.

## Purpose
Creates identifiable UUIDs with prefixes like `user-123e4567-e89b-12d3-a456-426614174000` to make IDs self-documenting and easier to work with in logs and debugging.

## Features
- **Self-documenting**: Prefix indicates entity type at a glance
- **Type-safe**: Strong typing prevents ID confusion
- **JSON compatible**: Automatic serialization/deserialization
- **fmt.Stringer**: Works with logging and string formatting
- **UUID extraction**: Access underlying UUID when needed
- **Zero-value detection**: Check for uninitialized UUIDs
- **Equality comparison**: Type-safe comparison of prefixed UUIDs

## Basic Usage

```go
import "github.com/lewisedginton/go_project_boilerplate/pkg/prefixed_uuid"

// Create new prefixed UUID
userID := prefixed_uuid.New("user")
// Result: user-123e4567-e89b-12d3-a456-426614174000

// Create from existing UUID
existingUUID := uuid.New()
orderID := prefixed_uuid.FromUUID("order", existingUUID)

// Parse from string
parsed, err := prefixed_uuid.FromString("user-123e4567-e89b-12d3-a456-426614174000")
if err != nil {
    // Handle error
}

// Convert to string (implements fmt.Stringer)
idStr := userID.String()

// Get raw UUID without prefix
rawUUID := userID.RawUUID()

// Check equality
if userID.Equal(otherID) {
    // IDs are equal
}

// Check if zero value
if userID.IsZero() {
    // Uninitialized
}
```

## JSON Serialization

Prefixed UUIDs automatically serialize to/from JSON as strings:

```go
type User struct {
    ID   prefixed_uuid.PrefixedUUID `json:"id"`
    Name string                     `json:"name"`
    Age  int                        `json:"age"`
}

user := User{
    ID:   prefixed_uuid.New("user"),
    Name: "Alice",
    Age:  30,
}

// Serializes as: {"id": "user-123e4567-e89b-12d3-a456-426614174000", "name": "Alice", "age": 30}
data, _ := json.Marshal(user)

// Deserializes from JSON string
var parsed User
json.Unmarshal(data, &parsed)
```

## Common Prefixes

```go
userID    := prefixed_uuid.New("user")     // User accounts
orderID   := prefixed_uuid.New("order")    // Orders/transactions  
productID := prefixed_uuid.New("product")  // Products/items
sessionID := prefixed_uuid.New("session")  // User sessions
imageID   := prefixed_uuid.New("img")      // Images/media
orgID     := prefixed_uuid.New("org")      // Organizations
teamID    := prefixed_uuid.New("team")     // Teams/groups
apiKeyID  := prefixed_uuid.New("key")      // API keys
```

## Integration with Logging

Works seamlessly with the logger package:

```go
import (
    "github.com/lewisedginton/go_project_boilerplate/pkg/logger"
    "github.com/lewisedginton/go_project_boilerplate/pkg/prefixed_uuid"
)

userID := prefixed_uuid.New("user")

// Automatically converts to string for logging
log.Info("User created",
    logger.StringField("user_id", userID.String()),
    logger.StringField("user_prefix", userID.Prefix),
)

// Or use the generic field helper
log.Info("Processing user",
    logger.Field("user_id", userID),
)
```

## Error Handling

```go
// FromString validates both prefix format and UUID format
id, err := prefixed_uuid.FromString("invalid-format")
if err != nil {
    // Handle parsing errors:
    // - Missing dash separator
    // - Invalid UUID format  
    // - Empty string
}

// JSON unmarshaling also validates format
var user User
err := json.Unmarshal([]byte(`{"id": "user-invalid-uuid"}`), &user)
if err != nil {
    // Handle invalid UUID in JSON
}
```

## Advanced Usage

### Custom Validation

```go
func validateUserID(id string) error {
    parsed, err := prefixed_uuid.FromString(id)
    if err != nil {
        return fmt.Errorf("invalid user ID format: %w", err)
    }
    
    if parsed.Prefix != "user" {
        return fmt.Errorf("expected user prefix, got: %s", parsed.Prefix)
    }
    
    return nil
}
```

### Database Storage

```go
// Store as string in database
userID := prefixed_uuid.New("user")
_, err := db.Exec("INSERT INTO users (id, name) VALUES (?, ?)", userID.String(), "Alice")

// Retrieve and parse
var idStr string
var name string
err := db.QueryRow("SELECT id, name FROM users WHERE id = ?", userID.String()).Scan(&idStr, &name)

parsedID, err := prefixed_uuid.FromString(idStr)
```

### Multiple Prefixes

```go
type ResourceID struct {
    prefixed_uuid.PrefixedUUID
}

func NewUserID() ResourceID {
    return ResourceID{prefixed_uuid.New("user")}
}

func NewOrderID() ResourceID {  
    return ResourceID{prefixed_uuid.New("order")}
}

// Type-safe resource identification
func (r ResourceID) IsUser() bool {
    return r.Prefix == "user"
}

func (r ResourceID) IsOrder() bool {
    return r.Prefix == "order"  
}
```

## Testing

```go
func TestWithPrefixedUUID(t *testing.T) {
    // Create deterministic UUID for testing
    fixedUUID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
    userID := prefixed_uuid.FromUUID("user", fixedUUID)
    
    assert.Equal(t, "user-123e4567-e89b-12d3-a456-426614174000", userID.String())
    assert.Equal(t, "user", userID.Prefix)
    assert.Equal(t, fixedUUID, userID.RawUUID())
}
```