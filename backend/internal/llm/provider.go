package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderXAI      ProviderType = "xai"
	ProviderQrok     ProviderType = "qrok"
	ProviderOllama   ProviderType = "ollama"
	ProviderLlamaCpp ProviderType = "llama.cpp"
	ProviderDeepSeek ProviderType = "deepseek"
	ProviderMistral  ProviderType = "mistral"
	ProviderCohere   ProviderType = "cohere"
	ProviderOpenRouter ProviderType = "openrouter"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	Content      string `json:"content"`
	Usage        Usage  `json:"usage"`
	FinishReason string `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Delta        string `json:"delta"`
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
}

// Config holds provider configuration
type Config struct {
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	MaxRetries int    `json:"max_retries"`
	Timeout    int    `json:"timeout"`
}

// Provider interface for all LLM providers
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	StreamChat(req ChatRequest, onChunk func(StreamChunk)) error
	GetModels() ([]string, error)
	GetProviderType() ProviderType
}

// RateLimiter for API calls
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

var globalRateLimiter = &RateLimiter{
	limiters: make(map[string]*rate.Limiter),
}

func (r *RateLimiter) GetLimiter(identifier string, rps float64, burst int) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limiter, exists := r.limiters[identifier]; exists {
		return limiter
	}
	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	r.limiters[identifier] = limiter
	return limiter
}

func (r *RateLimiter) Wait(ctx context.Context, identifier string, rps float64, burst int) error {
	return r.GetLimiter(identifier, rps, burst).Wait(ctx)
}

// ============ OpenAI Provider ============

type OpenAIProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewOpenAIProvider(config Config) (*OpenAIProvider, error) {
	timeout := 60
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &OpenAIProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	baseURL := "https://api.openai.com/v1"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	openAIReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		openAIReq["max_tokens"] = req.MaxTokens
	}

	jsonData, _ := json.Marshal(openAIReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/chat/completions", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(body, &openAIResp)

	content := ""
	finishReason := ""
	if len(openAIResp.Choices) > 0 {
		content = openAIResp.Choices[0].Message.Content
		finishReason = openAIResp.Choices[0].FinishReason
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "openai",
		Content:      content,
		Usage:        Usage{InputTokens: openAIResp.Usage.PromptTokens, OutputTokens: openAIResp.Usage.CompletionTokens, TotalTokens: openAIResp.Usage.TotalTokens},
		FinishReason: finishReason,
	}, nil
}

func (p *OpenAIProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *OpenAIProvider) GetModels() ([]string, error) {
	return []string{"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"}, nil
}

func (p *OpenAIProvider) GetProviderType() ProviderType { return ProviderOpenAI }

// ============ Anthropic Provider ============

type AnthropicProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewAnthropicProvider(config Config) (*AnthropicProvider, error) {
	timeout := 60
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &AnthropicProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	model := req.Model
	if model == "" || !strings.HasPrefix(model, "claude-") {
		model = "claude-3-5-sonnet-20241022"
	}

	anthropicReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"max_tokens":  4096,
	}
	if req.Temperature > 0 {
		anthropicReq["temperature"] = req.Temperature
	}

	jsonData, _ := json.Marshal(anthropicReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.anthropic.com/v1/messages", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Anthropic API error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var anthroResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		StopReason string `json:"stop_reason"`
	}
	json.Unmarshal(body, &anthroResp)

	content := ""
	if len(anthroResp.Content) > 0 {
		content = anthroResp.Content[0].Text
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "anthropic",
		Content:      content,
		Usage:        Usage{InputTokens: anthroResp.Usage.InputTokens, OutputTokens: anthroResp.Usage.OutputTokens, TotalTokens: anthroResp.Usage.InputTokens + anthroResp.Usage.OutputTokens},
		FinishReason: anthroResp.StopReason,
	}, nil
}

func (p *AnthropicProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *AnthropicProvider) GetModels() ([]string, error) {
	return []string{"claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022", "claude-3-opus-20240229"}, nil
}

func (p *AnthropicProvider) GetProviderType() ProviderType { return ProviderAnthropic }

// ============ XAI (Grok) Provider ============

type XAIProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewXAIProvider(config Config) (*XAIProvider, error) {
	timeout := 60
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &XAIProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *XAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	baseURL := "https://api.x.ai/v1"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	model := req.Model
	if model == "" {
		model = "grok-beta"
	}

	openAIReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		openAIReq["max_tokens"] = req.MaxTokens
	}

	jsonData, _ := json.Marshal(openAIReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/chat/completions", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("XAI API error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(body, &openAIResp)

	content := ""
	if len(openAIResp.Choices) > 0 {
		content = openAIResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "xai",
		Content:      content,
		Usage:        Usage{InputTokens: openAIResp.Usage.PromptTokens, OutputTokens: openAIResp.Usage.CompletionTokens, TotalTokens: openAIResp.Usage.TotalTokens},
		FinishReason: openAIResp.Choices[0].FinishReason,
	}, nil
}

func (p *XAIProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *XAIProvider) GetModels() ([]string, error) { return []string{"grok-beta", "grok-vision-beta"}, nil }
func (p *XAIProvider) GetProviderType() ProviderType { return ProviderXAI }

// ============ Ollama Provider ============

type OllamaProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewOllamaProvider(config Config) (*OllamaProvider, error) {
	timeout := 120
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &OllamaProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	model := req.Model
	if model == "" {
		model = "llama2"
	}

	ollamaReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"stream":      false,
	}

	jsonData, _ := json.Marshal(ollamaReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", p.getOllamaURL()+"/api/chat", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Ollama API error: %w", err)
	}
	defer resp.Body.Close()

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&ollamaResp)

	return &ChatResponse{
		Model:        model,
		Provider:     "ollama",
		Content:      ollamaResp.Message.Content,
		Usage:        Usage{},
		FinishReason: "stop",
	}, nil
}

func (p *OllamaProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *OllamaProvider) GetModels() ([]string, error) {
	baseURL := "http://localhost:11434"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}
	resp, err := p.client.Get(baseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	json.NewDecoder(resp.Body).Decode(&tags)

	models := make([]string, len(tags.Models))
	for i, m := range tags.Models {
		models[i] = m.Name
	}
	return models, nil
}

func (p *OllamaProvider) GetProviderType() ProviderType { return ProviderOllama }

func (p *OllamaProvider) getOllamaURL() string {
	if p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "http://localhost:11434"
}

// ============ Llama.cpp Provider ============

type LlamaCppProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewLlamaCppProvider(config Config) (*LlamaCppProvider, error) {
	timeout := 180
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &LlamaCppProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *LlamaCppProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	baseURL := "http://localhost:8080"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	model := req.Model
	if model == "" {
		model = "llama-3.1-8b-instruct"
	}

	openAIReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}

	jsonData, _ := json.Marshal(openAIReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/chat/completions", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llama.cpp API error: %w", err)
	}
	defer resp.Body.Close()

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.NewDecoder(resp.Body).Decode(&openAIResp)

	content := ""
	if len(openAIResp.Choices) > 0 {
		content = openAIResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "llama.cpp",
		Content:      content,
		Usage:        Usage{InputTokens: openAIResp.Usage.PromptTokens, OutputTokens: openAIResp.Usage.CompletionTokens, TotalTokens: openAIResp.Usage.TotalTokens},
		FinishReason: openAIResp.Choices[0].FinishReason,
	}, nil
}

func (p *LlamaCppProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *LlamaCppProvider) GetModels() ([]string, error) {
	return []string{"llama-3.1-8b-instruct", "llama-3.1-70b-instruct", "mistral-7b-instruct"}, nil
}

func (p *LlamaCppProvider) GetProviderType() ProviderType { return ProviderLlamaCpp }

// ============ DeepSeek Provider ============

type DeepSeekProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewDeepSeekProvider(config Config) (*DeepSeekProvider, error) {
	timeout := 60
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &DeepSeekProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *DeepSeekProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	baseURL := "https://api.deepseek.com"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	model := req.Model
	if model == "" {
		model = "deepseek-chat"
	}

	openAIReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}

	jsonData, _ := json.Marshal(openAIReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/chat/completions", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek API error: %w", err)
	}
	defer resp.Body.Close()

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.NewDecoder(resp.Body).Decode(&openAIResp)

	content := ""
	if len(openAIResp.Choices) > 0 {
		content = openAIResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "deepseek",
		Content:      content,
		Usage:        Usage{InputTokens: openAIResp.Usage.PromptTokens, OutputTokens: openAIResp.Usage.CompletionTokens, TotalTokens: openAIResp.Usage.TotalTokens},
		FinishReason: openAIResp.Choices[0].FinishReason,
	}, nil
}

func (p *DeepSeekProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *DeepSeekProvider) GetModels() ([]string, error) { return []string{"deepseek-chat", "deepseek-coder"}, nil }
func (p *DeepSeekProvider) GetProviderType() ProviderType { return ProviderDeepSeek }

// ============ Mistral Provider ============

type MistralProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewMistralProvider(config Config) (*MistralProvider, error) {
	timeout := 60
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &MistralProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *MistralProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	baseURL := "https://api.mistral.ai"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	model := req.Model
	if model == "" {
		model = "mistral-large-latest"
	}

	openAIReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}

	jsonData, _ := json.Marshal(openAIReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/chat/completions", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Mistral API error: %w", err)
	}
	defer resp.Body.Close()

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.NewDecoder(resp.Body).Decode(&openAIResp)

	content := ""
	if len(openAIResp.Choices) > 0 {
		content = openAIResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "mistral",
		Content:      content,
		Usage:        Usage{InputTokens: openAIResp.Usage.PromptTokens, OutputTokens: openAIResp.Usage.CompletionTokens, TotalTokens: openAIResp.Usage.TotalTokens},
		FinishReason: openAIResp.Choices[0].FinishReason,
	}, nil
}

func (p *MistralProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *MistralProvider) GetModels() ([]string, error) { return []string{"mistral-large-latest", "mistral-medium", "mistral-small"}, nil }
func (p *MistralProvider) GetProviderType() ProviderType { return ProviderMistral }

// ============ Cohere Provider ============

type CohereProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewCohereProvider(config Config) (*CohereProvider, error) {
	timeout := 60
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &CohereProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *CohereProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	baseURL := "https://api.cohere.ai"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	model := req.Model
	if model == "" {
		model = "command-r-plus"
	}

	openAIReq := map[string]interface{}{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}

	jsonData, _ := json.Marshal(openAIReq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/chat/completions", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Cohere API error: %w", err)
	}
	defer resp.Body.Close()

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.NewDecoder(resp.Body).Decode(&openAIResp)

	content := ""
	if len(openAIResp.Choices) > 0 {
		content = openAIResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "cohere",
		Content:      content,
		Usage:        Usage{InputTokens: openAIResp.Usage.PromptTokens, OutputTokens: openAIResp.Usage.CompletionTokens, TotalTokens: openAIResp.Usage.TotalTokens},
		FinishReason: openAIResp.Choices[0].FinishReason,
	}, nil
}

func (p *CohereProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *CohereProvider) GetModels() ([]string, error) { return []string{"command-r-plus", "command-r", "command"}, nil }
func (p *CohereProvider) GetProviderType() ProviderType { return ProviderCohere }

// ============ OpenRouter Provider ============

type OpenRouterProvider struct {
	config    Config
	rateLimit *RateLimiter
	client    *http.Client
}

func NewOpenRouterProvider(config Config) (*OpenRouterProvider, error) {
	timeout := 120
	if config.Timeout > 0 {
		timeout = config.Timeout
	}
	return &OpenRouterProvider{
		config:    config,
		rateLimit: globalRateLimiter,
		client:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *OpenRouterProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	baseURL := "https://openrouter.ai/api/v1"
	if p.config.BaseURL != "" {
		baseURL = p.config.BaseURL
	}

	model := req.Model
	if model == "" {
		model = "openrouter/free" // Default to free models
	}

	// Convert messages to OpenRouter format
	openRouterMessages := make([]map[string]string, len(req.Messages))
	for i, msg := range req.Messages {
		openRouterMessages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	openAIRreq := map[string]interface{}{
		"model":       model,
		"messages":    openRouterMessages,
		"temperature": req.Temperature,
	}

	if req.MaxTokens > 0 {
		openAIRreq["max_tokens"] = req.MaxTokens
	}

	jsonData, _ := json.Marshal(openAIRreq)
	httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/chat/completions", strings.NewReader(string(jsonData)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	httpReq.Header.Set("HTTP-Referer", "https://ace-framework.dev")
	httpReq.Header.Set("X-Title", "ACE Framework")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(body, &openAIResp)

	content := ""
	finishReason := ""
	if len(openAIResp.Choices) > 0 {
		content = openAIResp.Choices[0].Message.Content
		finishReason = openAIResp.Choices[0].FinishReason
	}

	return &ChatResponse{
		Model:        model,
		Provider:     "openrouter",
		Content:      content,
		Usage:        Usage{InputTokens: openAIResp.Usage.PromptTokens, OutputTokens: openAIResp.Usage.CompletionTokens, TotalTokens: openAIResp.Usage.TotalTokens},
		FinishReason: finishReason,
	}, nil
}

func (p *OpenRouterProvider) StreamChat(req ChatRequest, onChunk func(StreamChunk)) error {
	return fmt.Errorf("streaming not yet implemented")
}

func (p *OpenRouterProvider) GetModels() ([]string, error) {
	// Return common free models available on OpenRouter
	return []string{
		"openrouter/free",
		"google/gemma-3n-e4b-it",
		"qwen/qwen3-8b",
		"deepseek/deepseek-chat",
		"mistralai/mistral-7b-instruct",
		"meta-llama/llama-3-8b-instruct",
	}, nil
}

func (p *OpenRouterProvider) GetProviderType() ProviderType { return ProviderOpenRouter }

// ============ Provider Factory ============

// NewProvider creates a new LLM provider based on type and config
func NewProvider(providerType ProviderType, config Config) (interface{}, error) {
	switch providerType {
	case ProviderOpenAI:
		return NewOpenAIProvider(config)
	case ProviderAnthropic:
		return NewAnthropicProvider(config)
	case ProviderXAI:
		return NewXAIProvider(config)
	case ProviderOllama:
		return NewOllamaProvider(config)
	case ProviderLlamaCpp:
		return NewLlamaCppProvider(config)
	case ProviderDeepSeek:
		return NewDeepSeekProvider(config)
	case ProviderMistral:
		return NewMistralProvider(config)
	case ProviderCohere:
		return NewCohereProvider(config)
	case ProviderOpenRouter:
		return NewOpenRouterProvider(config)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// ============ Provider Manager ============

type ProviderManager struct {
	mu        sync.RWMutex
	providers map[string]interface{}
	defaults  map[ProviderType]string
}

func NewProviderManager() *ProviderManager {
	pm := &ProviderManager{
		providers: make(map[string]interface{}),
		defaults: map[ProviderType]string{
			ProviderOpenAI:     "gpt-4o",
			ProviderAnthropic: "claude-3-5-sonnet-20241022",
			ProviderXAI:       "grok-beta",
			ProviderOllama:    "llama2",
			ProviderLlamaCpp:  "llama-3.1-8b-instruct",
			ProviderDeepSeek:  "deepseek-chat",
			ProviderMistral:   "mistral-large-latest",
			ProviderCohere:    "command-r-plus",
			ProviderOpenRouter: "openrouter/free",
		},
	}
	return pm
}

func (pm *ProviderManager) GetDefaultModel(providerType ProviderType) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if model, ok := pm.defaults[providerType]; ok {
		return model
	}
	return ""
}

func (pm *ProviderManager) GetSupportedProviders() []ProviderType {
	return []ProviderType{
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderXAI,
		ProviderOllama,
		ProviderLlamaCpp,
		ProviderDeepSeek,
		ProviderMistral,
		ProviderCohere,
		ProviderOpenRouter,
	}
}
