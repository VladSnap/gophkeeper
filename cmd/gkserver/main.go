package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/app"
	"github.com/VladSnap/gophkeeper/internal/server/handlers"
	"github.com/VladSnap/gophkeeper/internal/server/repository"
	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Printf("Failed to close logger: %v\n", err)
		}
	}()

	log.Zap.Info("Starting gophkeeper server application")

	// Parse configuration
	conf, err := app.ParseFlags()
	if err != nil {
		log.Zap.Error("Failed to parse configuration", zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("Configuration loaded",
		zap.String("database_uri", conf.DatabaseURI),
		zap.String("server_address", conf.ServerAddress))

	// Initialize database
	db, err := storage.NewDatabaseServer(conf.DatabaseURI)
	if err != nil {
		log.Zap.Error("Failed to create database connection", zap.Error(err))
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Zap.Error("Failed to close database", zap.Error(err))
		}
	}()

	// Initialize database schema
	if err := db.InitDatabase(); err != nil {
		log.Zap.Error("Failed to initialize database", zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("Database initialized successfully")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB)
	sessionRepo := repository.NewSessionRepository(db.DB)
	secretRepo := repository.NewSecretRepository(db.DB)
	metadataRepo := repository.NewMetadataRepository(db.DB)

	// Initialize handlers
	jwtSecret := "secret-key" // TODO: Move to config
	authHandler := handlers.NewAuthHandler(userRepo, sessionRepo, jwtSecret)
	syncHandler := handlers.NewSyncHandler(secretRepo, metadataRepo)

	// Setup router
	r := chi.NewRouter()

	// Middleware
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

	// Register authentication routes (no auth required)
	authHandler.RegisterRoutes(r)

	// Create auth middleware
	authMiddleware := func(next http.Handler) http.Handler {
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

			// Add user information to context using the same keys as in sync.go
			ctx := context.WithValue(r.Context(), handlers.UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, handlers.UsernameKey, claims.Username)
			ctx = context.WithValue(ctx, handlers.SessionIDKey, claims.SessionID)

			// Call next handler with enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Register protected routes (requires authentication)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		syncHandler.RegisterRoutes(r)
	})

	log.Zap.Info("Routes registered successfully")

	// Create HTTP server
	srv := &http.Server{
		Addr:    conf.ServerAddress,
		Handler: r,
	}

	// Start server in goroutine
	go func() {
		log.Zap.Info("Starting HTTP server", zap.String("address", conf.ServerAddress))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Zap.Error("Failed to start server", zap.Error(err))
			os.Exit(1)
		}
	}()

	log.Zap.Info("Gophkeeper server is ready")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Zap.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Zap.Error("Server forced to shutdown", zap.Error(err))
		os.Exit(1)
	}

	log.Zap.Info("Server exited")
}
