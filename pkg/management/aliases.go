package management

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/db"

	"github.com/go-chi/chi/v5"
)

type ModelAliasRequest struct {
	ID                  int     `json:"id"`
	Alias               string  `json:"alias"`
	TargetModel         string  `json:"target_model"`
	ProviderKeyID       int     `json:"provider_key_id"`
	FallbackAliasID     *int    `json:"fallback_alias_id"`
	UseLightModel       bool    `json:"use_light_model"`
	LightModelThreshold int     `json:"light_model_threshold"`
	LightModel          *string `json:"light_model"`
}

// UpsertModelAlias creates or updates a routing rule
func UpsertModelAlias(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.KeyUser).(int)

	var req ModelAliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Alias == "" {
		http.Error(w, "Alias name is required", http.StatusBadRequest)
		return
	}
	if req.TargetModel == "" {
		http.Error(w, "Target model is required", http.StatusBadRequest)
		return
	}
	if req.ProviderKeyID <= 0 {
		http.Error(w, "A valid provider key is required", http.StatusBadRequest)
		return
	}

	// Normalize optional fields: treat zero as null
	if req.FallbackAliasID != nil && *req.FallbackAliasID <= 0 {
		req.FallbackAliasID = nil
	}
	if req.LightModel != nil && *req.LightModel == "" {
		req.LightModel = nil
	}

	err := db.Repo.UpsertModelAlias(context.Background(), userID, req.Alias, req.TargetModel, req.ProviderKeyID, req.FallbackAliasID, req.UseLightModel, req.LightModelThreshold, req.LightModel)
	if err != nil {
		log.Printf("upsert model alias error: %v", err)
		http.Error(w, "Failed to save model alias", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// PatchModelAlias updates specific fields of a routing rule
func PatchModelAlias(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.KeyUser).(int)
	aliasName := chi.URLParam(r, "alias")

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err := db.Repo.PatchModelAlias(context.Background(), userID, aliasName, req)
	if err != nil {
		log.Printf("patch model alias error: %v", err)
		http.Error(w, "Failed to update model alias", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ListAliases returns all routing rules
func ListAliases(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.KeyUser).(int)

	results, err := db.Repo.ListModelAliases(context.Background(), userID)
	if err != nil {
		log.Printf("list aliases error for user %d: %v", userID, err)
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}

	var aliases []ModelAliasRequest
	for _, a := range results {
		aliases = append(aliases, ModelAliasRequest{
			ID:                  a.ID,
			Alias:               a.Alias,
			TargetModel:         a.TargetModel,
			ProviderKeyID:       a.ProviderKeyID,
			FallbackAliasID:     a.FallbackAliasID,
			UseLightModel:       a.UseLightModel,
			LightModelThreshold: a.LightModelThreshold,
			LightModel:          a.LightModel,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(aliases); err != nil {
		log.Printf("list aliases: encode response error: %v", err)
	}
}

// ListProviderModels returns cached models for a given provider key
func ListProviderModels(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.KeyUser).(int)
	keyID := chi.URLParam(r, "keyID")

	keyIDInt := 0
	if _, err := fmt.Sscanf(keyID, "%d", &keyIDInt); err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	providerName, _, err := db.Repo.GetProviderKey(context.Background(), keyIDInt, userID)
	if err != nil {
		log.Printf("list provider models: get provider key %d error for user %d: %v", keyIDInt, userID, err)
		http.Error(w, "Unauthorized or not found", http.StatusUnauthorized)
		return
	}

	models, err := db.Repo.ListProviderModelsByType(context.Background(), providerName)
	if err != nil {
		log.Printf("list provider models: list models error for provider %q: %v", providerName, err)
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models); err != nil {
		log.Printf("list provider models: encode response error: %v", err)
	}
}

// ListAllModels returns all cached models for all providers
func ListAllModels(w http.ResponseWriter, r *http.Request) {
	models, err := db.Repo.ListAllProviderModels(context.Background())
	if err != nil {
		log.Printf("list all models error: %v", err)
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models); err != nil {
		log.Printf("list all models: encode response error: %v", err)
	}
}
