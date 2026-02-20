package translator

import (
	"reflect"
	"testing"
	"tokentracer-proxy/pkg/types"
)

func TestOpenAIToAnthropicRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     types.OpenAIRequest
		want    types.AnthropicRequest
		wantErr bool
	}{
		{
			name: "Map GPT-4 to Opus",
			req: types.OpenAIRequest{
				Model: "gpt-4",
				Messages: []types.OpenAIMessage{
					{Role: "user", Content: "Hello"},
				},
			},
			want: types.AnthropicRequest{
				Model:     "claude-3-opus-20240229",
				MaxTokens: DefaultMaxTokens,
				Messages: []types.AnthropicMessage{
					{Role: "user", Content: "Hello"},
				},
				System: "",
			},
			wantErr: false,
		},
		{
			name: "System Message Extraction",
			req: types.OpenAIRequest{
				Model: "gpt-4",
				Messages: []types.OpenAIMessage{
					{Role: "system", Content: "Be helpful"},
					{Role: "user", Content: "Hello"},
				},
			},
			want: types.AnthropicRequest{
				Model:     "claude-3-opus-20240229",
				MaxTokens: DefaultMaxTokens,
				Messages: []types.AnthropicMessage{
					{Role: "user", Content: "Hello"},
				},
				System: "Be helpful",
			},
			wantErr: false,
		},
		{
			name: "Pass through unknown model",
			req: types.OpenAIRequest{
				Model: "claude-3-unknown",
				Messages: []types.OpenAIMessage{
					{Role: "user", Content: "Hello"},
				},
			},
			want: types.AnthropicRequest{
				Model:     "claude-3-unknown",
				MaxTokens: DefaultMaxTokens,
				Messages: []types.AnthropicMessage{
					{Role: "user", Content: "Hello"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OpenAIToAnthropicRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenAIToAnthropicRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OpenAIToAnthropicRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnthropicToOpenAIResponse(t *testing.T) {
	resp := types.AnthropicResponse{
		ID:    "msg_123",
		Model: "claude-3",
		Content: []types.AnthropicBlock{
			{Type: "text", Text: "Hello World"},
		},
		StopReason: "end_turn",
		Usage: types.AnthropicUsage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}

	got, err := AnthropicToOpenAIResponse(resp)
	if err != nil {
		t.Fatalf("AnthropicToOpenAIResponse() error = %v", err)
	}

	if got.ID != "msg_123" {
		t.Errorf("ID mismatch: got %v", got.ID)
	}
	if len(got.Choices) != 1 {
		t.Errorf("Choices length: got %d", len(got.Choices))
	}
	if got.Choices[0].Message.Content != "Hello World" {
		t.Errorf("Content mismatch: got %v", got.Choices[0].Message.Content)
	}
	if got.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens mismatch: got %d", got.Usage.TotalTokens)
	}
}
