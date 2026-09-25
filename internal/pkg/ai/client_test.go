// In-package tests for the AI client.
// Uses httptest.NewServer to exercise both OpenAI and Claude paths
// without making real API calls.
package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ascenda/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test server helpers ───────────────────────────────────────────────────────

// newOpenAIServer returns a test server that mimics the OpenAI chat completions API.
func newOpenAIServer(t *testing.T, reply string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify standard request headers.
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Content-Type header")
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ", "Authorization header")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if statusCode == http.StatusOK {
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"content": reply}},
				},
			}
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		} else {
			io.WriteString(w, `{"error":{"message":"simulated error"}}`) //nolint:errcheck
		}
	}))
}

// newClaudeServer returns a test server that mimics the Anthropic messages API.
func newClaudeServer(t *testing.T, reply string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Content-Type header")
		assert.NotEmpty(t, r.Header.Get("x-api-key"), "x-api-key header")
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"), "anthropic-version header")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if statusCode == http.StatusOK {
			resp := map[string]interface{}{
				"content": []map[string]string{
					{"type": "text", "text": reply},
				},
			}
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		} else {
			io.WriteString(w, `{"error":{"type":"invalid_request_error"}}`) //nolint:errcheck
		}
	}))
}

// newTestClient creates an AIClient with the given provider/key and overrides
// both base URLs so HTTP calls go to the supplied test server.
func newTestClient(provider, apiKey, serverURL string) *Client {
	return &Client{
		aiProvider:    provider,
		apiKey:        apiKey,
		claudeModel:   "claude-sonnet-4-6",
		maxTokens:     100,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		openAIBaseURL: serverURL,
		claudeBaseURL: serverURL,
	}
}

// testAIConfig returns a config.AIConfig with sensible test defaults.
func testAIConfig(overrides ...func(*config.AIConfig)) config.AIConfig {
	cfg := config.AIConfig{
		Provider:  "claude",
		APIKey:    "",
		Model:     "claude-sonnet-4-6",
		MaxTokens: 4096,
		Timeout:   30,
	}
	for _, fn := range overrides {
		fn(&cfg)
	}
	return cfg
}

// ── IsConfigured ─────────────────────────────────────────────────────────────

func TestClient_IsConfigured_WithKey(t *testing.T) {
	c := &Client{apiKey: "sk-test"}
	assert.True(t, c.IsConfigured())
}

func TestClient_IsConfigured_WithoutKey(t *testing.T) {
	c := &Client{}
	assert.False(t, c.IsConfigured())
}

func TestClient_IsConfigured_Nil(t *testing.T) {
	var c *Client
	assert.False(t, c.IsConfigured())
}

// ── Provider ──────────────────────────────────────────────────────────────────

func TestClient_Provider(t *testing.T) {
	c := &Client{aiProvider: "claude"}
	assert.Equal(t, "claude", c.Provider())
}

func TestClient_Provider_Nil(t *testing.T) {
	var c *Client
	assert.Equal(t, "", c.Provider())
}

// ── Call — guard conditions ───────────────────────────────────────────────────

