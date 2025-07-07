package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/handlers"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func (app *Application) SetupRoutes() *chi.Mux {
	r := chi.NewRouter()

	// Setup middleware
	app.setupMiddleware(r)

	// Register authentication routes (no auth required)
	app.AuthHandler.RegisterRoutes(r)

	// Register protected routes (requires authentication)
	r.Group(func(r chi.Router) {
		r.Use(app.authMiddleware)
		app.SyncHandler.RegisterRoutes(r)
	})

	log.Zap.Info("Routes registered successfully")
	return r
}

func (app *Application) setupMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS middleware for development
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})
}

func (app *Application) authMiddleware(next http.Handler) http.Handler {
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
		claims, err := app.AuthHandler.VerifyJWT(token)
		if err != nil {
			log.Zap.Warn("Invalid JWT token", zap.Error(err))
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user information to context using the same keys as in sync.go
		ctx := context.WithValue(r.Context(), handlers.UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, handlers.UsernameKey, claims.Username)
		ctx = context.WithValue(ctx, handlers.SessionIDKey, claims.SessionID)

		// Call next handler with enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
