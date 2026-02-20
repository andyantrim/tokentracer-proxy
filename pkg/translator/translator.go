package translator

import (
	"strings"
	"tokentracer-proxy/pkg/types"
)

// DefaultMaxTokens is used if no max_tokens is specified, as Anthropic requires this field.
const DefaultMaxTokens = 4096

// Map OpenAI models to Anthropic equivalents for the MVP
var ModelMap = map[string]string{
	"gpt-4":         "claude-3-opus-20240229",
	"gpt-4-turbo":   "claude-3-opus-20240229",
	"gpt-4o":        "claude-3-5-sonnet-20240620",
	"gpt-3.5-turbo": "claude-3-haiku-20240307",
}

func OpenAIToAnthropicRequest(req types.OpenAIRequest) (types.AnthropicRequest, error) {
	var anthropicReq types.AnthropicRequest

	// Map Model
	if mapped, ok := ModelMap[req.Model]; ok {
		anthropicReq.Model = mapped
	} else {
		// Fallback: use the requested model name if not in map (assuming user might pass actual Anthropic ID)
		anthropicReq.Model = req.Model
	}

	// Extract System Message and Convert Messages
	var messages []types.AnthropicMessage
	var systemPrompt string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt += msg.Content + "\n"
		} else {
			messages = append(messages, types.AnthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	anthropicReq.System = strings.TrimSpace(systemPrompt)
	anthropicReq.Messages = messages

	if req.MaxTokens > 0 {
		anthropicReq.MaxTokens = req.MaxTokens
	} else {
		anthropicReq.MaxTokens = DefaultMaxTokens
	}

	anthropicReq.Stream = req.Stream

	return anthropicReq, nil
}

func AnthropicToOpenAIResponse(resp types.AnthropicResponse) (types.OpenAIResponse, error) {
	var openAIResp types.OpenAIResponse

	openAIResp.ID = resp.ID
	openAIResp.Object = "chat.completion"
	openAIResp.Created = 0        // timestamp logic if needed, or 0
	openAIResp.Model = resp.Model // This isn't returned by Anthropic in the body usually, but let's leave it empty or fill from context if needed.

	// Helper to extract text content
	content := ""
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	openAIResp.Choices = []types.OpenAIChoice{
		{
			Index: 0,
			Message: types.OpenAIMessage{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: resp.StopReason,
		},
	}

	openAIResp.Usage = types.OpenAIUsage{
		PromptTokens:     resp.Usage.InputTokens,
		CompletionTokens: resp.Usage.OutputTokens,
		TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
	}

	return openAIResp, nil
}
