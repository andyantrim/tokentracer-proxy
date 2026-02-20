package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"tokentracer-proxy/pkg/crypto"
	"tokentracer-proxy/pkg/db"
	"tokentracer-proxy/pkg/translator"
	"tokentracer-proxy/pkg/types"
)

type AnthropicProvider struct {
	repo          db.Repository
	providerKeyID int
	userID        int
	baseURL       string
}

func NewAnthropicProvider(repository db.Repository, providerKeyID, userID int) *AnthropicProvider {
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	return &AnthropicProvider{
		repo:          repository,
		providerKeyID: providerKeyID,
		userID:        userID,
		baseURL:       baseURL,
	}
}

func (p *AnthropicProvider) Send(ctx context.Context, req types.OpenAIRequest) (*types.OpenAIResponse, error) {
	// 1. Fetch Key
	_, encryptedKey, err := p.repo.GetProviderKey(ctx, p.providerKeyID, p.userID)
	if err != nil {
		return nil, fmt.Errorf("provider configuration not found: %w", err)
	}

	// 2. Translate Request
	anthropicReq, err := translator.OpenAIToAnthropicRequest(req)
	if err != nil {
		return nil, fmt.Errorf("translation error: %w", err)
	}
	reqBody, _ := json.Marshal(anthropicReq)

	// 3. Send Request
	upstreamReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	apiKey, err := crypto.Decrypt(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt provider key: %w", err)
	}

	upstreamReq.Header.Set("x-api-key", apiKey)
	upstreamReq.Header.Set("anthropic-version", "2023-06-01")
	upstreamReq.Header.Set("content-type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream error: status %d", resp.StatusCode)
	}

	// 4. Handle Response
	var anthropicResp types.AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode upstream response: %w", err)
	}

	openAIResp, err := translator.AnthropicToOpenAIResponse(anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("response translation error: %w", err)
	}

	return &openAIResp, nil
}
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]string, error) {
	// Anthropic recently added a models API: https://docs.anthropic.com/en/api/models-list
	// 1. Fetch Key
	_, encryptedKey, err := p.repo.GetProviderKey(ctx, p.providerKeyID, p.userID)
	if err != nil {
		return nil, fmt.Errorf("provider configuration not found: %w", err)
	}

	// 2. Send Request
	upstreamReq, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	apiKey, err := crypto.Decrypt(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt provider key: %w", err)
	}

	upstreamReq.Header.Set("x-api-key", apiKey)
	upstreamReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If 404, fallback to hardcoded list as it might be an older API version or proxy
		if resp.StatusCode == http.StatusNotFound {
			return []string{"claude-3-5-sonnet-20240620", "claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"}, nil
		}
		return nil, fmt.Errorf("upstream error: status %d", resp.StatusCode)
	}

	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode upstream response: %w", err)
	}

	var models []string
	for _, m := range data.Data {
		models = append(models, m.ID)
	}
	return models, nil
}
