package session

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultManager implements the Manager interface
type DefaultManager struct {
	store            Store
	cache            Cache
	workspaceStorage WorkspaceStorage

	// Configuration
	defaultTTL      time.Duration
	maxTTL          time.Duration
	cleanupInterval time.Duration

	// Background cleanup
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// ManagerConfig holds manager configuration
type ManagerConfig struct {
	DefaultTTL      time.Duration
	MaxTTL          time.Duration
	CleanupInterval time.Duration
}

// DefaultManagerConfig returns default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		DefaultTTL:      24 * time.Hour,
		MaxTTL:          7 * 24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}
}

// NewManager creates a new session manager
func NewManager(store Store, cache Cache, workspaceStorage WorkspaceStorage, config ManagerConfig) *DefaultManager {
	m := &DefaultManager{
		store:            store,
		cache:            cache,
		workspaceStorage: workspaceStorage,
		defaultTTL:       config.DefaultTTL,
		maxTTL:           config.MaxTTL,
		cleanupInterval:  config.CleanupInterval,
		stopCh:           make(chan struct{}),
	}

	// Start background cleanup
	m.wg.Add(1)
	go m.cleanupLoop()

	return m
}

// Create creates a new session
func (m *DefaultManager) Create(ctx context.Context, req CreateSessionRequest) (*Session, error) {
	now := time.Now()

	// Determine TTL
	ttl := req.TTL
	if ttl == 0 {
		ttl = m.defaultTTL
	}
	if ttl > m.maxTTL {
		ttl = m.maxTTL
	}

	// Set defaults
	if req.Image == "" {
		req.Image = "python:3.11-slim"
	}
	if req.CPUCount == 0 {
		req.CPUCount = 2
	}
	if req.MemoryMB == 0 {
		req.MemoryMB = 2048
	}

	session := &Session{
		ID:           uuid.New().String(),
		UserID:       req.UserID,
		Status:       StatusActive,
		Image:        req.Image,
		CPUCount:     req.CPUCount,
		MemoryMB:     req.MemoryMB,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now.Add(ttl),
		Metadata:     req.Metadata,
	}

	// Store in database
	if err := m.store.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Cache the session
	if m.cache != nil {
		if err := m.cache.Set(ctx, session, ttl); err != nil {
			log.Printf("[Session] Failed to cache session %s: %v", session.ID, err)
		}
	}

	log.Printf("[Session] Created session %s for user %s", session.ID, session.UserID)
	return session, nil
}

// Get retrieves a session by ID
func (m *DefaultManager) Get(ctx context.Context, id string) (*Session, error) {
	// Try cache first
	if m.cache != nil {
		if session, err := m.cache.Get(ctx, id); err == nil && session != nil {
			return session, nil
		}
	}

	// Fallback to database
	session, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check expiration
	if session.IsExpired() {
		return nil, fmt.Errorf("session expired: %s", id)
	}

	// Update cache
	if m.cache != nil {
		ttl := time.Until(session.ExpiresAt)
		if ttl > 0 {
			m.cache.Set(ctx, session, ttl)
		}
	}

	return session, nil
}

// GetByUser retrieves all sessions for a user
func (m *DefaultManager) GetByUser(ctx context.Context, userID string) ([]*Session, error) {
	sessions, err := m.store.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter out expired sessions
	active := make([]*Session, 0, len(sessions))
	for _, s := range sessions {
		if !s.IsExpired() {
			active = append(active, s)
		}
	}

	return active, nil
}

// Update updates a session
func (m *DefaultManager) Update(ctx context.Context, session *Session) error {
	session.UpdatedAt = time.Now()

	if err := m.store.Update(ctx, session); err != nil {
		return err
	}

	// Update cache
	if m.cache != nil {
		ttl := time.Until(session.ExpiresAt)
		if ttl > 0 {
			m.cache.Set(ctx, session, ttl)
		}
	}

	return nil
}

// Delete deletes a session
func (m *DefaultManager) Delete(ctx context.Context, id string) error {
	// Delete workspace if exists
	if m.workspaceStorage != nil {
		if exists, _ := m.workspaceStorage.Exists(ctx, id); exists {
			if err := m.workspaceStorage.Delete(ctx, id); err != nil {
				log.Printf("[Session] Failed to delete workspace for session %s: %v", id, err)
			}
		}
	}

	// Delete from cache
	if m.cache != nil {
		m.cache.Delete(ctx, id)
	}

	// Delete from database
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}

	log.Printf("[Session] Deleted session %s", id)
	return nil
}

