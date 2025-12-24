package session

import (
	"context"
	"time"
)

// Status represents the current state of a session
type Status string

const (
	StatusActive  Status = "active"
	StatusPaused  Status = "paused"
	StatusExpired Status = "expired"
)

// Session represents a user session
type Session struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	UserID    string    `json:"user_id" gorm:"index;size:64"`
	SandboxID string    `json:"sandbox_id,omitempty" gorm:"size:36"`
	Status    Status    `json:"status" gorm:"size:20"`

	// Workspace info
	WorkspaceURL string `json:"workspace_url,omitempty" gorm:"size:255"`

	// Resource configuration
	Image    string `json:"image" gorm:"size:255"`
	CPUCount int    `json:"cpu_count"`
	MemoryMB int64  `json:"memory_mb"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	PausedAt    *time.Time `json:"paused_at,omitempty"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty" gorm:"-"`
}

// TableName specifies the table name for GORM
func (Session) TableName() string {
	return "sessions"
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsActive checks if the session is active
func (s *Session) IsActive() bool {
	return s.Status == StatusActive && !s.IsExpired()
}

// CreateSessionRequest represents a request to create a new session
type CreateSessionRequest struct {
	UserID   string            `json:"user_id"`
	Image    string            `json:"image,omitempty"`
	CPUCount int               `json:"cpu_count,omitempty"`
	MemoryMB int64             `json:"memory_mb,omitempty"`
	TTL      time.Duration     `json:"ttl,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Manager defines the interface for session management
type Manager interface {
	// Create creates a new session
	Create(ctx context.Context, req CreateSessionRequest) (*Session, error)

	// Get retrieves a session by ID
	Get(ctx context.Context, id string) (*Session, error)

	// GetByUser retrieves all sessions for a user
	GetByUser(ctx context.Context, userID string) ([]*Session, error)

	// Update updates a session
	Update(ctx context.Context, session *Session) error

	// Delete deletes a session
	Delete(ctx context.Context, id string) error

	// Pause pauses a session (saves workspace, releases sandbox)
	Pause(ctx context.Context, id string) error

	// Resume resumes a paused session
	Resume(ctx context.Context, id string) (*Session, error)

	// Touch updates the last active time
	Touch(ctx context.Context, id string) error

	// Cleanup removes expired sessions
	Cleanup(ctx context.Context) (int, error)

	// BindSandbox binds a sandbox to a session
	BindSandbox(ctx context.Context, sessionID, sandboxID string) error

	// UnbindSandbox unbinds a sandbox from a session
	UnbindSandbox(ctx context.Context, sessionID string) error
}

// Store defines the interface for session persistence
type Store interface {
	// Create stores a new session
	Create(ctx context.Context, session *Session) error

	// Get retrieves a session by ID
	Get(ctx context.Context, id string) (*Session, error)

	// GetByUser retrieves sessions by user ID
	GetByUser(ctx context.Context, userID string) ([]*Session, error)

	// Update updates a session
	Update(ctx context.Context, session *Session) error

	// Delete deletes a session
	Delete(ctx context.Context, id string) error

	// ListExpired lists expired sessions
	ListExpired(ctx context.Context) ([]*Session, error)

	// DeleteExpired deletes expired sessions
	DeleteExpired(ctx context.Context) (int, error)
}

// Cache defines the interface for session caching
type Cache interface {
	// Get retrieves a session from cache
	Get(ctx context.Context, id string) (*Session, error)

	// Set stores a session in cache
	Set(ctx context.Context, session *Session, ttl time.Duration) error

	// Delete removes a session from cache
	Delete(ctx context.Context, id string) error

	// Touch updates the TTL of a cached session
	Touch(ctx context.Context, id string, ttl time.Duration) error
}

// WorkspaceStorage defines the interface for workspace persistence
type WorkspaceStorage interface {
	// Save saves the workspace from a sandbox
	Save(ctx context.Context, sessionID, sandboxID string) (string, error)

	// Restore restores the workspace to a sandbox
	Restore(ctx context.Context, sessionID, sandboxID string) error

	// Delete deletes the saved workspace
	Delete(ctx context.Context, sessionID string) error

	// Exists checks if a workspace exists
	Exists(ctx context.Context, sessionID string) (bool, error)
}
