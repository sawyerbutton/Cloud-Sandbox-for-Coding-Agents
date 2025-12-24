package auth

import (
	"context"
)

type contextKey string

const (
	claimsKey contextKey = "claims"
)

// SetClaimsContext adds claims to context
func SetClaimsContext(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// GetClaimsFromContext retrieves claims from context
func GetClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	return claims, ok
}

// GetUserIDFromContext retrieves user ID from context
func GetUserIDFromContext(ctx context.Context) string {
	claims, ok := GetClaimsFromContext(ctx)
	if !ok {
		return ""
	}
	return claims.UserID
}
