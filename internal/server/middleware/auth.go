package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/VladSnap/gophkeeper/internal/server/handlers"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "user_id"
	// UsernameKey is the context key for username
	UsernameKey ContextKey = "username"
	// SessionIDKey is the context key for session ID
	SessionIDKey ContextKey = "session_id"
)

// AuthMiddleware creates a middleware that validates JWT tokens
func AuthMiddleware(authHandler *handlers.AuthHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Zap.Warn("No Authorization header provided")
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Check if the header has Bearer token
			if !strings.HasPrefix(authHeader, "Bearer ") {
				log.Zap.Warn("Invalid Authorization header format")
				http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
				return
			}

			// Extract token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				log.Zap.Warn("Empty token in Authorization header")
				http.Error(w, "Empty token", http.StatusUnauthorized)
				return
			}

			// Verify JWT token
			claims, err := authHandler.VerifyJWT(token)
			if err != nil {
				log.Zap.Warn("Invalid JWT token", zap.Error(err))
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Add user information to context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UsernameKey, claims.Username)
			ctx = context.WithValue(ctx, SessionIDKey, claims.SessionID)

			// Call next handler with enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext extracts user ID from request context
func GetUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetUsernameFromContext extracts username from request context
func GetUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(UsernameKey).(string); ok {
		return username
	}
	return ""
}

// GetSessionIDFromContext extracts session ID from request context
func GetSessionIDFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value(SessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}
