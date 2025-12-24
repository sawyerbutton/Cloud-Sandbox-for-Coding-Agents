package sandbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
)

const (
	labelPrefix    = "cloud-sandbox"
	labelSandboxID = labelPrefix + ".sandbox-id"
	labelManaged   = labelPrefix + ".managed"
)

// DockerRuntime implements Runtime interface using Docker
type DockerRuntime struct {
	client *client.Client
	config Config

	mu        sync.RWMutex
	sandboxes map[string]*Sandbox
}

// NewDockerRuntime creates a new Docker-based sandbox runtime
func NewDockerRuntime(config Config) (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to docker: %w", err)
	}

	return &DockerRuntime{
		client:    cli,
		config:    config,
		sandboxes: make(map[string]*Sandbox),
	}, nil
}

// Create creates a new sandbox container
func (r *DockerRuntime) Create(ctx context.Context, config Config) (*Sandbox, error) {
	// Merge with default config
	if config.Image == "" {
		config.Image = r.config.Image
	}
	if config.CPUCount == 0 {
		config.CPUCount = r.config.CPUCount
	}
	if config.MemoryMB == 0 {
		config.MemoryMB = r.config.MemoryMB
	}
	if config.WorkDir == "" {
		config.WorkDir = r.config.WorkDir
	}

	sandboxID := generateSandboxID()

	// Ensure image exists
	if err := r.ensureImage(ctx, config.Image); err != nil {
		return nil, fmt.Errorf("failed to ensure image: %w", err)
	}

	// Create container config
	containerConfig := &container.Config{
		Image:      config.Image,
		WorkingDir: config.WorkDir,
		Tty:        false,
		OpenStdin:  true,
		Labels: map[string]string{
			labelSandboxID: sandboxID,
			labelManaged:   "true",
		},
		// Keep container running
		Cmd: []string{"sleep", "infinity"},
	}

	// Host config with resource limits
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   config.MemoryMB * 1024 * 1024,
			NanoCPUs: int64(config.CPUCount) * 1e9,
		},
		// Security options
		SecurityOpt: []string{"no-new-privileges"},
		// Mount workspace as tmpfs for better isolation
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeTmpfs,
				Target: config.WorkDir,
				TmpfsOptions: &mount.TmpfsOptions{
					SizeBytes: config.DiskSizeMB * 1024 * 1024,
				},
			},
		},
		// Network mode
		NetworkMode: container.NetworkMode("bridge"),
	}

	// Disable network if requested
	if !config.NetworkEnabled {
		hostConfig.NetworkMode = "none"
	}

	// Create container
	resp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "sandbox-"+sandboxID)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := r.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up on failure
		r.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get container info for IP
	info, err := r.client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		r.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	ip := ""
	if info.NetworkSettings != nil && info.NetworkSettings.Networks != nil {
		for _, network := range info.NetworkSettings.Networks {
			ip = network.IPAddress
			break
		}
	}

	sandbox := &Sandbox{
		ID:           sandboxID,
		Status:       StatusIdle,
		ContainerID:  resp.ID,
		Image:        config.Image,
		IP:           ip,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		Labels:       containerConfig.Labels,
	}

	r.mu.Lock()
	r.sandboxes[sandboxID] = sandbox
	r.mu.Unlock()

	return sandbox, nil
}

