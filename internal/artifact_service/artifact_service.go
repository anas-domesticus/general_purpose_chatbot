// Package artifact_service provides an implementation of the ADK artifact.Service interface.
// It supports both local filesystem and S3 storage backends through the FileProvider abstraction.
package artifact_service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/artifact"
	"google.golang.org/genai"
)

// ArtifactService implements the artifact.Service interface using JSON file storage.
type ArtifactService struct {
	fileProvider storage_manager.FileProvider
	mutex        sync.RWMutex
	log          logger.Logger
}

// ArtifactMetadata stores metadata about an artifact's versions.
type ArtifactMetadata struct {
	FileName       string    `json:"file_name"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []int64   `json:"versions"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VersionedArtifact stores the actual artifact content for a specific version.
type VersionedArtifact struct {
	Version   int64       `json:"version"`
	Part      *genai.Part `json:"part"`
	CreatedAt time.Time   `json:"created_at"`
}

// NewArtifactService creates a new artifact service with the given file provider.
// The provider should be obtained from a StorageManager, typically with an
// "artifacts" namespace prefix.
func NewArtifactService(provider storage_manager.FileProvider, log logger.Logger) *ArtifactService {
	if provider == nil {
		panic("file provider cannot be nil")
	}
	if log == nil {
		panic("logger cannot be nil")
	}
	return &ArtifactService{
		fileProvider: provider,
		log:          log,
	}
}

// Save saves an artifact to storage.
// If Version is specified in the request, it saves at that version.
// If Version is 0, it creates a new version by incrementing the current version.
func (s *ArtifactService) Save(ctx context.Context, req *artifact.SaveRequest) (*artifact.SaveResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid save request: %w", err)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	basePath := s.getArtifactBasePath(req.AppName, req.UserID, req.SessionID, req.FileName)
	metadataPath := path.Join(basePath, "metadata.json")

	// Load or create metadata
	metadata, err := s.loadMetadata(ctx, metadataPath)
	if err != nil {
		// Create new metadata if it doesn't exist
		metadata = &ArtifactMetadata{
			FileName:       req.FileName,
			CurrentVersion: 0,
			Versions:       []int64{},
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}

	// Determine version to save
	version := req.Version
	if version == 0 {
		// Auto-increment version
		version = metadata.CurrentVersion + 1
	}

	// Create versioned artifact
	versionedArtifact := &VersionedArtifact{
		Version:   version,
		Part:      req.Part,
		CreatedAt: time.Now(),
	}

	// Save version file
	versionPath := s.getVersionPath(basePath, version)
	versionData, err := json.MarshalIndent(versionedArtifact, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal artifact: %w", err)
	}

	if err := s.fileProvider.Write(ctx, versionPath, versionData); err != nil {
		return nil, fmt.Errorf("failed to save artifact version: %w", err)
	}

	// Update metadata
	if !slices.Contains(metadata.Versions, version) {
		metadata.Versions = append(metadata.Versions, version)
		slices.Sort(metadata.Versions)
	}
	if version > metadata.CurrentVersion {
		metadata.CurrentVersion = version
	}
	metadata.UpdatedAt = time.Now()

	// Save metadata
	if err := s.saveMetadata(ctx, metadataPath, metadata); err != nil {
		return nil, fmt.Errorf("failed to save artifact metadata: %w", err)
	}

	s.log.Debug("Saved artifact",
		logger.StringField("app", req.AppName),
		logger.StringField("user", req.UserID),
		logger.StringField("session", req.SessionID),
		logger.StringField("file", req.FileName),
		logger.Int64Field("version", version))

	return &artifact.SaveResponse{
		Version: version,
	}, nil
}

// Load loads an artifact from storage.
// If Version is 0, it loads the latest version.
func (s *ArtifactService) Load(ctx context.Context, req *artifact.LoadRequest) (*artifact.LoadResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid load request: %w", err)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	basePath := s.getArtifactBasePath(req.AppName, req.UserID, req.SessionID, req.FileName)
	metadataPath := path.Join(basePath, "metadata.json")

	// Load metadata to get version info
	metadata, err := s.loadMetadata(ctx, metadataPath)
	if err != nil {
		return nil, fmt.Errorf("artifact not found: %w", err)
	}

	// Determine which version to load
	version := req.Version
	if version == 0 {
		// Load latest version
		version = metadata.CurrentVersion
	}

	// Check if version exists
	if !slices.Contains(metadata.Versions, version) {
		return nil, fmt.Errorf("artifact version %d not found", version)
	}

	// Load version file
	versionPath := s.getVersionPath(basePath, version)
	data, err := s.fileProvider.Read(ctx, versionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact version: %w", err)
	}

	var versionedArtifact VersionedArtifact
	if err := json.Unmarshal(data, &versionedArtifact); err != nil {
		return nil, fmt.Errorf("failed to unmarshal artifact: %w", err)
	}

	s.log.Debug("Loaded artifact",
		logger.StringField("app", req.AppName),
		logger.StringField("user", req.UserID),
		logger.StringField("session", req.SessionID),
		logger.StringField("file", req.FileName),
		logger.Int64Field("version", version))

	return &artifact.LoadResponse{
		Part: versionedArtifact.Part,
	}, nil
}

// Delete removes an artifact.
// If Version is specified, it deletes only that version.
// If Version is 0, it deletes the entire artifact (all versions).
func (s *ArtifactService) Delete(ctx context.Context, req *artifact.DeleteRequest) error {
	if err := req.Validate(); err != nil {
		return fmt.Errorf("invalid delete request: %w", err)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	basePath := s.getArtifactBasePath(req.AppName, req.UserID, req.SessionID, req.FileName)
	metadataPath := path.Join(basePath, "metadata.json")

	if req.Version == 0 {
		// Delete all versions and metadata
		metadata, err := s.loadMetadata(ctx, metadataPath)
		if err != nil {
			// Artifact doesn't exist, consider it deleted
			return nil
		}

		// Delete all version files
		for _, version := range metadata.Versions {
			versionPath := s.getVersionPath(basePath, version)
			_ = s.fileProvider.Delete(ctx, versionPath) // Ignore errors for missing files
		}

		// Delete metadata
		_ = s.fileProvider.Delete(ctx, metadataPath)

		s.log.Debug("Deleted all artifact versions",
			logger.StringField("app", req.AppName),
			logger.StringField("user", req.UserID),
			logger.StringField("session", req.SessionID),
			logger.StringField("file", req.FileName))
	} else {
		// Delete specific version
		metadata, err := s.loadMetadata(ctx, metadataPath)
		if err != nil {
			// Artifact doesn't exist, consider it deleted
			return nil
		}

		// Delete version file
		versionPath := s.getVersionPath(basePath, req.Version)
		_ = s.fileProvider.Delete(ctx, versionPath)

		// Update metadata
		metadata.Versions = slices.DeleteFunc(metadata.Versions, func(v int64) bool {
			return v == req.Version
		})

		if len(metadata.Versions) == 0 {
			// No versions left, delete metadata too
			_ = s.fileProvider.Delete(ctx, metadataPath)
		} else {
			// Update current version if we deleted it
			if req.Version == metadata.CurrentVersion {
				metadata.CurrentVersion = metadata.Versions[len(metadata.Versions)-1]
			}
			metadata.UpdatedAt = time.Now()
			_ = s.saveMetadata(ctx, metadataPath, metadata)
		}

		s.log.Debug("Deleted artifact version",
			logger.StringField("app", req.AppName),
			logger.StringField("user", req.UserID),
			logger.StringField("session", req.SessionID),
			logger.StringField("file", req.FileName),
			logger.Int64Field("version", req.Version))
	}

	return nil
}

// List lists all artifact filenames within a session.
func (s *ArtifactService) List(ctx context.Context, req *artifact.ListRequest) (*artifact.ListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid list request: %w", err)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sessionPath := s.getSessionPath(req.AppName, req.UserID, req.SessionID)

	// List all files under the session path
	files, err := s.fileProvider.List(ctx, sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	// Extract unique filenames from paths
	fileNameSet := make(map[string]struct{})
	for _, file := range files {
		// Files are in format: fileName/metadata.json or fileName/versions/X.json
		// We want to extract the fileName part
		relPath := strings.TrimPrefix(file, sessionPath)
		relPath = strings.TrimPrefix(relPath, "/")

		parts := strings.SplitN(relPath, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			fileNameSet[parts[0]] = struct{}{}
		}
	}

	// Convert to slice
	fileNames := make([]string, 0, len(fileNameSet))
	for fileName := range fileNameSet {
		fileNames = append(fileNames, fileName)
	}
	slices.Sort(fileNames)

	s.log.Debug("Listed artifacts",
		logger.StringField("app", req.AppName),
		logger.StringField("user", req.UserID),
		logger.StringField("session", req.SessionID),
		logger.IntField("count", len(fileNames)))

	return &artifact.ListResponse{
		FileNames: fileNames,
	}, nil
}

// Versions lists all versions of an artifact.
func (s *ArtifactService) Versions(ctx context.Context, req *artifact.VersionsRequest) (*artifact.VersionsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid versions request: %w", err)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	basePath := s.getArtifactBasePath(req.AppName, req.UserID, req.SessionID, req.FileName)
	metadataPath := path.Join(basePath, "metadata.json")

	metadata, err := s.loadMetadata(ctx, metadataPath)
	if err != nil {
		// Artifact doesn't exist, return empty versions
		return &artifact.VersionsResponse{
			Versions: []int64{},
		}, nil
	}

	s.log.Debug("Listed artifact versions",
		logger.StringField("app", req.AppName),
		logger.StringField("user", req.UserID),
		logger.StringField("session", req.SessionID),
		logger.StringField("file", req.FileName),
		logger.IntField("count", len(metadata.Versions)))

	return &artifact.VersionsResponse{
		Versions: metadata.Versions,
	}, nil
}

// Helper methods

// getSessionPath returns the path for a session's artifacts.
func (s *ArtifactService) getSessionPath(appName, userID, sessionID string) string {
	return path.Join(appName, userID, sessionID)
}

// getArtifactBasePath returns the base path for an artifact.
func (s *ArtifactService) getArtifactBasePath(appName, userID, sessionID, fileName string) string {
	return path.Join(appName, userID, sessionID, fileName)
}

// getVersionPath returns the path for a specific version file.
func (s *ArtifactService) getVersionPath(basePath string, version int64) string {
	return path.Join(basePath, "versions", fmt.Sprintf("%d.json", version))
}

// loadMetadata loads artifact metadata from storage.
func (s *ArtifactService) loadMetadata(ctx context.Context, metadataPath string) (*ArtifactMetadata, error) {
	data, err := s.fileProvider.Read(ctx, metadataPath)
	if err != nil {
		return nil, err
	}

	var metadata ArtifactMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// saveMetadata saves artifact metadata to storage.
func (s *ArtifactService) saveMetadata(ctx context.Context, metadataPath string, metadata *ArtifactMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := s.fileProvider.Write(ctx, metadataPath, data); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}
