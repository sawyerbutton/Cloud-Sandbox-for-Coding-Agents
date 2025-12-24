package sandbox

import (
	"context"
	"io"
	"time"
)

// Status represents the current state of a sandbox
type Status string

const (
	StatusCreating Status = "creating"
	StatusIdle     Status = "idle"
	StatusActive   Status = "active"
	StatusPaused   Status = "paused"
	StatusStopped  Status = "stopped"
	StatusError    Status = "error"
)

// Sandbox represents a sandbox instance
type Sandbox struct {
	ID           string            `json:"id"`
	Status       Status            `json:"status"`
	ContainerID  string            `json:"container_id,omitempty"`
	Image        string            `json:"image"`
	IP           string            `json:"ip,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	LastActiveAt time.Time         `json:"last_active_at"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// Config holds sandbox configuration
type Config struct {
	// Container image to use
	Image string `yaml:"image"`

	// Resource limits
	CPUCount   int   `yaml:"cpu_count"`
	MemoryMB   int64 `yaml:"memory_mb"`
	DiskSizeMB int64 `yaml:"disk_size_mb"`

	// Execution limits
	MaxExecutionTime time.Duration `yaml:"max_execution_time"`
	MaxOutputSize    int64         `yaml:"max_output_size"`

	// Network settings
	NetworkEnabled bool     `yaml:"network_enabled"`
	AllowedHosts   []string `yaml:"allowed_hosts"`

	// Working directory inside container
	WorkDir string `yaml:"work_dir"`
}

// DefaultConfig returns default sandbox configuration
func DefaultConfig() Config {
	return Config{
		Image:            "python:3.11-slim",
		CPUCount:         2,
		MemoryMB:         2048,
		DiskSizeMB:       10240,
		MaxExecutionTime: 5 * time.Minute,
		MaxOutputSize:    10 * 1024 * 1024, // 10MB
		NetworkEnabled:   true,
		WorkDir:          "/workspace",
	}
}

// ExecRequest represents a code execution request
type ExecRequest struct {
	// Code to execute
	Code string `json:"code"`

	// Language/runtime (python, node, bash, etc.)
	Language string `json:"language"`

	// Command to run (alternative to Code)
	Command []string `json:"command,omitempty"`

	// Working directory
	WorkDir string `json:"work_dir,omitempty"`

	// Environment variables
	Env map[string]string `json:"env,omitempty"`

	// Timeout for this execution
	Timeout time.Duration `json:"timeout,omitempty"`

	// Stdin input
	Stdin io.Reader `json:"-"`
}

// ExecResult represents the result of code execution
type ExecResult struct {
	// Exit code of the process
	ExitCode int `json:"exit_code"`

	// Standard output
	Stdout string `json:"stdout"`

	// Standard error
	Stderr string `json:"stderr"`

	// Execution time
	Duration time.Duration `json:"duration_ms"`

	// Whether execution timed out
	TimedOut bool `json:"timed_out"`

	// Error message if execution failed
	Error string `json:"error,omitempty"`
}

// FileInfo represents file metadata
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
}

// Runtime defines the interface for sandbox runtime implementations
type Runtime interface {
	// Create creates a new sandbox
	Create(ctx context.Context, config Config) (*Sandbox, error)

	// Start starts a stopped sandbox
	Start(ctx context.Context, id string) error

	// Stop stops a running sandbox
	Stop(ctx context.Context, id string) error

	// Destroy destroys a sandbox and cleans up resources
	Destroy(ctx context.Context, id string) error

	// Get returns sandbox by ID
	Get(ctx context.Context, id string) (*Sandbox, error)

	// List returns all sandboxes
	List(ctx context.Context) ([]*Sandbox, error)

	// Exec executes code in a sandbox
	Exec(ctx context.Context, id string, req ExecRequest) (*ExecResult, error)

	// WriteFile writes content to a file in the sandbox
	WriteFile(ctx context.Context, id string, path string, content []byte) error

	// ReadFile reads a file from the sandbox
	ReadFile(ctx context.Context, id string, path string) ([]byte, error)

	// ListFiles lists files in a directory
	ListFiles(ctx context.Context, id string, path string) ([]FileInfo, error)

	// DeleteFile deletes a file or directory
	DeleteFile(ctx context.Context, id string, path string) error
}
