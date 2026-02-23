package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/db"
	"tokentracer-proxy/pkg/provider"
	"tokentracer-proxy/pkg/types"
)

// Provider factories for testing
type ProviderCreator func(repository db.Repository, providerKeyID, userID int) provider.Provider

var (
	AnthropicProviderFactory ProviderCreator = func(r db.Repository, k, u int) provider.Provider {
		return provider.NewAnthropicProvider(r, k, u)
	}
	OpenAIProviderFactory ProviderCreator = func(r db.Repository, k, u int) provider.Provider {
		return provider.NewOpenAIProvider(r, k, u)
	}
	GeminiProviderFactory ProviderCreator = func(r db.Repository, k, u int) provider.Provider {
		return provider.NewGeminiProvider(r, k, u)
	}
)

type ProxyServer struct {
	Repo db.Repository
}

func NewProxyServer(repo db.Repository) *ProxyServer {
	return &ProxyServer{Repo: repo}
}

func (s *ProxyServer) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// 0. Get User from Context (set by AuthMiddleware)
	userID, ok := r.Context().Value(auth.KeyUser).(int)
	if !ok {
		log.Printf("proxy handler: missing user context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 1. Decode OpenAI Request
	var openAIReq types.OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&openAIReq); err != nil {
		log.Printf("proxy handler: decode request body error: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 2. Resolve Alias and Handle Request (with fallback)
	currentModel := openAIReq.Model
	maxDepth := 2 // Prevent infinite loops
	var lastErr error

	for i := 0; i < maxDepth; i++ {
		// Lookup Model Alias
		alias, err := s.Repo.GetModelAlias(r.Context(), userID, currentModel)

		if err != nil {
			log.Printf("proxy handler: get model alias %q error: %v", currentModel, err)
			http.Error(w, "Unknown model alias: "+currentModel, http.StatusNotFound)
			return
		}

		// Fetch Provider Type
		providerType, _, err := s.Repo.GetProviderKey(r.Context(), alias.ProviderKeyID, userID)

		if err != nil {
			log.Printf("proxy handler: get provider key for alias %q error: %v", currentModel, err)
			http.Error(w, "Provider configuration not found", http.StatusInternalServerError)
			return
		}

		// Instantiate Provider Strategy
		var prov provider.Provider
		switch providerType {
		case "anthropic":
			prov = AnthropicProviderFactory(s.Repo, alias.ProviderKeyID, userID)
		case "openai":
			prov = OpenAIProviderFactory(s.Repo, alias.ProviderKeyID, userID)
		case "gemini":
			prov = GeminiProviderFactory(s.Repo, alias.ProviderKeyID, userID)
		default:
			log.Printf("proxy handler: unsupported provider type %q for alias %q", providerType, currentModel)
			http.Error(w, "Unsupported provider: "+providerType, http.StatusBadRequest)
			return
		}

		// Send Request
		reqCopy := openAIReq
		reqCopy.Model = alias.TargetModel

		// Check for light model optimization
		if alias.UseLightModel && alias.LightModel != nil && *alias.LightModel != "" {
			tokens := estimateTokens(openAIReq.Messages)
			if tokens < alias.LightModelThreshold {
				reqCopy.Model = *alias.LightModel
			}
		}

		openAIResp, err := prov.Send(r.Context(), reqCopy)
		if err != nil {
			if alias.FallbackAliasID != nil {
				// Get fallback alias name
				fallbackAliasName, errFB := s.Repo.GetModelAliasByID(r.Context(), *alias.FallbackAliasID)
				if errFB == nil {
					log.Printf("proxy handler: provider request failed for alias %q (user %d), trying fallback: %v", currentModel, userID, err)
					currentModel = fallbackAliasName
					lastErr = err
					continue // Try again with fallback alias
				}
			}
			log.Printf("proxy handler: provider request failed for alias %q (user %d): %v", currentModel, userID, err)
			http.Error(w, "Provider request failed", http.StatusBadGateway)
			return
		}

		// Success!
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(openAIResp); err != nil {
			log.Printf("proxy handler: encode response error: %v", err)
		}

		// Async Logging
		go func(uid int, provType, model, aliasUsed string, in, out int) {
			if err := s.Repo.InsertRequestLog(context.Background(), db.RequestLog{
				UserID:       uid,
				AliasUsed:    aliasUsed,
				ProviderUsed: provType,
				ModelUsed:    model,
				InputTokens:  in,
				OutputTokens: out,
				StatusCode:   http.StatusOK,
			}); err != nil {
				log.Printf("proxy handler: insert request log error: %v", err)
			}
		}(userID, providerType, reqCopy.Model, currentModel, openAIResp.Usage.PromptTokens, openAIResp.Usage.CompletionTokens)

		return
	}

	if lastErr != nil {
		log.Printf("proxy handler: all fallbacks failed for user %d: %v", userID, lastErr)
		http.Error(w, "All fallbacks failed", http.StatusBadGateway)
	} else {
		log.Printf("proxy handler: max fallback depth reached for user %d", userID)
		http.Error(w, "Max fallback depth reached", http.StatusLoopDetected)
	}
}

func estimateTokens(messages []types.OpenAIMessage) int {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	// Simple heuristic: 4 characters per token
	if totalChars == 0 {
		return 0
	}
	tokens := totalChars / 4
	if tokens == 0 {
		return 1
	}
	return tokens
}
