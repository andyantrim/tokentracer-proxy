package management

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/db"

	"github.com/go-chi/chi/v5"
)

// -- Handlers --

// GetUsageStats returns basic aggregated stats
func GetUsageStats(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.KeyUser).(int)

	results, err := db.Repo.GetUsageStats(context.Background(), userID)
	if err != nil {
		log.Printf("get usage stats error: %v", err)
		http.Error(w, "Failed to retrieve usage stats", http.StatusInternalServerError)
		return
	}

	var stats []map[string]interface{}
	for _, s := range results {
		stats = append(stats, map[string]interface{}{
			"provider": s.Provider, "alias": s.Alias, "input_tokens": s.Input, "output_tokens": s.Output, "requests": s.Reqs,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func RegisterRoutes(r chi.Router) {
	r.Post("/providers", CreateProviderKey)
	r.Get("/providers", ListProviderKeys)
	r.Get("/providers/{keyID}/models", ListProviderModels)
	r.Get("/models", ListAllModels)

	r.Post("/aliases", UpsertModelAlias)
	r.Get("/aliases", ListAliases)
	r.Patch("/aliases/{alias}", PatchModelAlias)

	r.Get("/usage", GetUsageStats)
}
