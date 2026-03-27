// Package socrate provides a client for the Socrate API (backend-to-backend)
package socrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ContextKey for storing JWT in context
type contextKey string

const JWTContextKey contextKey = "socrate_jwt"

// ErrUserAlreadyExists is returned when trying to create a user that already exists in Socrate
var ErrUserAlreadyExists = errors.New("user already exists in Socrate")

// Client is the Socrate API client that forwards the admin's JWT
type Client struct {
	baseURL      string
	clientID     string // OAuth client ID (used to resolve numeric app ID)
	clientSecret string // OAuth client secret (used for client_credentials token exchange)

	resolvedAppID string // Cached numeric app ID (resolved on first request)
	httpClient    *http.Client

	// Service-account token cache (client_credentials grant).
	svcTokenMu     sync.Mutex
	svcToken       string
	svcTokenExpiry time.Time
}

// ClientConfig holds configuration for the Socrate client
type ClientConfig struct {
	BaseURL      string
	ClientID     string // OAuth client ID - will be resolved to numeric app ID
	ClientSecret string // OAuth client secret - used for backend-to-backend calls
	Timeout      time.Duration
}

// NewClient creates a new Socrate API client
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("socrate: base URL is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("socrate: client ID is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL:      cfg.BaseURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient:   &http.Client{Timeout: timeout},
	}, nil
}

// WithJWT adds the JWT to the context for API calls
func WithJWT(ctx context.Context, jwt string) context.Context {
	return context.WithValue(ctx, JWTContextKey, jwt)
}

// getJWT extracts JWT from context
func getJWT(ctx context.Context) (string, error) {
	jwt, ok := ctx.Value(JWTContextKey).(string)
	if !ok || jwt == "" {
		return "", errors.New("no JWT in context - admin must be authenticated")
	}
	return jwt, nil
}

// doRequest performs a request to Socrate API, forwarding the admin's JWT
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	jwt, err := getJWT(ctx)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Forward the admin's JWT to Socrate
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// ============================================================================
// App ID Resolution
// ============================================================================

// App represents an application in Socrate
type App struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	ClientID string `json:"client_id"`
}

// AppListResponse is the response from listing apps
type AppListResponse struct {
	Apps []App `json:"apps"`
}

