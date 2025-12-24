package sandbox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	if os.Getenv("SKIP_DOCKER_TESTS") == "1" {
		t.Skip("Skipping Docker tests")
	}
}

func TestDockerRuntime_Create(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create sandbox
	sb, err := runtime.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	if sb.ID == "" {
		t.Error("Sandbox ID should not be empty")
	}

	if sb.ContainerID == "" {
		t.Error("Container ID should not be empty")
	}

	if sb.Status != StatusIdle {
		t.Errorf("Expected status %s, got %s", StatusIdle, sb.Status)
	}

	// Cleanup
	if err := runtime.Destroy(ctx, sb.ID); err != nil {
		t.Errorf("Failed to destroy sandbox: %v", err)
	}
}

func TestDockerRuntime_Exec(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create sandbox
	sb, err := runtime.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sb.ID)

	// Execute simple command
	result, err := runtime.Exec(ctx, sb.ID, ExecRequest{
		Command: []string{"echo", "hello world"},
	})
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if strings.TrimSpace(result.Stdout) != "hello world" {
		t.Errorf("Expected 'hello world', got %q", result.Stdout)
	}
}

func TestDockerRuntime_ExecPython(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "python:3.11-slim"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create sandbox
	sb, err := runtime.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sb.ID)

	// Execute Python code
	result, err := runtime.Exec(ctx, sb.ID, ExecRequest{
		Language: "python",
		Code:     "print(sum(range(10)))",
	})
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d. Stderr: %s", result.ExitCode, result.Stderr)
	}

	if strings.TrimSpace(result.Stdout) != "45" {
		t.Errorf("Expected '45', got %q", result.Stdout)
	}
}

func TestDockerRuntime_ExecTimeout(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create sandbox
	sb, err := runtime.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sb.ID)

	// Execute command that times out
	result, err := runtime.Exec(ctx, sb.ID, ExecRequest{
		Command: []string{"sleep", "10"},
		Timeout: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if !result.TimedOut {
		t.Error("Expected execution to time out")
	}
}

func TestDockerRuntime_FileOperations(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create sandbox
	sb, err := runtime.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sb.ID)

	// Write file
	testContent := []byte("Hello, Sandbox!")
	testPath := "/workspace/test.txt"

	if err := runtime.WriteFile(ctx, sb.ID, testPath, testContent); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Read file
	content, err := runtime.ReadFile(ctx, sb.ID, testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Content mismatch: expected %q, got %q", testContent, content)
	}

	// List files
	files, err := runtime.ListFiles(ctx, sb.ID, "/workspace")
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Name == "test.txt" {
			found = true
			break
		}
	}

	if !found {
		t.Error("test.txt not found in file listing")
	}

	// Delete file
	if err := runtime.DeleteFile(ctx, sb.ID, testPath); err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Verify deletion
	_, err = runtime.ReadFile(ctx, sb.ID, testPath)
	if err == nil {
		t.Error("Expected error reading deleted file")
	}
}

func TestDockerRuntime_List(t *testing.T) {
	skipIfNoDocker(t)

	config := DefaultConfig()
	config.Image = "alpine:latest"

	runtime, err := NewDockerRuntime(config)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create multiple sandboxes
	sb1, err := runtime.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox 1: %v", err)
	}
	defer runtime.Destroy(ctx, sb1.ID)

	sb2, err := runtime.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox 2: %v", err)
	}
	defer runtime.Destroy(ctx, sb2.ID)

	// List sandboxes
	sandboxes, err := runtime.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list sandboxes: %v", err)
	}

	if len(sandboxes) < 2 {
		t.Errorf("Expected at least 2 sandboxes, got %d", len(sandboxes))
	}
}
