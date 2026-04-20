// Package ai provides an HTTP client for calling external AI providers
// (OpenAI and Anthropic Claude).
//
// Configuration is injected via config.AIConfig — no environment variables
// are read inside this package.
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

// Client is an AI gateway client for OpenAI and Claude providers.
// It is safe for concurrent use.
type Client struct {
	aiProvider    string
	apiKey        string
	claudeModel   string
	allowedModels []string
	maxTokens     int
	log           *logrus.Entry

	// httpClient is reused across calls to take advantage of keep-alive
	// connections and reduce TLS handshake overhead.
	// Timeout is set to cfg.AI.Timeout (default 90 s).  This value must be
	// less than the HTTP server's WriteTimeout (150 s, set in bootstrap.go)
	// and greater than typical LLM inference time (~10–60 s for claude-sonnet).
	// The application-level aiRouteTimeout context (120 s) acts as the outer
	// hard deadline for any in-flight request.
	httpClient *http.Client

	openAIBaseURL string
	claudeBaseURL string
}

// NewClient creates a Client from Ascenda's centralised AIConfig.
// Pass a *logrus.Entry for startup diagnostics; pass nil to suppress logging.
func NewClient(cfg config.AIConfig, log *logrus.Entry) *Client {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 90 * time.Second // default: enough for typical LLM inference
	}
	c := &Client{
		aiProvider:    cfg.Provider,
		apiKey:        cfg.APIKey,
		claudeModel:   cfg.Model,
		allowedModels: cfg.AllowedModels,
		maxTokens:     cfg.MaxTokens,
		httpClient:    &http.Client{Timeout: timeout},
		log:           log,
		openAIBaseURL: "https://api.openai.com/v1/chat/completions",
		claudeBaseURL: "https://api.anthropic.com/v1/messages",
	}

	if log != nil {
		f := logrus.Fields{
			"component":  "ai-client",
			"provider":   cfg.Provider,
			"model":      cfg.Model,
			"max_tokens": cfg.MaxTokens,
			"timeout_s":  cfg.Timeout,
		}
		if cfg.APIKey != "" {
			f["api_key"] = "configured"
		} else {
			f["api_key"] = "not configured"
		}
		if len(cfg.AllowedModels) > 0 {
			f["allowed_models"] = strings.Join(cfg.AllowedModels, ", ")
		}
		log.WithFields(f).Info("AI client initialised")
	}
	return c
}

// ClientForTest creates a Client for unit testing. Both provider base URLs are
// pointed at serverURL so tests use an httptest.Server without real HTTP calls.
// Returns a value (not pointer) so callers can do: c := ClientForTest(...); svc{client: &c}.
func ClientForTest(provider, apiKey, serverURL string) Client {
	return Client{
		aiProvider:    provider,
		apiKey:        apiKey,
		claudeModel:   "claude-sonnet-4-6",
		maxTokens:     100,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		openAIBaseURL: serverURL,
		claudeBaseURL: serverURL,
	}
}

// IsConfigured returns true when an API key is present.
func (c *Client) IsConfigured() bool {
	return c != nil && c.apiKey != ""
}

// Provider returns the configured AI provider name.
func (c *Client) Provider() string {
	if c == nil {
		return ""
	}
	return c.aiProvider
}

// Call sends prompt to the configured AI provider and returns the text completion.
func (c *Client) Call(ctx context.Context, prompt string) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}
	switch c.aiProvider {
	case "openai":
		return c.callOpenAI(ctx, prompt, c.maxTokens)
	case "claude":
		return c.callClaude(ctx, prompt, c.maxTokens)
	default:
		return "", fmt.Errorf("unsupported AI provider: %q", c.aiProvider)
	}
}

// CallWithMaxTokens sends prompt with a custom token ceiling, temporarily
// overriding the client's default maxTokens for this call only.
func (c *Client) CallWithMaxTokens(ctx context.Context, prompt string, maxTokens int) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}
	original := c.maxTokens
	c.maxTokens = maxTokens
	defer func() { c.maxTokens = original }()
	return c.Call(ctx, prompt)
}

// ── Internal provider implementations ────────────────────────────────────────

func (c *Client) callOpenAI(ctx context.Context, prompt string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model":       "gpt-4",
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens":  maxTokens,
		"temperature": 0.7,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.openAIBaseURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Choices []struct {
			Message struct{ Content string `json:"content"` } `json:"message"`
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

func (c *Client) callClaude(ctx context.Context, prompt string, maxTokens int) (string, error) {
	if len(c.allowedModels) > 0 {
		allowed := false
		for _, m := range c.allowedModels {
			if m == c.claudeModel {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("model %q not in CLAUDE_ALLOWED_MODELS", c.claudeModel)
		}
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":      c.claudeModel,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens": maxTokens,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.claudeBaseURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Claude API error %d: %s", resp.StatusCode, b)
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

// ExtractJSON extracts the first complete JSON object from an AI response string
// that may contain surrounding prose or markdown code fences.
func ExtractJSON(response string) (string, error) {
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return "", fmt.Errorf("extractJSON: no valid JSON found in AI response")
	}
	return response[jsonStart : jsonEnd+1], nil
}