// getAppID returns the numeric app ID, resolving it from client ID on first call
func (c *Client) getAppID(ctx context.Context) (string, error) {
	// Return cached value if available
	if c.resolvedAppID != "" {
		return c.resolvedAppID, nil
	}

	// Resolve numeric app ID from client ID via Admin API
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/admin/apps", nil)
	if err != nil {
		return "", fmt.Errorf("fetch apps: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("fetch apps failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result AppListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode apps response: %w", err)
	}

	// Find app matching our client ID
	for _, app := range result.Apps {
		if app.ClientID == c.clientID {
			c.resolvedAppID = fmt.Sprintf("%d", app.ID)
			return c.resolvedAppID, nil
		}
	}

	return "", fmt.Errorf("no app found with client_id %s", c.clientID)
}

// ============================================================================
// User Management DTOs
// ============================================================================

// User represents a user in Socrate (matches Socrate's API response)
type User struct {
	ID               uint       `json:"id"`
	Email            string     `json:"email"`
	Name             string     `json:"name"`         // Socrate uses "name" not "full_name"
	FirstName        string     `json:"first_name"`
	LastName         string     `json:"last_name"`
	DisplayName      string     `json:"display_name"`
	Role             string     `json:"role"`
	SubscriptionTier string     `json:"subscription_tier"`
	Status           string     `json:"status"`
	IsVerified       bool       `json:"is_verified"` // Socrate uses "is_verified" not "email_verified"
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	LastLogin        *time.Time `json:"last_login,omitempty"`
}

// UserListResponse is the response from listing users (matches Socrate's API response)
type UserListResponse struct {
	Users      []User `json:"users"`
	TotalCount int64  `json:"total_count"` // Socrate uses "total_count" not "total"
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// CreateUserRequest is the request to create a user
type CreateUserRequest struct {
	Email       string `json:"email"`
	FullName    string `json:"full_name"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role"`
	Password    string `json:"password,omitempty"` // Optional - if not provided, invite flow is used
}

// UpdateUserRequest is the request to update a user
type UpdateUserRequest struct {
	FullName    *string `json:"full_name,omitempty"`
	FirstName   *string `json:"first_name,omitempty"`
	LastName    *string `json:"last_name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	Role        *string `json:"role,omitempty"`
}

// ActivityLog represents a security audit log entry from Socrate
type ActivityLog struct {
	ID        uint            `json:"id"`
	EventType string          `json:"event_type"` // login_success, login_failed, user_created, etc.
	Severity  string          `json:"severity"`   // info, warning, error, critical
	UserID    *uint           `json:"user_id,omitempty"`
	UserEmail string          `json:"user_email,omitempty"`
	AppID     *uint           `json:"app_id,omitempty"`
	IPAddress string          `json:"ip_address,omitempty"`
	UserAgent string          `json:"user_agent,omitempty"`
	Success   bool            `json:"success"`
	Details   json.RawMessage `json:"details,omitempty"`  // Can be object or string
	Metadata  json.RawMessage `json:"metadata,omitempty"` // Can be object or string
	RequestID string          `json:"request_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// ActivityLogResponse is the response from listing security events
type ActivityLogResponse struct {
	Events     []ActivityLog `json:"events"`
	TotalCount int64         `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
}

// ============================================================================
// Internal helpers
// ============================================================================

// decodeUserResponse decodes a single-user JSON body into *User.
// Socrate may return either a flat object {"id":1,"email":"…"} or a wrapped
// object {"user":{"id":1,"email":"…"}}. We try both so the client is robust
// against either convention without breaking existing tests.
func decodeUserResponse(body []byte) (*User, error) {
	// Attempt 1: flat object at root level.
	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}
	if user.ID != 0 {
		return &user, nil
	}

	// Attempt 2: envelope {"user": {...}} — common Socrate single-resource convention.
	var envelope struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.User.ID != 0 {
		return &envelope.User, nil
	}

	// Return the zero-ID user; callers decide whether that is an error.
	return &user, nil
}

// ============================================================================
// User Management Methods
// ============================================================================

// ListUsers retrieves users for the app with pagination and search
func (c *Client) ListUsers(ctx context.Context, search string, page, pageSize int) (*UserListResponse, error) {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users?page=%d&page_size=%d", appID, page, pageSize)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list users failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result UserListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GetUser retrieves a specific user by ID
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users/%s", appID, userID)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read get user response: %w", err)
	}
	return decodeUserResponse(body)
}

// CreateUser creates a new user in the app
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users", appID)

	resp, err := c.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		// 409 Conflict - user already exists in Socrate
		return nil, ErrUserAlreadyExists
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create user failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read create user response: %w", err)
	}
	return decodeUserResponse(body)
}

// UpdateUser updates a user's information
func (c *Client) UpdateUser(ctx context.Context, userID string, req UpdateUserRequest) (*User, error) {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users/%s", appID, userID)

	resp, err := c.doRequest(ctx, http.MethodPut, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("update user failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read update user response: %w", err)
	}
	return decodeUserResponse(body)
}

// DeleteUser removes a user from the app
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users/%s", appID, userID)

	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete user failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ResendVerification resends the verification email to a user
func (c *Client) ResendVerification(ctx context.Context, userID string) error {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users/%s/resend-verification", appID, userID)

	resp, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend verification failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ResetPassword triggers a password reset for a user
func (c *Client) ResetPassword(ctx context.Context, userID string) error {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users/%s/reset-password", appID, userID)

	resp, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reset password failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ============================================================================
// Service-Account Methods (backend-to-backend, no user JWT required)
// ============================================================================

// clientCredentialsTokenResponse is the token response from Socrate's token endpoint.
type clientCredentialsTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// getServiceToken returns a cached service-account access token, refreshing it
// via the OAuth2 client_credentials grant when it has expired (or was never fetched).
// Thread-safe.
func (c *Client) getServiceToken(ctx context.Context) (string, error) {
	c.svcTokenMu.Lock()
	defer c.svcTokenMu.Unlock()

	// Return cached token if still valid (with 30 s safety margin).
	if c.svcToken != "" && time.Now().Add(30*time.Second).Before(c.svcTokenExpiry) {
		return c.svcToken, nil
	}

	if c.clientSecret == "" {
		return "", errors.New("socrate: client_secret required for service-account token exchange")
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tr clientCredentialsTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	c.svcToken = tr.AccessToken
	if tr.ExpiresIn > 0 {
		c.svcTokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		c.svcTokenExpiry = time.Now().Add(55 * time.Minute) // sensible default
	}

	return c.svcToken, nil
}

// doRequestWithToken performs a request using an explicit Bearer token instead
// of pulling the JWT from the request context.  Used for self-service operations
// where no authenticated user exists (e.g. account registration).
func (c *Client) doRequestWithToken(ctx context.Context, token, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// getAppIDWithToken resolves the numeric Socrate app ID using a service token.
// The resolved ID is shared with the normal JWT-based path (same cache field).
func (c *Client) getAppIDWithToken(ctx context.Context, token string) (string, error) {
	if c.resolvedAppID != "" {
		return c.resolvedAppID, nil
	}

	resp, err := c.doRequestWithToken(ctx, token, http.MethodGet, "/api/admin/apps", nil)
	if err != nil {
		return "", fmt.Errorf("fetch apps: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("fetch apps failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result AppListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode apps response: %w", err)
	}

	for _, app := range result.Apps {
		if app.ClientID == c.clientID {
			c.resolvedAppID = fmt.Sprintf("%d", app.ID)
			return c.resolvedAppID, nil
		}
	}

	return "", fmt.Errorf("no app found with client_id %s", c.clientID)
}

// RegisterUser creates a new user in Socrate using the OAuth2 client_credentials
// service token derived from SOCRATE_CLIENT_ID + SOCRATE_CLIENT_SECRET.
// No admin user JWT is required.  Socrate sends a verification/magic-link email
// to the new user automatically.
func (c *Client) RegisterUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	svcToken, err := c.getServiceToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get service token: %w", err)
	}

	appID, err := c.getAppIDWithToken(ctx, svcToken)
	if err != nil {
		return nil, fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/apps/%s/users", appID)

	resp, err := c.doRequestWithToken(ctx, svcToken, http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, ErrUserAlreadyExists
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("register user failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read register user response: %w", err)
	}
	return decodeUserResponse(body)
}

// SendMagicLink asks Socrate to dispatch a passwordless sign-in email to the
// user identified by email.  callbackURL is the backend endpoint that Socrate
// will embed in the email link (typically `.../auth/magic-link/verify?token=…`).
//
// The method uses the client_credentials service token so no user JWT is needed.
// If Socrate returns 404 (user not found) the error is silently suppressed so
// callers can return a consistent "if the email exists you'll receive a link"
// response without leaking account existence.
func (c *Client) SendMagicLink(ctx context.Context, email, callbackURL string) error {
	svcToken, err := c.getServiceToken(ctx)
	if err != nil {
		return fmt.Errorf("get service token: %w", err)
	}

	appID, err := c.getAppIDWithToken(ctx, svcToken)
	if err != nil {
		return fmt.Errorf("resolve app ID: %w", err)
	}

	// Find user by email via search.
	listPath := fmt.Sprintf("/api/apps/%s/users?search=%s&page=1&page_size=1", appID, url.QueryEscape(email))
	listResp, err := c.doRequestWithToken(ctx, svcToken, http.MethodGet, listPath, nil)
	if err != nil {
		return fmt.Errorf("find user by email: %w", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		return fmt.Errorf("find user failed with status %d: %s", listResp.StatusCode, string(body))
	}

	var list UserListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		return fmt.Errorf("decode user list: %w", err)
	}

	if len(list.Users) == 0 {
		// User not found — suppress silently (no account enumeration).
		return nil
	}

	userID := fmt.Sprintf("%d", list.Users[0].ID)
	path := fmt.Sprintf("/api/apps/%s/users/%s/magic-link", appID, userID)

	body := map[string]string{"callback_url": callbackURL}
	resp, err := c.doRequestWithToken(ctx, svcToken, http.MethodPost, path, body)
	if err != nil {
		return fmt.Errorf("send magic link: %w", err)
	}
	defer resp.Body.Close()

	// 404 = user exists in list but has no magic-link feature → suppress.
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send magic link failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ============================================================================
// Activity Logs
// ============================================================================

// GetActivityLogs retrieves security audit logs for the app from Socrate
func (c *Client) GetActivityLogs(ctx context.Context, page, pageSize int) (*ActivityLogResponse, error) {
	appID, err := c.getAppID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve app ID: %w", err)
	}

	path := fmt.Sprintf("/api/admin/security/events?app_id=%s&page=%d&page_size=%d", appID, page, pageSize)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get security events failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ActivityLogResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
