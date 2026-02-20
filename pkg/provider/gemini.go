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
	"tokentracer-proxy/pkg/types"
)

type GeminiProvider struct {
	repo          db.Repository
	providerKeyID int
	userID        int
	baseURL       string
}

func NewGeminiProvider(repository db.Repository, providerKeyID, userID int) *GeminiProvider {
	baseURL := os.Getenv("GEMINI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	}

	return &GeminiProvider{
		repo:          repository,
		providerKeyID: providerKeyID,
		userID:        userID,
		baseURL:       baseURL,
	}
}

func (p *GeminiProvider) Send(ctx context.Context, req types.OpenAIRequest) (*types.OpenAIResponse, error) {
	// 1. Fetch Key
	_, encryptedKey, err := p.repo.GetProviderKey(ctx, p.providerKeyID, p.userID)
	if err != nil {
		return nil, fmt.Errorf("provider configuration not found: %w", err)
	}

	// 2. Prepare Request
	reqBody, _ := json.Marshal(req)

	// 3. Send Request
	upstreamReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	apiKey, err := crypto.Decrypt(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt provider key: %w", err)
	}

	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

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
	var openAIResp types.OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("failed to decode upstream response: %w", err)
	}

	return &openAIResp, nil
}

func (p *GeminiProvider) ListModels(ctx context.Context) ([]string, error) {
	// 1. Fetch Key
	_, encryptedKey, err := p.repo.GetProviderKey(ctx, p.providerKeyID, p.userID)
	if err != nil {
		return nil, fmt.Errorf("provider configuration not found: %w", err)
	}

	// 2. Send Request
	upstreamReq, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}

	apiKey, err := crypto.Decrypt(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt provider key: %w", err)
	}

	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
