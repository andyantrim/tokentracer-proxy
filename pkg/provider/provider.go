package provider

import (
	"context"
	"tokentracer-proxy/pkg/types"
)

const (
	ProviderKeyAnthropic = "2"
	ProviderKeyOpenAI    = "1"
	ProviderKeyGemini    = "3"
)

type Provider interface {
	Send(ctx context.Context, req types.OpenAIRequest) (*types.OpenAIResponse, error)
	ListModels(ctx context.Context) ([]string, error)
}

func SupportedProviders() []string {
	return []string{"openai", "anthropic", "gemini"}
}
