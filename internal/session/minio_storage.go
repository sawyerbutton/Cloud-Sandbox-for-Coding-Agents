package session

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStorage implements WorkspaceStorage using MinIO
type MinIOStorage struct {
	client       *minio.Client
	dockerClient *client.Client
	bucket       string
	workDir      string
}

// MinIOConfig holds MinIO connection configuration
type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	WorkDir   string // Workspace directory inside container
}

// DefaultMinIOConfig returns default MinIO configuration
func DefaultMinIOConfig() MinIOConfig {
	return MinIOConfig{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "sandbox-workspaces",
		UseSSL:    false,
		WorkDir:   "/workspace",
	}
}

// NewMinIOStorage creates a new MinIO workspace storage
func NewMinIOStorage(config MinIOConfig) (*MinIOStorage, error) {
	// Create MinIO client
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// Create Docker client for accessing container filesystem
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	storage := &MinIOStorage{
		client:       minioClient,
		dockerClient: dockerClient,
		bucket:       config.Bucket,
		workDir:      config.WorkDir,
	}

	// Ensure bucket exists
	ctx := context.Background()
	if err := storage.ensureBucket(ctx); err != nil {
		return nil, err
	}

	return storage, nil
}

// ensureBucket creates the bucket if it doesn't exist
func (s *MinIOStorage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}

	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		log.Printf("[MinIO] Created bucket: %s", s.bucket)
	}

	return nil
}

// workspaceKey generates the object key for a session's workspace
func workspaceKey(sessionID string) string {
	return fmt.Sprintf("workspaces/%s/workspace.tar.gz", sessionID)
}

// Save saves the workspace from a sandbox container to MinIO
func (s *MinIOStorage) Save(ctx context.Context, sessionID, sandboxID string) (string, error) {
	// Get container ID from sandbox ID (assuming container name format: sandbox-<sandboxID>)
	containerName := "sandbox-" + sandboxID

	// Copy from container
	reader, _, err := s.dockerClient.CopyFromContainer(ctx, containerName, s.workDir)
	if err != nil {
		return "", fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	// Compress the tar archive
	var compressed bytes.Buffer
	gzWriter := gzip.NewWriter(&compressed)

	if _, err := io.Copy(gzWriter, reader); err != nil {
		return "", fmt.Errorf("failed to compress workspace: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Upload to MinIO
	objectKey := workspaceKey(sessionID)
	_, err = s.client.PutObject(ctx, s.bucket, objectKey, &compressed, int64(compressed.Len()),
		minio.PutObjectOptions{
			ContentType: "application/gzip",
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload workspace: %w", err)
	}

	log.Printf("[MinIO] Saved workspace for session %s (%d bytes)", sessionID, compressed.Len())
	return objectKey, nil
}

// Restore restores the workspace from MinIO to a sandbox container
func (s *MinIOStorage) Restore(ctx context.Context, sessionID, sandboxID string) error {
	containerName := "sandbox-" + sandboxID
	objectKey := workspaceKey(sessionID)

	// Download from MinIO
	object, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get workspace object: %w", err)
	}
	defer object.Close()

	// Decompress
	gzReader, err := gzip.NewReader(object)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// The content from docker is already a tar, just need to extract and re-tar
	// First, read all content
	var content bytes.Buffer
	if _, err := io.Copy(&content, gzReader); err != nil {
		return fmt.Errorf("failed to decompress workspace: %w", err)
	}

	// Copy to container
	if err := s.dockerClient.CopyToContainer(ctx, containerName, "/", &content, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy to container: %w", err)
	}

	log.Printf("[MinIO] Restored workspace for session %s to sandbox %s", sessionID, sandboxID)
	return nil
}

// Delete deletes the saved workspace for a session
func (s *MinIOStorage) Delete(ctx context.Context, sessionID string) error {
	objectKey := workspaceKey(sessionID)

	if err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	log.Printf("[MinIO] Deleted workspace for session %s", sessionID)
	return nil
}

// Exists checks if a workspace exists for a session
func (s *MinIOStorage) Exists(ctx context.Context, sessionID string) (bool, error) {
	objectKey := workspaceKey(sessionID)

	_, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		// Check if it's a "not found" error
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check workspace: %w", err)
	}

	return true, nil
}

// Close closes the storage clients
func (s *MinIOStorage) Close() error {
	return s.dockerClient.Close()
}

// Helper function to create tar from directory content
func createTarFromDir(dirPath string, files map[string][]byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for path, content := range files {
		hdr := &tar.Header{
			Name: filepath.Join(dirPath, path),
			Mode: 0644,
			Size: int64(len(content)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}

		if _, err := tw.Write(content); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

// Helper function to extract tar content
func extractTar(reader io.Reader) (map[string][]byte, error) {
	files := make(map[string][]byte)
	tr := tar.NewReader(reader)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Skip directories
		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		// Read file content
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}

		// Clean the path
		cleanPath := strings.TrimPrefix(hdr.Name, "/")
		files[cleanPath] = content
	}

	return files, nil
}