// Start starts a stopped sandbox
func (r *DockerRuntime) Start(ctx context.Context, id string) error {
	sandbox, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := r.client.ContainerStart(ctx, sandbox.ContainerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	r.mu.Lock()
	sandbox.Status = StatusIdle
	sandbox.LastActiveAt = time.Now()
	r.mu.Unlock()

	return nil
}

// Stop stops a running sandbox
func (r *DockerRuntime) Stop(ctx context.Context, id string) error {
	sandbox, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	timeout := 10
	if err := r.client.ContainerStop(ctx, sandbox.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	r.mu.Lock()
	sandbox.Status = StatusStopped
	r.mu.Unlock()

	return nil
}

// Destroy destroys a sandbox and cleans up resources
func (r *DockerRuntime) Destroy(ctx context.Context, id string) error {
	sandbox, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := r.client.ContainerRemove(ctx, sandbox.ContainerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	r.mu.Lock()
	delete(r.sandboxes, id)
	r.mu.Unlock()

	return nil
}

// Get returns sandbox by ID
func (r *DockerRuntime) Get(ctx context.Context, id string) (*Sandbox, error) {
	r.mu.RLock()
	sandbox, ok := r.sandboxes[id]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("sandbox not found: %s", id)
	}

	return sandbox, nil
}

// List returns all sandboxes
func (r *DockerRuntime) List(ctx context.Context) ([]*Sandbox, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Sandbox, 0, len(r.sandboxes))
	for _, sb := range r.sandboxes {
		result = append(result, sb)
	}

	return result, nil
}

// Exec executes code in a sandbox
func (r *DockerRuntime) Exec(ctx context.Context, id string, req ExecRequest) (*ExecResult, error) {
	sandbox, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update status
	r.mu.Lock()
	sandbox.Status = StatusActive
	sandbox.LastActiveAt = time.Now()
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		sandbox.Status = StatusIdle
		r.mu.Unlock()
	}()

	// Build command based on language
	var cmd []string
	if len(req.Command) > 0 {
		cmd = req.Command
	} else {
		cmd = r.buildCommand(req.Language, req.Code)
	}

	// Set timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = r.config.MaxExecutionTime
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create exec instance
	workDir := req.WorkDir
	if workDir == "" {
		workDir = r.config.WorkDir
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		WorkingDir:   workDir,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  req.Stdin != nil,
	}

	// Add environment variables
	if len(req.Env) > 0 {
		env := make([]string, 0, len(req.Env))
		for k, v := range req.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		execConfig.Env = env
	}

	execResp, err := r.client.ContainerExecCreate(execCtx, sandbox.ContainerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec
	attachResp, err := r.client.ContainerExecAttach(execCtx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// Capture output
	startTime := time.Now()
	var stdout, stderr bytes.Buffer

	// Copy stdin if provided
	if req.Stdin != nil {
		go func() {
			io.Copy(attachResp.Conn, req.Stdin)
			attachResp.CloseWrite()
		}()
	}

	// Read output with size limit
	outputDone := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(
			&limitedWriter{w: &stdout, limit: r.config.MaxOutputSize},
			&limitedWriter{w: &stderr, limit: r.config.MaxOutputSize},
			attachResp.Reader,
		)
		outputDone <- err
	}()

	// Wait for completion or timeout
	var timedOut bool
	select {
	case <-execCtx.Done():
		timedOut = true
	case <-outputDone:
	}

	duration := time.Since(startTime)

	// Get exit code
	inspectResp, err := r.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec: %w", err)
	}

	return &ExecResult{
		ExitCode: inspectResp.ExitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		TimedOut: timedOut,
	}, nil
}

// WriteFile writes content to a file in the sandbox
func (r *DockerRuntime) WriteFile(ctx context.Context, id string, path string, content []byte) error {
	dir := filepath.Dir(path)

	// First create parent directories
	result, err := r.Exec(ctx, id, ExecRequest{
		Command: []string{"mkdir", "-p", dir},
	})
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to create directory: %s", result.Stderr)
	}

	// Use base64 encoding to handle binary content safely
	encoded := base64.StdEncoding.EncodeToString(content)

	// Write file using sh to handle the base64 decoding
	result, err = r.Exec(ctx, id, ExecRequest{
		Command: []string{"sh", "-c", fmt.Sprintf("echo '%s' | base64 -d > '%s'", encoded, path)},
	})
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to write file: %s", result.Stderr)
	}

	return nil
}

