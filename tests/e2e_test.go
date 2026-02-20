package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/crypto"
	"tokentracer-proxy/pkg/db"
	"tokentracer-proxy/pkg/handler"
	"tokentracer-proxy/pkg/types"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestProxyEndToEnd(t *testing.T) {
	// 1. Setup Mock Anthropic Server
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Header
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("Expected x-api-key header")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify Request Body (Translation correctness)
		var anthropicReq types.AnthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
			t.Errorf("Failed to decode upstream request: %v", err)
			return
		}

		if anthropicReq.Model != "claude-3-opus-20240229" {
			t.Errorf("Expected mapped model claude-3-opus-20240229, got %s", anthropicReq.Model)
		}

		if len(anthropicReq.Messages) == 0 || anthropicReq.Messages[0].Content != "Hello world" {
			t.Errorf("Incorrect message content")
		}

		// Send Mock Response
		resp := types.AnthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []types.AnthropicBlock{
				{Type: "text", Text: "Hello there!"},
			},
			Usage: types.AnthropicUsage{
				InputTokens:  10,
				OutputTokens: 20,
			},
			StopReason: "end_turn",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockAnthropic.Close()

	// 2. Setup Proxy Server
	// Set env vars
	os.Setenv("ANTHROPIC_API_KEY", "test-api-key")
	os.Setenv("ANTHROPIC_BASE_URL", mockAnthropic.URL)
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key-for-e2e")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("ANTHROPIC_BASE_URL")
	defer os.Unsetenv("ENCRYPTION_KEY")

	// Init crypto for test
	crypto.Init()

	// Init Mock DB
	mockDB, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()

	// Encrypt the test API key as it would be stored in the real DB
	encryptedTestKey, err := crypto.Encrypt("test-api-key")
	if err != nil {
		t.Fatalf("Failed to encrypt test key: %v", err)
	}

	repo := db.NewPostgresRepository(mockDB)
	ps := handler.NewProxyServer(repo)

	// Expect DB calls for ProxyHandler
	// 1. Model Alias
	mockDB.ExpectQuery("SELECT target_model, provider_key_id, fallback_alias_id, use_light_model, light_model_threshold, light_model FROM model_aliases").
		WithArgs(123, "gpt-4").
		WillReturnRows(mockDB.NewRows([]string{"target_model", "provider_key_id", "fallback_alias_id", "use_light_model", "light_model_threshold", "light_model"}).
			AddRow("claude-3-opus-20240229", 10, nil, false, 100, nil))

	// 2. Provider Key (Lookup for type)
	mockDB.ExpectQuery("SELECT provider, encrypted_key FROM provider_keys").
		WithArgs(10, 123).
		WillReturnRows(mockDB.NewRows([]string{"provider", "encrypted_key"}).AddRow("anthropic", encryptedTestKey))

	mockDB.ExpectQuery("SELECT provider, encrypted_key FROM provider_keys").
		WithArgs(10, 123).
		WillReturnRows(mockDB.NewRows([]string{"provider", "encrypted_key"}).AddRow("anthropic", encryptedTestKey))

	// 4. Request Logging (Async)
	mockDB.ExpectExec("INSERT INTO request_logs").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	r := chi.NewRouter()

	// Mock Auth Middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), auth.KeyUser, 123)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Post("/v1/chat/completions", ps.ProxyHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// 3. Send OpenAI Request to Proxy
	openAIReq := types.OpenAIRequest{
		Model: "gpt-4",
		Messages: []types.OpenAIMessage{
			{Role: "user", Content: "Hello world"},
		},
	}
	reqBody, _ := json.Marshal(openAIReq)

	res, err := http.Post(ts.URL+"/v1/chat/completions", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to call proxy: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", res.StatusCode)
	}

	// 4. Verify OpenAI Response
	var openAIResp types.OpenAIResponse
	if err := json.NewDecoder(res.Body).Decode(&openAIResp); err != nil {
		t.Fatalf("Failed to decode proxy response: %v", err)
	}

	if openAIResp.ID != "msg_123" {
		t.Errorf("Expected ID msg_123, got %s", openAIResp.ID)
	}
	if len(openAIResp.Choices) == 0 || openAIResp.Choices[0].Message.Content != "Hello there!" {
		t.Errorf("Incorrect response content")
	}
	if openAIResp.Usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", openAIResp.Usage.TotalTokens)
	}
}
