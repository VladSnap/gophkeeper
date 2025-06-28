package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/repository"
	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

// getUserIDFromContext extracts user ID from request context
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	if userIDStr, ok := ctx.Value(UserIDKey).(string); ok {
		return uuid.Parse(userIDStr)
	}
	return uuid.Nil, ErrUserNotFound
}

var ErrUserNotFound = errors.New("user not found in context")

// SyncHandler handles synchronization requests
type SyncHandler struct {
	secretRepo   repository.SecretRepositoryInterface
	metadataRepo repository.MetadataRepositoryInterface
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(secretRepo repository.SecretRepositoryInterface, metadataRepo repository.MetadataRepositoryInterface) *SyncHandler {
	return &SyncHandler{
		secretRepo:   secretRepo,
		metadataRepo: metadataRepo,
	}
}

// PullRequest represents a request to pull changes from server
type PullRequest struct {
	Since time.Time `json:"since"`
}

// PullResponse represents a response with changes from server
type PullResponse struct {
	Secrets   []*storage.Secret   `json:"secrets"`
	Metadata  []*storage.Metadata `json:"metadata"`
	Timestamp time.Time           `json:"timestamp"`
}

// PushRequest represents a request to push changes to server
type PushRequest struct {
	Secrets  []*storage.Secret   `json:"secrets"`
	Metadata []*storage.Metadata `json:"metadata"`
}

// PushResponse represents a response after pushing changes
type PushResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Pull handles requests to get changes from server
func (h *SyncHandler) Pull(w http.ResponseWriter, r *http.Request) {
	var req PullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Zap.Error("Failed to decode pull request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		log.Zap.Error("Failed to get user ID from context", zap.Error(err))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Zap.Info("Pull request received",
		zap.String("user_id", userID.String()),
		zap.Time("since", req.Since))

	// Get changed secrets since the specified time
	secrets, err := h.secretRepo.GetChangedSince(userID, req.Since)
	if err != nil {
		log.Zap.Error("Failed to get changed secrets", zap.Error(err))
		http.Error(w, "Failed to retrieve secrets", http.StatusInternalServerError)
		return
	}

	// Get changed metadata since the specified time
	metadata, err := h.metadataRepo.GetChangedSince(userID, req.Since)
	if err != nil {
		log.Zap.Error("Failed to get changed metadata", zap.Error(err))
		http.Error(w, "Failed to retrieve metadata", http.StatusInternalServerError)
		return
	}

	response := PullResponse{
		Secrets:   secrets,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	log.Zap.Info("Pull response prepared",
		zap.String("user_id", userID.String()),
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Zap.Error("Failed to encode pull response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// Push handles requests to send changes to server
func (h *SyncHandler) Push(w http.ResponseWriter, r *http.Request) {
	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Zap.Error("Failed to decode push request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		log.Zap.Error("Failed to get user ID from context", zap.Error(err))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Zap.Info("Push request received",
		zap.String("user_id", userID.String()),
		zap.Int("secrets_count", len(req.Secrets)),
		zap.Int("metadata_count", len(req.Metadata)))

	// Process secrets
	secretsProcessed := 0
	secretsErrors := 0
	for _, secret := range req.Secrets {
		log.Zap.Debug("Processing secret",
			zap.String("secret_id", secret.SecretID.String()),
			zap.String("user_id", secret.UserID.String()))

		// Ensure the secret belongs to the authenticated user
		if secret.UserID != userID {
			log.Zap.Warn("Secret user ID mismatch",
				zap.String("secret_user_id", secret.UserID.String()),
				zap.String("authenticated_user_id", userID.String()))
			secretsErrors++
			continue
		}

		// Try to get existing secret
		existingSecret, err := h.secretRepo.GetByID(secret.SecretID)
		if err != nil {
			// Secret doesn't exist, create it
			if err := h.secretRepo.Create(secret); err != nil {
				log.Zap.Error("Failed to create secret",
					zap.String("secret_id", secret.SecretID.String()),
					zap.Error(err))
				secretsErrors++
				continue
			}
			log.Zap.Debug("Secret created", zap.String("secret_id", secret.SecretID.String()))
		} else {
			// Secret exists, check if update is needed
			if existingSecret.LastUpdatedDate.Before(secret.LastUpdatedDate) {
				if err := h.secretRepo.Update(secret); err != nil {
					log.Zap.Error("Failed to update secret",
						zap.String("secret_id", secret.SecretID.String()),
						zap.Error(err))
					secretsErrors++
					continue
				}
				log.Zap.Debug("Secret updated", zap.String("secret_id", secret.SecretID.String()))
			} else {
				log.Zap.Debug("Secret is up to date", zap.String("secret_id", secret.SecretID.String()))
			}
		}
		secretsProcessed++
	}

	// Process metadata
	metadataProcessed := 0
	metadataErrors := 0
	for _, meta := range req.Metadata {
		log.Zap.Debug("Processing metadata",
			zap.String("metadata_id", meta.MetadataID.String()),
			zap.String("secret_id", meta.SecretID.String()))

		// Verify the secret exists and belongs to the user
		secret, err := h.secretRepo.GetByID(meta.SecretID)
		if err != nil {
			log.Zap.Warn("Metadata references non-existent secret",
				zap.String("metadata_id", meta.MetadataID.String()),
				zap.String("secret_id", meta.SecretID.String()))
			metadataErrors++
			continue
		}

		if secret.UserID != userID {
			log.Zap.Warn("Metadata secret user ID mismatch",
				zap.String("secret_user_id", secret.UserID.String()),
				zap.String("authenticated_user_id", userID.String()))
			metadataErrors++
			continue
		}

		// Try to get existing metadata
		existingMeta, err := h.metadataRepo.GetByID(meta.MetadataID)
		if err != nil {
			// Metadata doesn't exist, create it
			if err := h.metadataRepo.Create(meta); err != nil {
				log.Zap.Error("Failed to create metadata",
					zap.String("metadata_id", meta.MetadataID.String()),
					zap.Error(err))
				metadataErrors++
				continue
			}
			log.Zap.Debug("Metadata created", zap.String("metadata_id", meta.MetadataID.String()))
		} else {
			// Metadata exists, check if update is needed
			if existingMeta.LastUpdatedDate.Before(meta.LastUpdatedDate) {
				if err := h.metadataRepo.Update(meta); err != nil {
					log.Zap.Error("Failed to update metadata",
						zap.String("metadata_id", meta.MetadataID.String()),
						zap.Error(err))
					metadataErrors++
					continue
				}
				log.Zap.Debug("Metadata updated", zap.String("metadata_id", meta.MetadataID.String()))
			} else {
				log.Zap.Debug("Metadata is up to date", zap.String("metadata_id", meta.MetadataID.String()))
			}
		}
		metadataProcessed++
	}

	var message string
	success := secretsErrors == 0 && metadataErrors == 0

	if success {
		message = fmt.Sprintf("All changes processed successfully. Secrets: %d processed, Metadata: %d processed",
			secretsProcessed, metadataProcessed)
	} else {
		message = fmt.Sprintf("Processed with errors. Secrets: %d processed, %d errors. Metadata: %d processed, %d errors",
			secretsProcessed, secretsErrors, metadataProcessed, metadataErrors)
	}

	response := PushResponse{
		Success:   success,
		Message:   message,
		Timestamp: time.Now(),
	}

	log.Zap.Info("Push request processed",
		zap.String("user_id", userID.String()),
		zap.Bool("success", response.Success))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Zap.Error("Failed to encode push response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// Health handles health check requests
func (h *SyncHandler) Health(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "gophkeeper-server",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RegisterRoutes registers all sync-related routes
func (h *SyncHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/sync", func(r chi.Router) {
		r.Post("/pull", h.Pull)
		r.Post("/push", h.Push)
	})

	r.Get("/health", h.Health)
}