// ReadFile reads a file from the sandbox
func (r *DockerRuntime) ReadFile(ctx context.Context, id string, path string) ([]byte, error) {
	// First check if file exists
	checkResult, err := r.Exec(ctx, id, ExecRequest{
		Command: []string{"test", "-f", path},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check file: %w", err)
	}
	if checkResult.ExitCode != 0 {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// Use base64 encoding to handle binary content safely
	result, err := r.Exec(ctx, id, ExecRequest{
		Command: []string{"sh", "-c", fmt.Sprintf("cat '%s' | base64", path)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to read file: %s", result.Stderr)
	}

	// Decode base64 content
	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Stdout))
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
}

// ListFiles lists files in a directory
func (r *DockerRuntime) ListFiles(ctx context.Context, id string, path string) ([]FileInfo, error) {
	// Use basic ls -la which works with both GNU coreutils and BusyBox
	result, err := r.Exec(ctx, id, ExecRequest{
		Command: []string{"ls", "-la", path},
	})
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to list files: %s", result.Stderr)
	}

	return parseLsOutput(result.Stdout, path), nil
}

// DeleteFile deletes a file or directory
func (r *DockerRuntime) DeleteFile(ctx context.Context, id string, path string) error {
	result, err := r.Exec(ctx, id, ExecRequest{
		Command: []string{"rm", "-rf", path},
	})
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to delete file: %s", result.Stderr)
	}

	return nil
}

// ensureImage pulls the image if not present
func (r *DockerRuntime) ensureImage(ctx context.Context, imageName string) error {
	// Check if image exists
	_, _, err := r.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		return nil
	}

	// Pull image
	out, err := r.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	// Wait for pull to complete
	io.Copy(io.Discard, out)
	return nil
}

// buildCommand builds the execution command based on language
func (r *DockerRuntime) buildCommand(language, code string) []string {
	switch strings.ToLower(language) {
	case "python", "python3":
		return []string{"python3", "-c", code}
	case "node", "javascript", "js":
		return []string{"node", "-e", code}
	case "bash", "sh", "shell":
		return []string{"bash", "-c", code}
	case "ruby":
		return []string{"ruby", "-e", code}
	case "go", "golang":
		// For Go, we need to write to a file and run
		return []string{"bash", "-c", fmt.Sprintf("echo '%s' > /tmp/main.go && go run /tmp/main.go", escapeShell(code))}
	default:
		// Default to bash
		return []string{"bash", "-c", code}
	}
}

// SyncFromDocker syncs sandbox state from Docker
func (r *DockerRuntime) SyncFromDocker(ctx context.Context) error {
	// List all managed containers
	containers, err := r.client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", labelManaged+"=true"),
		),
	})
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Update sandbox map
	for _, c := range containers {
		sandboxID := c.Labels[labelSandboxID]
		if sandboxID == "" {
			continue
		}

		status := StatusIdle
		switch c.State {
		case "running":
			status = StatusIdle
		case "paused":
			status = StatusPaused
		case "exited", "dead":
			status = StatusStopped
		}

		if _, exists := r.sandboxes[sandboxID]; !exists {
			r.sandboxes[sandboxID] = &Sandbox{
				ID:           sandboxID,
				Status:       status,
				ContainerID:  c.ID,
				Image:        c.Image,
				CreatedAt:    time.Unix(c.Created, 0),
				LastActiveAt: time.Now(),
				Labels:       c.Labels,
			}
		} else {
			r.sandboxes[sandboxID].Status = status
		}
	}

	return nil
}

// Close closes the Docker client
func (r *DockerRuntime) Close() error {
	return r.client.Close()
}

// Helper functions

func generateSandboxID() string {
	return uuid.New().String()[:8]
}

func escapeShell(s string) string {
	return strings.ReplaceAll(s, "'", "'\"'\"'")
}

// limitedWriter limits the amount of data written
type limitedWriter struct {
	w       io.Writer
	limit   int64
	written int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written >= lw.limit {
		return len(p), nil // Silently discard
	}

	remaining := lw.limit - lw.written
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}

	n, err := lw.w.Write(p)
	lw.written += int64(n)
	return n, err
}
