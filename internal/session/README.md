# JSON Session Service

A custom implementation of the ADK `session.Service` interface that stores session data in JSON files. The implementation supports pluggable storage backends including local filesystem and AWS S3.

## Features

- **JSON File Storage**: Sessions are stored as JSON files with human-readable format
- **Pluggable Storage**: Supports local filesystem and AWS S3 backends
- **Thread-Safe**: Uses mutex for concurrent access protection
- **In-Memory Caching**: Improves performance with optional caching layer
- **ADK Compatible**: Implements the full `session.Service` interface

## Usage

### Local File Storage

```go
import "github.com/lewisedginton/general_purpose_chatbot/internal/session"

// Simple creation
sessionService := session.NewLocalJSONSessionService("/path/to/sessions")

// Using builder
sessionService, err := session.NewSessionServiceBuilder().
    WithLocalFileStorage("/path/to/sessions").
    Build()
```

### AWS S3 Storage

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/lewisedginton/general_purpose_chatbot/internal/session"
)

// Create S3 client
s3Client := s3.NewFromConfig(awsConfig)
awsS3Client := session.NewAWSS3Client(s3Client)

// Create session service
sessionService := session.NewS3JSONSessionService(
    "my-bucket", 
    "session-prefix", 
    awsS3Client,
)

// Using builder
sessionService, err := session.NewSessionServiceBuilder().
    WithS3Storage("my-bucket", "session-prefix", awsS3Client).
    Build()
```

### Custom File Provider

```go
// Implement the FileProvider interface
type MyCustomProvider struct {
    // your implementation
}

func (p *MyCustomProvider) Read(ctx context.Context, path string) ([]byte, error) {
    // your implementation
}

// ... implement other methods

// Use with builder
sessionService, err := session.NewSessionServiceBuilder().
    WithCustomFileProvider(&MyCustomProvider{}).
    Build()
```

## File Structure

### Local Filesystem

Sessions are stored in a hierarchical directory structure:

```
/sessions/
├── app1/
│   ├── user1/
│   │   ├── session1.json
│   │   └── session2.json
│   └── user2/
│       └── session3.json
└── app2/
    └── user1/
        └── session4.json
```

### AWS S3

For S3, the same structure is maintained using object keys:

```
sessions/app1/user1/session1.json
sessions/app1/user1/session2.json
sessions/app1/user2/session3.json
sessions/app2/user1/session4.json
```

## JSON Format

Each session is stored as a JSON file:

```json
{
  "app_name": "my-app",
  "user_id": "user123",
  "session_id": "session_abc",
  "created_at": "2023-01-01T12:00:00Z",
  "updated_at": "2023-01-01T12:30:00Z",
  "state": null
}
```

## Integration with ADK

Replace the default in-memory session service in your ADK configuration:

```go
// Before
sessionService := session.InMemoryService()

// After
sessionService := session.NewLocalJSONSessionService("/path/to/sessions")

// Create agent loader with JSON session service
agentLoader := agents.NewLoader(claudeModel, cfg.MCP, sessionService)
```

## Configuration

The session service can be configured through environment variables or config files. Example configuration in your main application:

```go
func createSessionService(cfg *config.Config) session.Service {
    switch cfg.Session.Backend {
    case "local":
        return session.NewLocalJSONSessionService(cfg.Session.LocalPath)
    case "s3":
        s3Client := createS3Client(cfg.AWS)
        awsS3Client := session.NewAWSS3Client(s3Client)
        return session.NewS3JSONSessionService(
            cfg.Session.S3Bucket,
            cfg.Session.S3Prefix,
            awsS3Client,
        )
    default:
        return session.InMemoryService() // fallback
    }
}
```

## Testing

The package includes comprehensive tests and a mock S3 client for testing:

```bash
cd internal/session
go test -v
```

For S3 integration tests, you can use the mock S3 client:

```go
s3Client := session.NewMockS3Client()
sessionService := session.NewS3JSONSessionService("test-bucket", "test-prefix", s3Client)
```

## Performance Considerations

- **Caching**: The service includes in-memory caching to reduce file I/O
- **Concurrency**: Uses RWMutex for thread-safe operations
- **S3 Operations**: S3 operations are subject to AWS API limits and costs
- **Local Files**: Local filesystem operations are generally faster but require local storage

## Error Handling

The service handles various error conditions:

- Invalid requests (nil parameters, missing required fields)
- File system errors (permissions, disk space)
- S3 errors (network issues, authentication)
- JSON parsing errors (corrupted files)

## Migration

To migrate from the in-memory session service:

1. Update your session service creation code
2. Sessions will be created fresh (no existing in-memory data is preserved)
3. Consider implementing a migration script if you need to preserve existing session data

## AWS Permissions

When using S3 storage, ensure your AWS credentials have the following permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:HeadObject",
                "s3:ListObjects*"
            ],
            "Resource": [
                "arn:aws:s3:::your-bucket/*",
                "arn:aws:s3:::your-bucket"
            ]
        }
    ]
}
```