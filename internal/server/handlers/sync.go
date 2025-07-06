package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/service"
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
	syncService service.SyncServiceInterface
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(syncService service.SyncServiceInterface) *SyncHandler {
	return &SyncHandler{
		syncService: syncService,
	}
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

	// Get changed data using sync service
	secrets, metadata, err := h.syncService.PullChanges(userID, req.Since)
	if err != nil {
		log.Zap.Error("Failed to pull changes", zap.Error(err))
		http.Error(w, "Failed to retrieve changes", http.StatusInternalServerError)
		return
	}

	response := PullResponse{
		Secrets:   convertSecretsToResponse(secrets),
		Metadata:  convertMetadataSliceToResponse(metadata),
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

	// Convert handler types to service types
	serviceSecrets := make([]*service.ClientSecret, len(req.Secrets))
	for i, secret := range req.Secrets {
		serviceSecrets[i] = convertSecretRequestToService(secret)
	}

	serviceMetadata := make([]*service.ClientMetadata, len(req.Metadata))
	for i, meta := range req.Metadata {
		serviceMetadata[i] = convertMetadataRequestToService(meta)
	}

	// Process changes using sync service
	result, err := h.syncService.PushChanges(userID, serviceSecrets, serviceMetadata)
	if err != nil {
		log.Zap.Error("Failed to push changes", zap.Error(err))
		http.Error(w, "Failed to process changes", http.StatusInternalServerError)
		return
	}

	response := PushResponse{
		Success:   result.Success,
		Message:   result.Message,
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

// convertSecretRequestToService converts handler SecretRequest to service ClientSecret
func convertSecretRequestToService(req *SecretRequest) *service.ClientSecret {
	return &service.ClientSecret{
		SecretID:        req.SecretID,
		Encrypted:       req.Encrypted,
		CreatedDate:     req.CreatedDate,
		LastUpdatedDate: req.LastUpdatedDate,
	}
}

// convertMetadataRequestToService converts handler MetadataRequest to service ClientMetadata
func convertMetadataRequestToService(req *MetadataRequest) *service.ClientMetadata {
	return &service.ClientMetadata{
		MetadataID:      req.MetadataID,
		SecretID:        req.SecretID,
		Key:             req.Key,
		ValueHash:       req.ValueHash,
		ValueEncrypted:  req.ValueEncrypted,
		CreatedDate:     req.CreatedDate,
		LastUpdatedDate: req.LastUpdatedDate,
	}
}

// convertSecretToResponse converts storage Secret to handler SecretResponse
func convertSecretToResponse(secret *storage.Secret) *SecretResponse {
	return &SecretResponse{
		SecretID:        secret.SecretID,
		Encrypted:       secret.Encrypted,
		CreatedDate:     secret.CreatedDate,
		LastUpdatedDate: secret.LastUpdatedDate,
	}
}

// convertMetadataToResponse converts storage Metadata to handler MetadataResponse
func convertMetadataToResponse(meta *storage.Metadata) *MetadataResponse {
	return &MetadataResponse{
		MetadataID:      meta.MetadataID,
		SecretID:        meta.SecretID,
		Key:             meta.Key,
		ValueHash:       meta.ValueHash,
		ValueEncrypted:  meta.ValueEncrypted,
		CreatedDate:     meta.CreatedDate,
		LastUpdatedDate: meta.LastUpdatedDate,
	}
}

// convertSecretsToResponse converts slice of storage Secrets to slice of handler SecretResponse
func convertSecretsToResponse(secrets []*storage.Secret) []*SecretResponse {
	responses := make([]*SecretResponse, len(secrets))
	for i, secret := range secrets {
		responses[i] = convertSecretToResponse(secret)
	}
	return responses
}

// convertMetadataToResponse converts slice of storage Metadata to slice of handler MetadataResponse
func convertMetadataSliceToResponse(metadata []*storage.Metadata) []*MetadataResponse {
	responses := make([]*MetadataResponse, len(metadata))
	for i, meta := range metadata {
		responses[i] = convertMetadataToResponse(meta)
	}
	return responses
}
