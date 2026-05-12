package ai_provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/exar/babycoder/internal/config"
)

// Provider defines the interface for AI providers
type Provider interface {
	ChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error)
	GetModel() string
}

// LMStudioProvider implements the Provider interface for LMStudio
type LMStudioProvider struct {
	endpoint    string
	model       string
	temperature float32
	apiKey      string
	httpClient  *http.Client
}

// NewLMStudioProvider creates a new LMStudio provider
func NewLMStudioProvider(configuration *config.AIProviderConfiguration) *LMStudioProvider {
	return &LMStudioProvider{
		endpoint:    configuration.Endpoint,
		model:       configuration.Model,
		temperature: configuration.Temperature,
		apiKey:      configuration.APIKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ChatCompletion sends a chat completion request to LMStudio
func (provider *LMStudioProvider) ChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Override model and temperature with provider defaults if not set
	if request.Model == "" {
		request.Model = provider.model
	}
	if request.Temperature == 0 {
		request.Temperature = provider.temperature
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", provider.endpoint)
	httpRequest, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpRequest.Header.Set("Content-Type", "application/json")
	if provider.apiKey != "" {
		httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.apiKey))
	}

	// Send request
	httpResponse, err := provider.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to LMStudio: %w", err)
	}
	defer httpResponse.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if httpResponse.StatusCode != http.StatusOK {
		var errorResponse ErrorResponse
		if err := json.Unmarshal(responseBody, &errorResponse); err != nil {
			return nil, fmt.Errorf("HTTP error %d: %s", httpResponse.StatusCode, string(responseBody))
		}
		return nil, fmt.Errorf("LMStudio API error: %s (type: %s, code: %s)",
			errorResponse.Error.Message,
			errorResponse.Error.Type,
			errorResponse.Error.Code)
	}

	// Parse response
	var response ChatCompletionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// GetModel returns the model name
func (provider *LMStudioProvider) GetModel() string {
	return provider.model
}

// NewProvider creates a new AI provider based on configuration
func NewProvider(configuration *config.AIProviderConfiguration) (Provider, error) {
	switch configuration.Type {
	case "lmstudio":
		return NewLMStudioProvider(configuration), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", configuration.Type)
	}
}
