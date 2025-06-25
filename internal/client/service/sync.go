package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VladSnap/gophkeeper/internal/client/models"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

// SyncService handles synchronization with the server
type SyncService struct {
	serverURL   string
	authService *AuthService
	httpClient  *http.Client
}

// PullRequest represents a request to pull changes from server
type PullRequest struct {
	Since time.Time `json:"since"`
}

// PullResponse represents a response with changes from server
type PullResponse struct {
	Secrets   []*models.Secret   `json:"secrets"`
	Metadata  []*models.Metadata `json:"metadata"`
	Timestamp time.Time          `json:"timestamp"`
}

// PushRequest represents a request to push changes to server
type PushRequest struct {
	Secrets  []*models.Secret   `json:"secrets"`
	Metadata []*models.Metadata `json:"metadata"`
}

// PushResponse represents a response after pushing changes
type PushResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// NewSyncService creates a new sync service
func NewSyncService(serverURL string, authService *AuthService) *SyncService {
	return &SyncService{
		serverURL:   serverURL,
		authService: authService,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Pull retrieves changes from the server
func (s *SyncService) Pull(since time.Time) (*PullResponse, error) {
	req := PullRequest{
		Since: since,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pull request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", s.serverURL+"/api/v1/sync/pull", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	// Add authorization header
	authHeader, err := s.authService.GetAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth header: %w", err)
	}
	httpReq.Header.Set("Authorization", authHeader)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pull request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read pull response: %w", err)
	}

	var pullResp PullResponse
	if err := json.Unmarshal(body, &pullResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pull response: %w", err)
	}

	log.Zap.Info("Pull completed successfully",
		zap.Int("secrets_count", len(pullResp.Secrets)),
		zap.Int("metadata_count", len(pullResp.Metadata)),
		zap.Time("timestamp", pullResp.Timestamp))

	return &pullResp, nil
}

// Push sends changes to the server
func (s *SyncService) Push(secrets []*models.Secret, metadata []*models.Metadata) (*PushResponse, error) {
	req := PushRequest{
		Secrets:  secrets,
		Metadata: metadata,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal push request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", s.serverURL+"/api/v1/sync/push", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create push request: %w", err)
	}

	// Add authorization header
	authHeader, err := s.authService.GetAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth header: %w", err)
	}
	httpReq.Header.Set("Authorization", authHeader)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("push request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read push response: %w", err)
	}

	var pushResp PushResponse
	if err := json.Unmarshal(body, &pushResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal push response: %w", err)
	}

	log.Zap.Info("Push completed",
		zap.Int("secrets_count", len(secrets)),
		zap.Int("metadata_count", len(metadata)),
		zap.Bool("success", pushResp.Success),
		zap.String("message", pushResp.Message))

	return &pushResp, nil
}
