package management

import (
	"context"
	"fmt"
	"time"
	"tokentracer-proxy/pkg/db"
	"tokentracer-proxy/pkg/provider"
)

// StartModelPolling starts a background goroutine that polls providers for models every 12 hours
func StartModelPolling(ctx context.Context) {
	// 1. Initial run on startup
	pollModels(ctx)

	// 2. Set up ticker for every 12 hours
	ticker := time.NewTicker(12 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				pollModels(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func pollModels(ctx context.Context) {
	fmt.Println("Polling providers for models...")

	// 1. Seed all known providers first so we have defaults even with no keys
	for _, p := range provider.SupportedProviders() {
		seedCommonModels(ctx, p)
	}

	// 2. Poll using one key per provider type
	results, err := db.Repo.ListUniqueProviderKeysPerProvider(ctx)
	if err != nil {
		fmt.Printf("Failed to query provider keys for polling: %v\n", err)
		return
	}

	for _, k := range results {
		fmt.Printf("Polling real-time models for %s using key ID %d...\n", k.Provider, k.ID)
		var prov provider.Provider
		switch k.Provider {
		case "openai":
			prov = provider.NewOpenAIProvider(db.Repo, k.ID, k.UserID)
		case "anthropic":
			prov = provider.NewAnthropicProvider(db.Repo, k.ID, k.UserID)
		case "gemini":
			prov = provider.NewGeminiProvider(db.Repo, k.ID, k.UserID)
		}

		if prov != nil {
			models, err := prov.ListModels(ctx)
			if err == nil {
				for _, m := range models {
					db.Repo.InsertProviderModel(ctx, k.Provider, m)
				}
			} else {
				fmt.Printf("Failed to list models for provider %s: %v\n", k.Provider, err)
			}
		}
	}
	fmt.Println("Model polling complete.")
}

func seedCommonModels(ctx context.Context, providerType string) {
	var commonModels []string
	switch providerType {
	case "openai":
		commonModels = []string{"gpt-5", "gpt-5.2-thinking", "gpt-5.2-pro", "gpt-4o", "gpt-4o-mini", "o3-pro", "o4-mini"}
	case "anthropic":
		commonModels = []string{"claude-4.5-opus", "claude-4.5-sonnet", "claude-4.5-haiku", "claude-4-sonnet", "claude-4-opus"}
	case "gemini":
		commonModels = []string{"gemini-3-pro", "gemini-3-flash", "gemini-2.5-pro", "gemini-2.5-flash"}
	}
	for _, m := range commonModels {
		db.Repo.InsertProviderModel(ctx, providerType, m)
	}
}
