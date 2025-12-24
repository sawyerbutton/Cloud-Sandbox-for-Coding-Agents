package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
	ErrMissingToken = errors.New("missing token")
)

// Claims represents JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// JWTAuth handles JWT authentication
type JWTAuth struct {
	secretKey     []byte
	tokenExpiry   time.Duration
	refreshExpiry time.Duration
}

// Config holds JWT configuration
type Config struct {
	SecretKey     string
	TokenExpiry   time.Duration
	RefreshExpiry time.Duration
}

// DefaultConfig returns default JWT configuration
func DefaultConfig() Config {
	return Config{
		SecretKey:     "your-secret-key-change-in-production",
		TokenExpiry:   24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
}

// NewJWTAuth creates a new JWT authenticator
func NewJWTAuth(config Config) *JWTAuth {
	return &JWTAuth{
		secretKey:     []byte(config.SecretKey),
		tokenExpiry:   config.TokenExpiry,
		refreshExpiry: config.RefreshExpiry,
	}
}

// GenerateToken generates a new JWT token
func (a *JWTAuth) GenerateToken(userID, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "cloud-sandbox",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
}

// GenerateRefreshToken generates a new refresh token
func (a *JWTAuth) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(a.refreshExpiry)),
		IssuedAt:  jwt.NewNumericDate(now),
		Subject:   userID,
		Issuer:    "cloud-sandbox-refresh",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
}

// ValidateToken validates a JWT token and returns claims
func (a *JWTAuth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ExtractTokenFromRequest extracts JWT token from request header
func ExtractTokenFromRequest(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingToken
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", ErrInvalidToken
	}

	return parts[1], nil
}

// Middleware returns an HTTP middleware for JWT authentication
func (a *JWTAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := ExtractTokenFromRequest(r)
		if err != nil {
			http.Error(w, `{"error":"unauthorized","message":"missing or invalid token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			if err == ErrExpiredToken {
				http.Error(w, `{"error":"unauthorized","message":"token expired"}`, http.StatusUnauthorized)
			} else {
				http.Error(w, `{"error":"unauthorized","message":"invalid token"}`, http.StatusUnauthorized)
			}
			return
		}

		// Add claims to request context
		ctx := SetClaimsContext(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MiddlewareFunc returns an HTTP middleware function
func (a *JWTAuth) MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Middleware(http.HandlerFunc(next)).ServeHTTP(w, r)
	}
}
