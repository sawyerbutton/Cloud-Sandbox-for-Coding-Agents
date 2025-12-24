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

	"github.com/cloud-sandbox/cloud-sandbox/internal/sandbox"
)

type Server struct {
	pool    *sandbox.Pool
	runtime *sandbox.DockerRuntime
}

func main() {
	log.Println("Starting Cloud Sandbox Scheduler...")

	// Initialize Docker runtime
	config := sandbox.DefaultConfig()
	runtime, err := sandbox.NewDockerRuntime(config)
	if err != nil {
		log.Fatalf("Failed to create Docker runtime: %v", err)
	}

	// Initialize sandbox pool
	poolConfig := sandbox.PoolConfig{
		MinSize:         2,
		MaxSize:         50,
		WarmupSize:      5,
		IdleTimeout:     30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		SandboxConfig:   config,
	}
	pool := sandbox.NewPool(poolConfig, runtime)

	server := &Server{
		pool:    pool,
		runtime: runtime,
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", server.handleHealth)

	// Sandbox management
	mux.HandleFunc("/api/v1/sandbox/acquire", server.handleAcquire)
	mux.HandleFunc("/api/v1/sandbox/release", server.handleRelease)
	mux.HandleFunc("/api/v1/sandbox/stats", server.handleStats)

	// Code execution
	mux.HandleFunc("/api/v1/execute", server.handleExecute)

	// File operations
	mux.HandleFunc("/api/v1/files", server.handleFiles)

	httpServer := &http.Server{
		Addr:         ":9090",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Long timeout for code execution
	}

	// Start server
	go func() {
		log.Println("Scheduler HTTP API listening on :9090")
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Println("Scheduler is running")
	log.Printf("Pool stats: %v", pool.Stats())

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Scheduler shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpServer.Shutdown(ctx)
	pool.Close()
	runtime.Close()

	log.Println("Scheduler stopped")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.pool.Stats())
}

type AcquireResponse struct {
	SandboxID   string `json:"sandbox_id"`
	ContainerID string `json:"container_id"`
	Status      string `json:"status"`
	IP          string `json:"ip,omitempty"`
}

func (s *Server) handleAcquire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	sb, err := s.pool.Acquire(ctx)
	if err != nil {
		log.Printf("Failed to acquire sandbox: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AcquireResponse{
		SandboxID:   sb.ID,
		ContainerID: sb.ContainerID,
		Status:      string(sb.Status),
		IP:          sb.IP,
	})
}

type ReleaseRequest struct {
	SandboxID string `json:"sandbox_id"`
}

func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := s.pool.Release(ctx, req.SandboxID); err != nil {
		log.Printf("Failed to release sandbox: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

type ExecuteRequest struct {
	SandboxID string            `json:"sandbox_id"`
	Code      string            `json:"code"`
	Language  string            `json:"language"`
	Command   []string          `json:"command,omitempty"`
	WorkDir   string            `json:"work_dir,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Timeout   int               `json:"timeout,omitempty"` // seconds
}

type ExecuteResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration int64  `json:"duration_ms"`
	TimedOut bool   `json:"timed_out"`
	Error    string `json:"error,omitempty"`
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SandboxID == "" {
		http.Error(w, "sandbox_id is required", http.StatusBadRequest)
		return
	}

	// Get sandbox from pool
	sb, err := s.pool.Get(r.Context(), req.SandboxID)
	if err != nil {
		http.Error(w, "Sandbox not found", http.StatusNotFound)
		return
	}

	// Build exec request
	execReq := sandbox.ExecRequest{
		Code:     req.Code,
		Language: req.Language,
		Command:  req.Command,
		WorkDir:  req.WorkDir,
		Env:      req.Env,
	}

	if req.Timeout > 0 {
		execReq.Timeout = time.Duration(req.Timeout) * time.Second
	}

	// Execute
	result, err := s.runtime.Exec(r.Context(), sb.ID, execReq)
	if err != nil {
		log.Printf("Execution error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExecuteResponse{
			ExitCode: -1,
			Error:    err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ExecuteResponse{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		Duration: result.Duration.Milliseconds(),
		TimedOut: result.TimedOut,
	})
}

type FileRequest struct {
	SandboxID string `json:"sandbox_id"`
	Path      string `json:"path"`
	Content   string `json:"content,omitempty"` // base64 encoded for write
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// List files - sandbox_id from query
		sandboxID := r.URL.Query().Get("sandbox_id")
		path := r.URL.Query().Get("path")

		if sandboxID == "" {
			http.Error(w, "sandbox_id is required", http.StatusBadRequest)
			return
		}
		if path == "" {
			path = "/workspace"
		}

		files, err := s.runtime.ListFiles(ctx, sandboxID, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"files": files})

	case http.MethodPut:
		// Write file - sandbox_id from body
		var req FileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.SandboxID == "" {
			http.Error(w, "sandbox_id is required", http.StatusBadRequest)
			return
		}

		if err := s.runtime.WriteFile(ctx, req.SandboxID, req.Path, []byte(req.Content)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodDelete:
		// Delete file - sandbox_id from query
		sandboxID := r.URL.Query().Get("sandbox_id")
		path := r.URL.Query().Get("path")

		if sandboxID == "" {
			http.Error(w, "sandbox_id is required", http.StatusBadRequest)
			return
		}
		if path == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}

		if err := s.runtime.DeleteFile(ctx, sandboxID, path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
