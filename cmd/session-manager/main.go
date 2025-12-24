package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloud-sandbox/cloud-sandbox/internal/session"
)

type Server struct {
	manager *session.DefaultManager
}

func main() {
	log.Println("Starting Cloud Sandbox Session Manager...")

	// Initialize PostgreSQL store
	pgConfig := session.DefaultPostgresConfig()
	if host := os.Getenv("POSTGRES_HOST"); host != "" {
		pgConfig.Host = host
	}

	store, err := session.NewPostgresStore(pgConfig)
	if err != nil {
		log.Printf("[Session Manager] Warning: Failed to connect to PostgreSQL: %v", err)
		log.Println("[Session Manager] Running without persistent storage (in-memory only)")
		store = nil
	}

	// Initialize Redis cache
	redisConfig := session.DefaultRedisConfig()
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		redisConfig.Addr = addr
	}

	var cache session.Cache
	redisCache, err := session.NewRedisCache(redisConfig, 24*time.Hour)
	if err != nil {
		log.Printf("[Session Manager] Warning: Failed to connect to Redis: %v", err)
		log.Println("[Session Manager] Running without cache")
		cache = nil
	} else {
		cache = redisCache
	}

	// Initialize MinIO storage
	minioConfig := session.DefaultMinIOConfig()
	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		minioConfig.Endpoint = endpoint
	}

	var workspaceStorage session.WorkspaceStorage
	minioStorage, err := session.NewMinIOStorage(minioConfig)
	if err != nil {
		log.Printf("[Session Manager] Warning: Failed to connect to MinIO: %v", err)
		log.Println("[Session Manager] Running without workspace storage")
		workspaceStorage = nil
	} else {
		workspaceStorage = minioStorage
	}

	// Use in-memory store if PostgreSQL is not available
	var sessionStore session.Store
	if store != nil {
		sessionStore = store
	} else {
		sessionStore = NewInMemoryStore()
	}

	// Create session manager
	managerConfig := session.DefaultManagerConfig()
	manager := session.NewManager(sessionStore, cache, workspaceStorage, managerConfig)

	server := &Server{
		manager: manager,
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", server.handleHealth)

	// Session management
	mux.HandleFunc("/api/v1/sessions", server.handleSessions)
	mux.HandleFunc("/api/v1/sessions/", server.handleSession)

	httpServer := &http.Server{
		Addr:         ":9091",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server
	go func() {
		log.Println("Session Manager HTTP API listening on :9091")
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Println("Session Manager is running")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Session Manager shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpServer.Shutdown(ctx)
	manager.Close()

	if store != nil {
		store.Close()
	}
	if redisCache != nil {
		redisCache.Close()
	}
	if minioStorage != nil {
		minioStorage.Close()
	}

	log.Println("Session Manager stopped")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleSessions handles /api/v1/sessions (list/create)
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// List sessions by user
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}

		sessions, err := s.manager.GetByUser(ctx, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"sessions": sessions})

	case http.MethodPost:
		// Create session
		var req session.CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.UserID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}

		sess, err := s.manager.Create(ctx, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sess)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSession handles /api/v1/sessions/{id}
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract session ID from path
	path := r.URL.Path
	if len(path) <= len("/api/v1/sessions/") {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Check for action paths like /api/v1/sessions/{id}/pause
	parts := splitPath(path[len("/api/v1/sessions/"):])
	sessionID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "pause":
		s.handlePause(ctx, w, r, sessionID)
	case "resume":
		s.handleResume(ctx, w, r, sessionID)
	case "touch":
		s.handleTouch(ctx, w, r, sessionID)
	case "bind":
		s.handleBind(ctx, w, r, sessionID)
	case "":
		s.handleSessionCRUD(ctx, w, r, sessionID)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

func (s *Server) handleSessionCRUD(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	switch r.Method {
	case http.MethodGet:
		// Get session
		sess, err := s.manager.Get(ctx, sessionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sess)

	case http.MethodDelete:
		// Delete session
		if err := s.manager.Delete(ctx, sessionID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePause(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.manager.Pause(ctx, sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sess, _ := s.manager.Get(ctx, sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess)
}

func (s *Server) handleResume(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess, err := s.manager.Resume(ctx, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess)
}

func (s *Server) handleTouch(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.manager.Touch(ctx, sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

type BindRequest struct {
	SandboxID string `json:"sandbox_id"`
}

func (s *Server) handleBind(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	switch r.Method {
	case http.MethodPost:
		// Bind sandbox
		var req BindRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.manager.BindSandbox(ctx, sessionID, req.SandboxID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodDelete:
		// Unbind sandbox
		if err := s.manager.UnbindSandbox(ctx, sessionID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func splitPath(path string) []string {
	var parts []string
	for _, p := range split(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func split(s string, sep rune) []string {
	var parts []string
	current := ""
	for _, r := range s {
		if r == sep {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// InMemoryStore is a simple in-memory implementation of Store for development
type InMemoryStore struct {
	sessions map[string]*session.Session
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sessions: make(map[string]*session.Session),
	}
}

func (s *InMemoryStore) Create(ctx context.Context, sess *session.Session) error {
	s.sessions[sess.ID] = sess
	return nil
}

func (s *InMemoryStore) Get(ctx context.Context, id string) (*session.Session, error) {
	sess, ok := s.sessions[id]
	if !ok {
		return nil, nil
	}
	return sess, nil
}

func (s *InMemoryStore) GetByUser(ctx context.Context, userID string) ([]*session.Session, error) {
	var result []*session.Session
	for _, sess := range s.sessions {
		if sess.UserID == userID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *InMemoryStore) Update(ctx context.Context, sess *session.Session) error {
	s.sessions[sess.ID] = sess
	return nil
}

func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	delete(s.sessions, id)
	return nil
}

func (s *InMemoryStore) ListExpired(ctx context.Context) ([]*session.Session, error) {
	var result []*session.Session
	now := time.Now()
	for _, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *InMemoryStore) DeleteExpired(ctx context.Context) (int, error) {
	now := time.Now()
	count := 0
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, id)
			count++
		}
	}
	return count, nil
}