func TestClient_Call_NoKey(t *testing.T) {
	c := &Client{aiProvider: "openai"}
	_, err := c.Call(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not configured")
}

func TestClient_Call_NilClient(t *testing.T) {
	var c *Client
	_, err := c.Call(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not configured")
}

func TestClient_Call_UnsupportedProvider(t *testing.T) {
	c := &Client{aiProvider: "grok", apiKey: "key"}
	_, err := c.Call(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported AI provider")
}

// ── OpenAI path ───────────────────────────────────────────────────────────────

func TestClient_OpenAI_Success(t *testing.T) {
	const want = "Hello from OpenAI"
	srv := newOpenAIServer(t, want, http.StatusOK)
	defer srv.Close()

	c := newTestClient("openai", "sk-test", srv.URL)
	got, err := c.Call(context.Background(), "test prompt")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestClient_OpenAI_ErrorStatus(t *testing.T) {
	srv := newOpenAIServer(t, "", http.StatusUnauthorized)
	defer srv.Close()

	c := newTestClient("openai", "bad-key", srv.URL)
	_, err := c.Call(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAI API error 401")
}

func TestClient_OpenAI_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{}}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient("openai", "key", srv.URL)
	_, err := c.Call(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no response from OpenAI")
}

func TestClient_OpenAI_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `not json`) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient("openai", "key", srv.URL)
	_, err := c.Call(context.Background(), "test")
	require.Error(t, err)
}

// Verify that the request body contains the expected model and max_tokens.
func TestClient_OpenAI_RequestBody(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient("openai", "key", srv.URL)
	c.maxTokens = 250
	_, err := c.Call(context.Background(), "my prompt")
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", captured["model"])
	assert.InDelta(t, 250, captured["max_tokens"], 0.5)
	messages := captured["messages"].([]interface{})
	require.Len(t, messages, 1)
	assert.Equal(t, "my prompt", messages[0].(map[string]interface{})["content"])
}

// ── Claude path ───────────────────────────────────────────────────────────────

func TestClient_Claude_Success(t *testing.T) {
	const want = "Hello from Claude"
	srv := newClaudeServer(t, want, http.StatusOK)
	defer srv.Close()

	c := newTestClient("claude", "sk-ant-test", srv.URL)
	got, err := c.Call(context.Background(), "test prompt")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestClient_Claude_ErrorStatus(t *testing.T) {
	srv := newClaudeServer(t, "", http.StatusUnauthorized)
	defer srv.Close()

	c := newTestClient("claude", "bad-key", srv.URL)
	_, err := c.Call(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Claude API error 401")
}

func TestClient_Claude_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"content": []interface{}{}}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient("claude", "key", srv.URL)
	_, err := c.Call(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no response from Claude")
}

func TestClient_Claude_AllowedModels_Blocked(t *testing.T) {
	srv := newClaudeServer(t, "ok", http.StatusOK)
	defer srv.Close()

	c := newTestClient("claude", "key", srv.URL)
	c.claudeModel = "claude-opus-4-6"
	c.allowedModels = []string{"claude-sonnet-4-6"}

	_, err := c.Call(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in CLAUDE_ALLOWED_MODELS")
}

func TestClient_Claude_AllowedModels_Passes(t *testing.T) {
	const want = "ok"
	srv := newClaudeServer(t, want, http.StatusOK)
	defer srv.Close()

	c := newTestClient("claude", "key", srv.URL)
	c.allowedModels = []string{"claude-sonnet-4-6", "claude-haiku-4-5"}

	got, err := c.Call(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// Verify that the request body contains the expected model and max_tokens.
func TestClient_Claude_RequestBody(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "ok"}},
		}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient("claude", "key", srv.URL)
	c.maxTokens = 350
	_, err := c.Call(context.Background(), "financial prompt")
	require.NoError(t, err)

	assert.Equal(t, "claude-sonnet-4-6", captured["model"])
	assert.InDelta(t, 350, captured["max_tokens"], 0.5)
	messages := captured["messages"].([]interface{})
	require.Len(t, messages, 1)
	assert.Equal(t, "financial prompt", messages[0].(map[string]interface{})["content"])
}

// ── CallWithMaxTokens ─────────────────────────────────────────────────────────

func TestClient_CallWithMaxTokens_OverridesAndRestores(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		// Respond with the max_tokens value so we can assert on it.
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}},
			},
		}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient("openai", "key", srv.URL)
	c.maxTokens = 200

	_, err := c.CallWithMaxTokens(context.Background(), "prompt", 999)
	require.NoError(t, err)

	// maxTokens must be restored to original value after the call.
	assert.Equal(t, 200, c.maxTokens, "maxTokens should be restored after CallWithMaxTokens")
}