// Pause pauses a session (saves workspace, releases sandbox)
func (m *DefaultManager) Pause(ctx context.Context, id string) error {
	session, err := m.Get(ctx, id)
	if err != nil {
		return err
	}

	if session.Status != StatusActive {
		return fmt.Errorf("session is not active: %s", session.Status)
	}

	if session.SandboxID == "" {
		return fmt.Errorf("session has no sandbox bound")
	}

	// Save workspace
	if m.workspaceStorage != nil {
		workspaceURL, err := m.workspaceStorage.Save(ctx, id, session.SandboxID)
		if err != nil {
			return fmt.Errorf("failed to save workspace: %w", err)
		}
		session.WorkspaceURL = workspaceURL
	}

	// Update session status
	now := time.Now()
	session.Status = StatusPaused
	session.PausedAt = &now
	session.SandboxID = "" // Sandbox will be released

	if err := m.Update(ctx, session); err != nil {
		return err
	}

	log.Printf("[Session] Paused session %s", id)
	return nil
}

// Resume resumes a paused session
func (m *DefaultManager) Resume(ctx context.Context, id string) (*Session, error) {
	session, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if session.Status != StatusPaused {
		return nil, fmt.Errorf("session is not paused: %s", session.Status)
	}

	// Update session status (sandbox will be bound later)
	now := time.Now()
	session.Status = StatusActive
	session.PausedAt = nil
	session.LastActiveAt = now

	// Extend expiration
	session.ExpiresAt = now.Add(m.defaultTTL)

	if err := m.Update(ctx, session); err != nil {
		return nil, err
	}

	log.Printf("[Session] Resumed session %s", id)
	return session, nil
}

// RestoreWorkspace restores workspace to a sandbox after resume
func (m *DefaultManager) RestoreWorkspace(ctx context.Context, sessionID, sandboxID string) error {
	if m.workspaceStorage == nil {
		return nil
	}

	exists, err := m.workspaceStorage.Exists(ctx, sessionID)
	if err != nil {
		return err
	}

	if !exists {
		log.Printf("[Session] No workspace to restore for session %s", sessionID)
		return nil
	}

	if err := m.workspaceStorage.Restore(ctx, sessionID, sandboxID); err != nil {
		return fmt.Errorf("failed to restore workspace: %w", err)
	}

	log.Printf("[Session] Restored workspace for session %s to sandbox %s", sessionID, sandboxID)
	return nil
}

// Touch updates the last active time
func (m *DefaultManager) Touch(ctx context.Context, id string) error {
	session, err := m.Get(ctx, id)
	if err != nil {
		return err
	}

	session.LastActiveAt = time.Now()

	if err := m.store.Update(ctx, session); err != nil {
		return err
	}

	// Touch cache
	if m.cache != nil {
		ttl := time.Until(session.ExpiresAt)
		if ttl > 0 {
			m.cache.Touch(ctx, id, ttl)
		}
	}

	return nil
}

// Cleanup removes expired sessions
func (m *DefaultManager) Cleanup(ctx context.Context) (int, error) {
	// Get expired sessions for workspace cleanup
	expired, err := m.store.ListExpired(ctx)
	if err != nil {
		return 0, err
	}

	// Delete workspaces
	if m.workspaceStorage != nil {
		for _, s := range expired {
			if exists, _ := m.workspaceStorage.Exists(ctx, s.ID); exists {
				m.workspaceStorage.Delete(ctx, s.ID)
			}
		}
	}

	// Delete expired sessions
	count, err := m.store.DeleteExpired(ctx)
	if err != nil {
		return 0, err
	}

	if count > 0 {
		log.Printf("[Session] Cleaned up %d expired sessions", count)
	}

	return count, nil
}

// BindSandbox binds a sandbox to a session
func (m *DefaultManager) BindSandbox(ctx context.Context, sessionID, sandboxID string) error {
	session, err := m.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	session.SandboxID = sandboxID
	session.LastActiveAt = time.Now()

	if err := m.Update(ctx, session); err != nil {
		return err
	}

	log.Printf("[Session] Bound sandbox %s to session %s", sandboxID, sessionID)
	return nil
}

// UnbindSandbox unbinds a sandbox from a session
func (m *DefaultManager) UnbindSandbox(ctx context.Context, sessionID string) error {
	session, err := m.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	session.SandboxID = ""

	if err := m.Update(ctx, session); err != nil {
		return err
	}

	log.Printf("[Session] Unbound sandbox from session %s", sessionID)
	return nil
}

// cleanupLoop periodically cleans up expired sessions
func (m *DefaultManager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			ctx := context.Background()
			m.Cleanup(ctx)
		}
	}
}

// Close stops the manager
func (m *DefaultManager) Close() error {
	close(m.stopCh)
	m.wg.Wait()
	return nil
}
