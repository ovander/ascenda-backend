// Package ai provides a shared HTTP client for calling external AI providers.
// Ported from GPWA internal/service/ai_client.go and adapted for Ascenda.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"ascenda/internal/config"
)

// Client is a shared client for calling external AI providers (OpenAI / Claude).
// Configuration is supplied via config.AIConfig — all env-var reading is
// centralised in config.Load() so the .env file is parsed exactly once.
type Client struct {
	aiProvider    string
	apiKey        string
	claudeModel   string
	allowedModels []string
	maxTokens     int
	httpTimeout   time.Duration
	log           *logrus.Entry

	// Base URLs — defaulted to the real provider endpoints.
	// Overridden in tests to point at httptest servers.
	openAIBaseURL string
	claudeBaseURL string
}

// NewClient creates a Client from the centralised AIConfig.
// Pass a *logrus.Entry for startup diagnostics; pass nil to suppress logging.
func NewClient(cfg config.AIConfig, log *logrus.Entry) *Client {
	c := &Client{
		aiProvider:    cfg.Provider,
		apiKey:        cfg.APIKey,
		claudeModel:   cfg.Model,
		allowedModels: cfg.AllowedModels,
		maxTokens:     cfg.MaxTokens,
		httpTimeout:   time.Duration(cfg.Timeout) * time.Second,
		log:           log,
		openAIBaseURL: "https://api.openai.com/v1/chat/completions",
		claudeBaseURL: "https://api.anthropic.com/v1/messages",
	}

	if log != nil {
		fields := logrus.Fields{
			"component":  "ai-client",
			"provider":   cfg.Provider,
			"model":      cfg.Model,
			"max_tokens": cfg.MaxTokens,
			"timeout_s":  cfg.Timeout,
		}
		if cfg.APIKey != "" {
			fields["api_key"] = "configured"
		} else {
			fields["api_key"] = "not configured"
		}
		if len(cfg.AllowedModels) > 0 {
			fields["allowed_models"] = strings.Join(cfg.AllowedModels, ", ")
		}
		log.WithFields(fields).Info("AI client initialised")
	}

	return c
}

// IsConfigured returns true when an API key is present.
func (c *Client) IsConfigured() bool {
	return c != nil && c.apiKey != ""
}

// Provider returns the configured AI provider name ("openai" or "claude").
func (c *Client) Provider() string {
	if c == nil {
		return ""
	}
	return c.aiProvider
}

// Call sends a prompt to the configured AI provider and returns the text response.
func (c *Client) Call(ctx context.Context, prompt string) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", fmt.Errorf("AI API key not configured")
	}

	switch c.aiProvider {
	case "openai":
		return c.callOpenAI(ctx, prompt)
	case "claude":
		return c.callClaude(ctx, prompt)
	default:
		return "", fmt.Errorf("unsupported AI provider: %s", c.aiProvider)
	}
}

// CallWithMaxTokens sends a prompt with a custom token limit.
// The configured default is restored after the call.
func (c *Client) CallWithMaxTokens(ctx context.Context, prompt string, maxTokens int) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", fmt.Errorf("AI API key not configured")
	}

	saved := c.maxTokens
	c.maxTokens = maxTokens
	defer func() { c.maxTokens = saved }()

	return c.Call(ctx, prompt)
}

// ── Internal provider implementations ────────────────────────────────────────

func (c *Client) callOpenAI(ctx context.Context, prompt string) (string, error) {
	url := c.openAIBaseURL

	requestBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  c.maxTokens,
		"temperature": 0.7,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := (&http.Client{Timeout: c.httpTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode openai response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}
	return result.Choices[0].Message.Content, nil
}

func (c *Client) callClaude(ctx context.Context, prompt string) (string, error) {
	// Validate model against allow-list when configured.
	if len(c.allowedModels) > 0 {
		allowed := false
		for _, m := range c.allowedModels {
			if m == c.claudeModel {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("claude model %q is not in CLAUDE_ALLOWED_MODELS", c.claudeModel)
		}
	}

	url := c.claudeBaseURL

	requestBody := map[string]interface{}{
		"model": c.claudeModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": c.maxTokens,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal claude request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := (&http.Client{Timeout: c.httpTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Claude API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode claude response: %w", err)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from Claude")
	}
	return result.Content[0].Text, nil
}

// ── Test helpers ──────────────────────────────────────────────────────────────

// ClientForTest creates a Client configured for unit testing.
// It points both openAIBaseURL and claudeBaseURL at serverURL so tests can
// use an httptest.Server without making real HTTP calls.
// Pass an empty apiKey to create an unconfigured client (IsConfigured → false).
func ClientForTest(provider, apiKey, serverURL string) Client {
	return Client{
		aiProvider:    provider,
		apiKey:        apiKey,
		claudeModel:   "claude-sonnet-4-6",
		maxTokens:     100,
		httpTimeout:   5 * time.Second,
		openAIBaseURL: serverURL,
		claudeBaseURL: serverURL,
	}
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// ExtractJSON extracts the first complete JSON object from an AI response string
// that may contain surrounding prose or markdown.
func ExtractJSON(response string) (string, error) {
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return "", fmt.Errorf("no valid JSON found in AI response")
	}
	return response[jsonStart : jsonEnd+1], nil
}
