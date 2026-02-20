package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/db"
	"tokentracer-proxy/pkg/handler"
	"tokentracer-proxy/pkg/provider"
	"tokentracer-proxy/pkg/types"

	"github.com/pashagolub/pgxmock/v4"
)

// MockProvider implements provider.Provider
type MockProvider struct {
	Response *types.OpenAIResponse
	Err      error
}

func (m *MockProvider) Send(ctx context.Context, req types.OpenAIRequest) (*types.OpenAIResponse, error) {
	return m.Response, m.Err
}

func (m *MockProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{"mock-model"}, nil
}

func TestProxyHandler_Anthropic(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mockDB.Close()

	repo := db.NewPostgresRepository(mockDB)
	ps := handler.NewProxyServer(repo)

	originalAnthropicFactory := handler.AnthropicProviderFactory
	defer func() {
		handler.AnthropicProviderFactory = originalAnthropicFactory
	}()

	// Mock Factory
	mockProv := &MockProvider{
		Response: &types.OpenAIResponse{
			ID: "mock-id",
			Choices: []types.OpenAIChoice{
				{Message: types.OpenAIMessage{Content: "Mock response"}},
			},
			Usage: types.OpenAIUsage{PromptTokens: 10, CompletionTokens: 20},
		},
	}
	handler.AnthropicProviderFactory = func(r db.Repository, k, u int) provider.Provider {
		return mockProv
	}

	// Test Data
	userID := 123
	reqBody := types.OpenAIRequest{
		Model: "my-alias",
		Messages: []types.OpenAIMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Expectations
	// 1. Lookup Model Alias
	mockDB.ExpectQuery("SELECT target_model, provider_key_id, fallback_alias_id, use_light_model, light_model_threshold, light_model FROM model_aliases").
		WithArgs(userID, "my-alias").
		WillReturnRows(mockDB.NewRows([]string{"target_model", "provider_key_id", "fallback_alias_id", "use_light_model", "light_model_threshold", "light_model"}).
			AddRow("claude-3-opus", 55, nil, false, 100, nil))

	// 2. Fetch Provider Type
	mockDB.ExpectQuery("SELECT provider, encrypted_key FROM provider_keys").
		WithArgs(55, userID).
		WillReturnRows(mockDB.NewRows([]string{"provider", "encrypted_key"}).AddRow("anthropic", "fake-key"))

	// 3. Async Logging
	mockDB.ExpectExec("INSERT INTO request_logs").
		WithArgs(userID, "my-alias", "anthropic", "claude-3-opus", 10, 20, 200).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Request
	req := httptest.NewRequest("POST", "/chat/completions", bytes.NewBuffer(bodyBytes))
	// Inject User Context
	ctx := context.WithValue(req.Context(), auth.KeyUser, userID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	ps.ProxyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Wait a bit for async goroutine
	time.Sleep(20 * time.Millisecond)

	if err := mockDB.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
