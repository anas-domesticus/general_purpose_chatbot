package artifact_service

import (
	"context"
	"testing"

	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/artifact"
	"google.golang.org/genai"
)

// Test helper functions
func testLogger() logger.Logger {
	return logger.NewLogger(logger.Config{
		Level:  logger.ErrorLevel,
		Format: "text",
	})
}

func emptyArtifactService(t *testing.T) artifact.Service {
	t.Helper()
	tmpDir := t.TempDir()
	provider := storage_manager.NewLocalFileProvider(tmpDir)
	return NewArtifactService(provider, testLogger())
}

func TestArtifactService_SaveAndLoad(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save an artifact
	part := genai.NewPartFromText("Hello, World!")
	saveResp, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "greeting.txt",
		Part:      part,
	})
	require.NoError(t, err)
	require.NotNil(t, saveResp)
	assert.Equal(t, int64(1), saveResp.Version)

	// Load the artifact
	loadResp, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "greeting.txt",
	})
	require.NoError(t, err)
	require.NotNil(t, loadResp)
	require.NotNil(t, loadResp.Part)
	assert.Equal(t, "Hello, World!", loadResp.Part.Text)
}

func TestArtifactService_SaveMultipleVersions(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save version 1
	part1 := genai.NewPartFromText("Version 1")
	saveResp1, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Part:      part1,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), saveResp1.Version)

	// Save version 2
	part2 := genai.NewPartFromText("Version 2")
	saveResp2, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Part:      part2,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), saveResp2.Version)

	// Save version 3
	part3 := genai.NewPartFromText("Version 3")
	saveResp3, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Part:      part3,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), saveResp3.Version)

	// Load latest (should be version 3)
	loadResp, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
	})
	require.NoError(t, err)
	assert.Equal(t, "Version 3", loadResp.Part.Text)
}

func TestArtifactService_LoadSpecificVersion(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save multiple versions
	for i := 1; i <= 3; i++ {
		part := genai.NewPartFromText("Version " + string(rune('0'+i)))
		_, err := service.Save(ctx, &artifact.SaveRequest{
			AppName:   "test-app",
			UserID:    "user1",
			SessionID: "session1",
			FileName:  "doc.txt",
			Part:      part,
		})
		require.NoError(t, err)
	}

	// Load specific version (version 2)
	loadResp, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   2,
	})
	require.NoError(t, err)
	assert.Equal(t, "Version 2", loadResp.Part.Text)
}

func TestArtifactService_SaveWithSpecificVersion(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save with explicit version 5
	part := genai.NewPartFromText("Explicit Version 5")
	saveResp, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Part:      part,
		Version:   5,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), saveResp.Version)

	// Load version 5
	loadResp, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   5,
	})
	require.NoError(t, err)
	assert.Equal(t, "Explicit Version 5", loadResp.Part.Text)
}

func TestArtifactService_LoadNotFound(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	_, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "nonexistent.txt",
	})
	assert.Error(t, err)
}

func TestArtifactService_LoadVersionNotFound(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save version 1
	part := genai.NewPartFromText("Version 1")
	_, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Part:      part,
	})
	require.NoError(t, err)

	// Try to load non-existent version 99
	_, err = service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   99,
	})
	assert.Error(t, err)
}

func TestArtifactService_DeleteAllVersions(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save multiple versions
	for i := 1; i <= 3; i++ {
		part := genai.NewPartFromText("Version " + string(rune('0'+i)))
		_, err := service.Save(ctx, &artifact.SaveRequest{
			AppName:   "test-app",
			UserID:    "user1",
			SessionID: "session1",
			FileName:  "doc.txt",
			Part:      part,
		})
		require.NoError(t, err)
	}

	// Delete all versions (version = 0)
	err := service.Delete(ctx, &artifact.DeleteRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
	})
	require.NoError(t, err)

	// Verify artifact is gone
	_, err = service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
	})
	assert.Error(t, err)
}

func TestArtifactService_DeleteSpecificVersion(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save 3 versions
	for i := 1; i <= 3; i++ {
		part := genai.NewPartFromText("Version " + string(rune('0'+i)))
		_, err := service.Save(ctx, &artifact.SaveRequest{
			AppName:   "test-app",
			UserID:    "user1",
			SessionID: "session1",
			FileName:  "doc.txt",
			Part:      part,
		})
		require.NoError(t, err)
	}

	// Delete version 2 only
	err := service.Delete(ctx, &artifact.DeleteRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   2,
	})
	require.NoError(t, err)

	// Verify version 2 is gone
	_, err = service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   2,
	})
	assert.Error(t, err)

	// Verify versions 1 and 3 still exist
	loadResp1, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   1,
	})
	require.NoError(t, err)
	assert.Equal(t, "Version 1", loadResp1.Part.Text)

	loadResp3, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   3,
	})
	require.NoError(t, err)
	assert.Equal(t, "Version 3", loadResp3.Part.Text)
}

