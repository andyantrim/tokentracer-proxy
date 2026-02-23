package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
	"tokentracer-proxy/pkg/db"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

// Init ensures we have a secret
func Init() {
	if len(jwtSecret) == 0 {
		panic("JWT_SECRET must be set")
	}
}

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

// SignupHandler registers a new user
func SignupHandler(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("signup: bcrypt error: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Insert into DB
	_, err = db.Repo.CreateUser(context.Background(), creds.Email, string(hashedPassword))

	if err != nil {
		log.Printf("signup error: %v", err)
		http.Error(w, "User already exists or DB error", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "User created"}); err != nil {
		log.Printf("signup: encode response error: %v", err)
	}
}

// LoginHandler authenticates a user and returns a session JWT
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	id, storedHash, err := db.Repo.GetUserByEmail(context.Background(), creds.Email)

	if err != nil {
		log.Printf("login error: %v", err)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(creds.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Identify this as a session token
	token, err := generateJWT(id, "session", 24*time.Hour)
	if err != nil {
		log.Printf("generate token error: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(AuthResponse{Token: token}); err != nil {
		log.Printf("login: encode response error: %v", err)
	}
}

// GenerateAPIKeyHandler creates a long-lived JWT for API usage
func GenerateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	// Middleware should have already validated the session and set UserID context
	userID := r.Context().Value(KeyUser)
	if userID == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Create a random name or take from request
	keyName := "api-key-" + time.Now().Format("20060102-150405")

	// Generate a long-lived JWT (e.g., 1 year)
	// We mark this as an 'api_key' type claim to distinguish scope if needed
	token, err := generateJWT(userID.(int), "api_key", 365*24*time.Hour)
	if err != nil {
		log.Printf("generate key error: %v", err)
		http.Error(w, "Failed to generate key", http.StatusInternalServerError)
		return
	}

	// Store a SHA-256 hash of the token (not the raw token) for revocation/tracking.
	prefix := token[:8]
	hash := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(hash[:])

	err = db.Repo.CreateAPIKey(context.Background(), userID.(int), keyName, keyHash, prefix)

	if err != nil {
		log.Printf("create api key error: %v", err)
		http.Error(w, "Failed to create API key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(AuthResponse{Token: token}); err != nil {
		log.Printf("generate api key: encode response error: %v", err)
	}
}

// UserInfoHandler returns details about the authenticated user
func UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(KeyUser).(int)
	email, _, _, err := db.Repo.GetUserByID(context.Background(), userID)
	if err != nil {
		log.Printf("User not found : %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"email": email}); err != nil {
		log.Printf("user info: encode response error: %v", err)
	}
}

func generateJWT(userID int, scope string, duration time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"scope": scope,
		"exp":   time.Now().Add(duration).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
