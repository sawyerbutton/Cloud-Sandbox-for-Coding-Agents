package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloud-sandbox/cloud-sandbox/internal/auth"
	"github.com/cloud-sandbox/cloud-sandbox/internal/middleware"
)

// Config holds gateway configuration
type Config struct {
	Port              string
	SchedulerURL      string
	SessionManagerURL string
	JWTSecret         string
	AllowedOrigins    []string
}

// Gateway is the API gateway server
type Gateway struct {
	config      Config
	jwtAuth     *auth.JWTAuth
	rateLimiter *middleware.RateLimiter
	httpClient  *http.Client
}

func main() {
	log.Println("Starting Cloud Sandbox Gateway...")

	config := Config{
		Port:              getEnv("PORT", "8080"),
		SchedulerURL:      getEnv("SCHEDULER_URL", "http://localhost:9090"),
		SessionManagerURL: getEnv("SESSION_MANAGER_URL", "http://localhost:9091"),
		JWTSecret:         getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		AllowedOrigins:    []string{"*"},
	}

	jwtAuth := auth.NewJWTAuth(auth.Config{
		SecretKey:   config.JWTSecret,
		TokenExpiry: 24 * time.Hour,
	})

	rateLimiter := middleware.NewRateLimiter(middleware.RateLimitConfig{
		Rate:     100,
		Interval: time.Minute,
		Burst:    200,
	})

	gateway := &Gateway{
		config:      config,
		jwtAuth:     jwtAuth,
		rateLimiter: rateLimiter,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}

	// Create router
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("/health", gateway.handleHealth)
	mux.HandleFunc("/api/v1/auth/token", gateway.handleToken)

	// Protected endpoints
	mux.HandleFunc("/api/v1/sandbox/", gateway.handleSandbox)
	mux.HandleFunc("/api/v1/sessions/", gateway.handleSession)
	mux.HandleFunc("/api/v1/sessions", gateway.handleSessions)
	mux.HandleFunc("/api/v1/execute", gateway.handleExecute)
	mux.HandleFunc("/api/v1/files", gateway.handleFiles)

	// Apply middleware chain
	handler := middleware.Recovery(
		middleware.Logging(
			middleware.CORS(config.AllowedOrigins)(
				rateLimiter.Middleware(mux),
			),
		),
	)

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
	}

	// Start server
	go func() {
		log.Printf("Gateway listening on :%s", config.Port)
		log.Printf("Scheduler URL: %s", config.SchedulerURL)
		log.Printf("Session Manager URL: %s", config.SessionManagerURL)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Gateway shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	log.Println("Gateway stopped")
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "gateway",
	})
}

// handleToken generates JWT tokens (for demo purposes)
func (g *Gateway) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, `{"error":"user_id required"}`, http.StatusBadRequest)
		return
	}

	token, err := g.jwtAuth.GenerateToken(req.UserID, req.Role)
	if err != nil {
		http.Error(w, `{"error":"token generation failed"}`, http.StatusInternalServerError)
		return
	}

	refreshToken, _ := g.jwtAuth.GenerateRefreshToken(req.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token":  token,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    "86400",
	})
}

// handleSandbox proxies sandbox management requests
func (g *Gateway) handleSandbox(w http.ResponseWriter, r *http.Request) {
	// Authenticate
	if !g.authenticate(w, r) {
		return
	}

	// Proxy to scheduler
	g.proxy(w, r, g.config.SchedulerURL)
}

// handleSessions proxies session list/create requests
func (g *Gateway) handleSessions(w http.ResponseWriter, r *http.Request) {
	if !g.authenticate(w, r) {
		return
	}

	// For POST, inject user_id from token
	if r.Method == http.MethodPost {
		claims, _ := auth.GetClaimsFromContext(r.Context())
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req map[string]interface{}
		json.Unmarshal(body, &req)
		if req == nil {
			req = make(map[string]interface{})
		}
		req["user_id"] = claims.UserID

		newBody, _ := json.Marshal(req)
		r.Body = io.NopCloser(bytes.NewReader(newBody))
		r.ContentLength = int64(len(newBody))
	}

	// For GET, add user_id query param
	if r.Method == http.MethodGet {
		claims, _ := auth.GetClaimsFromContext(r.Context())
		q := r.URL.Query()
		q.Set("user_id", claims.UserID)
		r.URL.RawQuery = q.Encode()
	}

	g.proxy(w, r, g.config.SessionManagerURL)
}

// handleSession proxies individual session requests
func (g *Gateway) handleSession(w http.ResponseWriter, r *http.Request) {
	if !g.authenticate(w, r) {
		return
	}
	g.proxy(w, r, g.config.SessionManagerURL)
}

// handleExecute proxies code execution requests
func (g *Gateway) handleExecute(w http.ResponseWriter, r *http.Request) {
	if !g.authenticate(w, r) {
		return
	}
	g.proxy(w, r, g.config.SchedulerURL)
}

// handleFiles proxies file operation requests
func (g *Gateway) handleFiles(w http.ResponseWriter, r *http.Request) {
	if !g.authenticate(w, r) {
		return
	}
	g.proxy(w, r, g.config.SchedulerURL)
}

// authenticate validates JWT and adds claims to context
func (g *Gateway) authenticate(w http.ResponseWriter, r *http.Request) bool {
	token, err := auth.ExtractTokenFromRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "unauthorized",
			"message": "missing or invalid authorization header",
		})
		return false
	}

	claims, err := g.jwtAuth.ValidateToken(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "unauthorized",
			"message": err.Error(),
		})
		return false
	}

	// Add claims to request context
	ctx := auth.SetClaimsContext(r.Context(), claims)
	*r = *r.WithContext(ctx)

	return true
}

// proxy forwards the request to the backend service
func (g *Gateway) proxy(w http.ResponseWriter, r *http.Request, targetURL string) {
	// Build target URL
	url := targetURL + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	// Create proxy request
	body, _ := io.ReadAll(r.Body)
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":"proxy_error"}`, http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Add forwarded headers
	proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	proxyReq.Header.Set("X-Forwarded-Host", r.Host)

	// Add user ID header
	if claims, ok := auth.GetClaimsFromContext(r.Context()); ok {
		proxyReq.Header.Set("X-User-ID", claims.UserID)
	}

	// Send request
	resp, err := g.httpClient.Do(proxyReq)
	if err != nil {
		log.Printf("[Gateway] Proxy error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": fmt.Sprintf("backend service unavailable: %s", targetURL),
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code and body
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