func TestArtifactService_DeleteLatestVersion(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save 3 versions
	for i := 1; i <= 3; i++ {
		part := genai.NewPartFromText("Version " + string(rune('0'+i)))
		_, err := service.Save(ctx, &artifact.SaveRequest{
			AppName:   "test-app",
			UserID:    "user1",
			SessionID: "session1",
			FileName:  "doc.txt",
			Part:      part,
		})
		require.NoError(t, err)
	}

	// Delete version 3 (latest)
	err := service.Delete(ctx, &artifact.DeleteRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
		Version:   3,
	})
	require.NoError(t, err)

	// Load latest (should now be version 2)
	loadResp, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
	})
	require.NoError(t, err)
	assert.Equal(t, "Version 2", loadResp.Part.Text)
}

func TestArtifactService_DeleteNonExistent(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Delete non-existent artifact should not error
	err := service.Delete(ctx, &artifact.DeleteRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "nonexistent.txt",
	})
	assert.NoError(t, err)
}

func TestArtifactService_List(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save multiple artifacts in the same session
	artifacts := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, fileName := range artifacts {
		part := genai.NewPartFromText("Content of " + fileName)
		_, err := service.Save(ctx, &artifact.SaveRequest{
			AppName:   "test-app",
			UserID:    "user1",
			SessionID: "session1",
			FileName:  fileName,
			Part:      part,
		})
		require.NoError(t, err)
	}

	// List artifacts
	listResp, err := service.List(ctx, &artifact.ListRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
	})
	require.NoError(t, err)
	assert.Len(t, listResp.FileNames, 3)
	assert.Contains(t, listResp.FileNames, "file1.txt")
	assert.Contains(t, listResp.FileNames, "file2.txt")
	assert.Contains(t, listResp.FileNames, "file3.txt")
}

func TestArtifactService_ListEmpty(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// List artifacts in empty session
	listResp, err := service.List(ctx, &artifact.ListRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "empty-session",
	})
	require.NoError(t, err)
	assert.Empty(t, listResp.FileNames)
}

func TestArtifactService_Versions(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save multiple versions
	for i := 1; i <= 5; i++ {
		part := genai.NewPartFromText("Version " + string(rune('0'+i)))
		_, err := service.Save(ctx, &artifact.SaveRequest{
			AppName:   "test-app",
			UserID:    "user1",
			SessionID: "session1",
			FileName:  "doc.txt",
			Part:      part,
		})
		require.NoError(t, err)
	}

	// Get versions
	versionsResp, err := service.Versions(ctx, &artifact.VersionsRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "doc.txt",
	})
	require.NoError(t, err)
	assert.Len(t, versionsResp.Versions, 5)
	assert.Equal(t, []int64{1, 2, 3, 4, 5}, versionsResp.Versions)
}

func TestArtifactService_VersionsEmpty(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Get versions for non-existent artifact
	versionsResp, err := service.Versions(ctx, &artifact.VersionsRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "nonexistent.txt",
	})
	require.NoError(t, err)
	assert.Empty(t, versionsResp.Versions)
}

func TestArtifactService_BlobContent(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save a blob artifact
	blobData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF}
	part := genai.NewPartFromBytes(blobData, "application/octet-stream")
	_, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "binary.bin",
		Part:      part,
	})
	require.NoError(t, err)

	// Load and verify
	loadResp, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "binary.bin",
	})
	require.NoError(t, err)
	require.NotNil(t, loadResp.Part)
	assert.Equal(t, blobData, loadResp.Part.InlineData.Data)
	assert.Equal(t, "application/octet-stream", loadResp.Part.InlineData.MIMEType)
}

func TestArtifactService_IsolationBetweenSessions(t *testing.T) {
	service := emptyArtifactService(t)
	ctx := context.Background()

	// Save artifact in session1
	part1 := genai.NewPartFromText("Session 1 content")
	_, err := service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "shared.txt",
		Part:      part1,
	})
	require.NoError(t, err)

	// Save artifact in session2
	part2 := genai.NewPartFromText("Session 2 content")
	_, err = service.Save(ctx, &artifact.SaveRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session2",
		FileName:  "shared.txt",
		Part:      part2,
	})
	require.NoError(t, err)

	// Verify isolation
	loadResp1, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session1",
		FileName:  "shared.txt",
	})
	require.NoError(t, err)
	assert.Equal(t, "Session 1 content", loadResp1.Part.Text)

	loadResp2, err := service.Load(ctx, &artifact.LoadRequest{
		AppName:   "test-app",
		UserID:    "user1",
		SessionID: "session2",
		FileName:  "shared.txt",
	})
	require.NoError(t, err)
	assert.Equal(t, "Session 2 content", loadResp2.Part.Text)
}

func TestNewArtifactService(t *testing.T) {
	tmpDir := t.TempDir()
	log := testLogger()

	// Test with local file provider
	provider := storage_manager.NewLocalFileProvider(tmpDir)
	service := NewArtifactService(provider, log)
	require.NotNil(t, service)

	// Test panic on nil provider
	assert.Panics(t, func() {
		NewArtifactService(nil, log)
	})

	// Test panic on nil logger
	assert.Panics(t, func() {
		NewArtifactService(provider, nil)
	})
}
