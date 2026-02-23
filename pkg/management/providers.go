package management

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/crypto"
	"tokentracer-proxy/pkg/db"
)

type ProviderKeyRequest struct {
	Provider     string `json:"provider"`
	EncryptedKey string `json:"api_key"`
	Label        string `json:"label"`
}

// CreateProviderKey stores a downstream provider's key (e.g. OpenAI)
func CreateProviderKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.KeyUser).(int)

	var req ProviderKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	encrypted, err := crypto.Encrypt(req.EncryptedKey)
	if err != nil {
		log.Printf("encrypt provider key error: %v", err)
		http.Error(w, "Failed to create provider key", http.StatusInternalServerError)
		return
	}

	err = db.Repo.CreateProviderKey(context.Background(), userID, req.Provider, encrypted, req.Label)
	if err != nil {
		log.Printf("create provider key error: %v", err)
		http.Error(w, "Failed to create provider key", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// ListProviderKeys returns all keys for the user
func ListProviderKeys(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.KeyUser).(int)

	results, err := db.Repo.ListProviderKeys(context.Background(), userID)
	if err != nil {
		log.Printf("list provider keys error for user %d: %v", userID, err)
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}

	var keys []map[string]interface{}
	for _, k := range results {
		keys = append(keys, map[string]interface{}{
			"id": k.ID, "provider": k.Provider, "label": k.Label, "created_at": k.CreatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(keys); err != nil {
		log.Printf("list provider keys: encode response error: %v", err)
	}
}