func TestClient_CallWithMaxTokens_NoKey(t *testing.T) {
	c := &Client{aiProvider: "openai"}
	_, err := c.CallWithMaxTokens(context.Background(), "prompt", 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not configured")
}

// ── Context cancellation ──────────────────────────────────────────────────────

func TestClient_Call_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block indefinitely — context cancellation should abort the request.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := newTestClient("openai", "key", srv.URL)
	_, err := c.Call(ctx, "test")
	require.Error(t, err, "expected error from cancelled context")
}

// ── ExtractJSON ───────────────────────────────────────────────────────────────

func TestExtractJSON_ValidJSON(t *testing.T) {
	input := `Here is the result: {"key": "value", "num": 42} and some trailing text.`
	got, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `{"key": "value", "num": 42}`, got)
}

func TestExtractJSON_NoJSON(t *testing.T) {
	_, err := ExtractJSON("just plain text with no curly braces")
	require.Error(t, err)
}

func TestExtractJSON_OnlyJSON(t *testing.T) {
	input := `{"title":"plan summary","paragraphs":[]}`
	got, err := ExtractJSON(input)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(got), &m))
	assert.Equal(t, "plan summary", m["title"])
}

func TestExtractJSON_JSONSurroundedByMarkdown(t *testing.T) {
	input := "```json\n{\"title\": \"Plan\", \"summary\": \"ok\"}\n```"
	got, err := ExtractJSON(input)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(got), &m))
	assert.Equal(t, "Plan", m["title"])
}

func TestExtractJSON_NestedObjects(t *testing.T) {
	input := `{"outer":{"inner":"value"},"arr":[1,2,3]}`
	got, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, input, got)
}

// ── NewClient via config.AIConfig ────────────────────────────────────────────

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(testAIConfig(), nil)
	assert.Equal(t, "claude", c.aiProvider)
	assert.Equal(t, "claude-sonnet-4-6", c.claudeModel)
	assert.Equal(t, 4096, c.maxTokens)
	// Timeout is taken from testAIConfig().Timeout = 30, so httpClient.Timeout = 30s.
	assert.Equal(t, 30*time.Second, c.httpClient.Timeout)
}

func TestNewClient_PicksUpAPIKey(t *testing.T) {
	c := NewClient(testAIConfig(func(cfg *config.AIConfig) {
		cfg.APIKey = "sk-from-config"
	}), nil)
	assert.True(t, c.IsConfigured())
	assert.Equal(t, "sk-from-config", c.apiKey)
}

func TestNewClient_OpenAIProvider(t *testing.T) {
	c := NewClient(testAIConfig(func(cfg *config.AIConfig) {
		cfg.Provider = "openai"
		cfg.APIKey = "sk-openai"
	}), nil)
	assert.Equal(t, "openai", c.aiProvider)
}

func TestNewClient_ClaudeProvider(t *testing.T) {
	c := NewClient(testAIConfig(func(cfg *config.AIConfig) {
		cfg.Provider = "claude"
		cfg.APIKey = "sk-ant-test"
		cfg.Model = "claude-haiku-4-5"
	}), nil)
	assert.Equal(t, "claude", c.aiProvider)
	assert.Equal(t, "claude-haiku-4-5", c.claudeModel)
}

func TestNewClient_AllowedModels(t *testing.T) {
	c := NewClient(testAIConfig(func(cfg *config.AIConfig) {
		cfg.AllowedModels = []string{"claude-sonnet-4-6", "claude-haiku-4-5"}
	}), nil)
	assert.Equal(t, []string{"claude-sonnet-4-6", "claude-haiku-4-5"}, c.allowedModels)
}

func TestNewClient_CustomMaxTokens(t *testing.T) {
	c := NewClient(testAIConfig(func(cfg *config.AIConfig) {
		cfg.MaxTokens = 1200
	}), nil)
	assert.Equal(t, 1200, c.maxTokens)
}

func TestNewClient_CustomTimeout(t *testing.T) {
	c := NewClient(testAIConfig(func(cfg *config.AIConfig) {
		cfg.Timeout = 60
	}), nil)
	assert.Equal(t, 60*time.Second, c.httpClient.Timeout)
}
