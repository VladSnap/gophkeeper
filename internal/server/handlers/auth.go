package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/VladSnap/gophkeeper/internal/server/repository"
	"github.com/VladSnap/gophkeeper/internal/server/storage"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	userRepo    repository.UserRepositoryInterface
	sessionRepo repository.SessionRepositoryInterface
	jwtSecret   []byte
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(userRepo repository.UserRepositoryInterface, sessionRepo repository.SessionRepositoryInterface, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtSecret:   []byte(jwtSecret),
	}
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // Already hashed on client side
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // Already hashed on client side
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Message string `json:"message,omitempty"`
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Zap.Error("Failed to decode register request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Zap.Info("Registration request received", zap.String("username", req.Username))

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	existingUser, err := h.userRepo.GetByLogin(req.Username)
	if err == nil && existingUser != nil {
		log.Zap.Warn("User already exists", zap.String("username", req.Username))
		response := AuthResponse{
			Success: false,
			Message: "User already exists",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Hash the already-hashed password from client with salt on server
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Zap.Error("Failed to hash password", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create new user
	user := &storage.User{
		UserID:    uuid.New(),
		Login:     req.Username,
		Password:  string(hashedPassword),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.userRepo.Create(user); err != nil {
		log.Zap.Error("Failed to create user", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	log.Zap.Info("User registered successfully",
		zap.String("username", user.Login),
		zap.String("user_id", user.UserID.String()))

	response := AuthResponse{
		Success: true,
		UserID:  user.UserID.String(),
		Message: "User registered successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Zap.Error("Failed to decode login request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Zap.Info("Login request received", zap.String("username", req.Username))

	// Get user by username
	user, err := h.userRepo.GetByLogin(req.Username)
	if err != nil || user == nil {
		log.Zap.Warn("User not found", zap.String("username", req.Username))
		response := AuthResponse{
			Success: false,
			Message: "Invalid credentials",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		log.Zap.Warn("Invalid password", zap.String("username", req.Username))
		response := AuthResponse{
			Success: false,
			Message: "Invalid credentials",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create session
	session := &storage.Session{
		SessionID:   uuid.New(),
		UserID:      user.UserID,
		StartedDate: time.Now(),
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour), // 24 hours
		IsActive:    true,
	}

	if err := h.sessionRepo.Create(session); err != nil {
		log.Zap.Error("Failed to create session", zap.Error(err))
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	token, err := h.generateJWT(user, session)
	if err != nil {
		log.Zap.Error("Failed to generate JWT", zap.Error(err))
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	log.Zap.Info("User logged in successfully",
		zap.String("username", user.Login),
		zap.String("user_id", user.UserID.String()),
		zap.String("session_id", session.SessionID.String()))

	response := AuthResponse{
		Success: true,
		Token:   token,
		UserID:  user.UserID.String(),
		Message: "Login successful",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateJWT creates a JWT token for the user
func (h *AuthHandler) generateJWT(user *storage.User, session *storage.Session) (string, error) {
	claims := JWTClaims{
		UserID:    user.UserID.String(),
		Username:  user.Login,
		SessionID: session.SessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(session.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gophkeeper-server",
			Subject:   user.UserID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}

// VerifyJWT validates a JWT token and returns claims
func (h *AuthHandler) VerifyJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return h.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		// Verify session is still active
		sessionID, err := uuid.Parse(claims.SessionID)
		if err != nil {
			return nil, err
		}

		session, err := h.sessionRepo.GetByID(sessionID)
		if err != nil || session == nil || !session.IsActive || session.ExpiresAt.Before(time.Now()) {
			return nil, errors.New("token expired")
		}

		return claims, nil
	}

	return nil, errors.New("token invalid")
}

// RegisterRoutes registers authentication routes
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
	})
}
