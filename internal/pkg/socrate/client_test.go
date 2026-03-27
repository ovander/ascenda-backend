package socrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ================================================================
// NewClient Tests
// ================================================================

func TestNewClient_Success(t *testing.T) {
	client, err := NewClient(ClientConfig{
		BaseURL:  "https://socrate.example.com",
		ClientID: "ascenda-client-id",
	})
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "https://socrate.example.com", client.baseURL)
	assert.Equal(t, "ascenda-client-id", client.clientID)
}

func TestNewClient_MissingBaseURL(t *testing.T) {
	_, err := NewClient(ClientConfig{ClientID: "ascenda-client-id"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base URL is required")
}

func TestNewClient_MissingClientID(t *testing.T) {
	_, err := NewClient(ClientConfig{BaseURL: "https://socrate.example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client ID is required")
}

// ================================================================
// JWT context helpers
// ================================================================

func TestWithJWT(t *testing.T) {
	ctx := WithJWT(context.Background(), "my-admin-jwt")
	extracted, err := getJWT(ctx)
	require.NoError(t, err)
	assert.Equal(t, "my-admin-jwt", extracted)
}

func TestGetJWT_MissingJWT(t *testing.T) {
	_, err := getJWT(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no JWT in context")
}

func TestGetJWT_EmptyJWT(t *testing.T) {
	_, err := getJWT(WithJWT(context.Background(), ""))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no JWT in context")
}

// ================================================================
// Helpers shared across test cases
// ================================================================

// newTestServer builds a minimal mock Socrate API server.
// appRoute handles /api/apps/{appID}/... calls.
// If appID is 0 a default of 123 is used.
func newTestServer(t *testing.T, appID int, appRoute func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	if appID == 0 {
		appID = 123
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/apps" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AppListResponse{
				Apps: []App{{ID: uint(appID), ClientID: "test-client-id"}},
			})
			return
		}
		appRoute(w, r)
	}))
}

func newTestClient(t *testing.T, serverURL string) *Client {
	c, err := NewClient(ClientConfig{BaseURL: serverURL, ClientID: "test-client-id"})
	require.NoError(t, err)
	return c
}

// ctxWithJWT returns a background context containing a stub Bearer token.
func ctxWithJWT() context.Context {
	return WithJWT(context.Background(), "admin-jwt")
}

// ================================================================
// ListUsers
// ================================================================

func TestClient_ListUsers_Success(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users" {
			assert.Equal(t, "Bearer admin-jwt", r.Header.Get("Authorization"))
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			assert.Equal(t, "10", r.URL.Query().Get("page_size"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(UserListResponse{
				Users: []User{
					{ID: 1, Email: "alice@example.com", Name: "Alice", Role: "user"},
					{ID: 2, Email: "bob@example.com", Name: "Bob", Role: "admin"},
				},
				TotalCount: 2,
				Page:       1,
				PageSize:   10,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	result, err := newTestClient(t, server.URL).ListUsers(ctxWithJWT(), "", 1, 10)
	require.NoError(t, err)
	assert.Len(t, result.Users, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	// Verify the two Socrate roles we received
	roles := map[string]bool{}
	for _, u := range result.Users {
		roles[u.Role] = true
	}
	assert.True(t, roles["user"])
	assert.True(t, roles["admin"])
}

func TestClient_ListUsers_WithSearch(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users" {
			assert.Equal(t, "alice", r.URL.Query().Get("search"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(UserListResponse{
				Users:      []User{{ID: 1, Email: "alice@example.com", Role: "user"}},
				TotalCount: 1,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	result, err := newTestClient(t, server.URL).ListUsers(ctxWithJWT(), "alice", 1, 10)
	require.NoError(t, err)
	assert.Len(t, result.Users, 1)
	assert.Equal(t, "alice@example.com", result.Users[0].Email)
}

func TestClient_ListUsers_NoJWT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL).ListUsers(context.Background(), "", 1, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no JWT in context")
}

// ================================================================
// GetUser
// ================================================================

func TestClient_GetUser_Success(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users/7" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(User{ID: 7, Email: "carol@example.com", Name: "Carol", Role: "user"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	user, err := newTestClient(t, server.URL).GetUser(ctxWithJWT(), "7")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, uint(7), user.ID)
	assert.Equal(t, "carol@example.com", user.Email)
}

func TestClient_GetUser_NotFound(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	user, err := newTestClient(t, server.URL).GetUser(ctxWithJWT(), "999")
	require.NoError(t, err)
	assert.Nil(t, user)
}

// ================================================================
// CreateUser — Socrate roles are "admin" or "user"
// ================================================================

func TestClient_CreateUser_AsUser(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users" && r.Method == http.MethodPost {
			var req CreateUserRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "user", req.Role, "Socrate role must be 'user'")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(User{ID: 10, Email: req.Email, Name: req.FullName, Role: req.Role})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	user, err := newTestClient(t, server.URL).CreateUser(ctxWithJWT(), CreateUserRequest{
		Email: "new@example.com", FullName: "New User", Role: "user",
	})
	require.NoError(t, err)
	assert.Equal(t, uint(10), user.ID)
	assert.Equal(t, "user", user.Role)
}

func TestClient_CreateUser_AsAdmin(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users" && r.Method == http.MethodPost {
			var req CreateUserRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "admin", req.Role)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(User{ID: 11, Email: req.Email, Role: req.Role})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	user, err := newTestClient(t, server.URL).CreateUser(ctxWithJWT(), CreateUserRequest{
		Email: "admin@example.com", FullName: "Admin User", Role: "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Role)
}

func TestClient_CreateUser_AlreadyExists(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"user already exists"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := newTestClient(t, server.URL).CreateUser(ctxWithJWT(), CreateUserRequest{
		Email: "dup@example.com", FullName: "Dup", Role: "user",
	})
	assert.ErrorIs(t, err, ErrUserAlreadyExists)
}

func TestClient_CreateUser_WrappedEnvelopeResponse(t *testing.T) {
	// Some Socrate versions return {"user":{…}}
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"user":{"id":42,"email":"env@example.com","name":"Env","role":"user"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	user, err := newTestClient(t, server.URL).CreateUser(ctxWithJWT(), CreateUserRequest{
		Email: "env@example.com", FullName: "Env", Role: "user",
	})
	require.NoError(t, err)
	assert.Equal(t, uint(42), user.ID)
}

// ================================================================
// UpdateUser
// ================================================================

func TestClient_UpdateUser_Success(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users/5" && r.Method == http.MethodPut {
			var req UpdateUserRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.WriteHeader(http.StatusOK)
			role := "user"
			if req.Role != nil {
				role = *req.Role
			}
			_ = json.NewEncoder(w).Encode(User{ID: 5, Email: "user@example.com", Name: *req.FullName, Role: role})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	newName := "Updated"
	newRole := "admin" // promote to admin in Socrate
	user, err := newTestClient(t, server.URL).UpdateUser(ctxWithJWT(), "5", UpdateUserRequest{
		FullName: &newName,
		Role:     &newRole,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", user.Name)
	assert.Equal(t, "admin", user.Role)
}

// ================================================================
// DeleteUser
// ================================================================

func TestClient_DeleteUser_Success(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users/9" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := newTestClient(t, server.URL).DeleteUser(ctxWithJWT(), "9")
	assert.NoError(t, err)
}

func TestClient_DeleteUser_NotFound(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})
	defer server.Close()

	err := newTestClient(t, server.URL).DeleteUser(ctxWithJWT(), "999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// ================================================================
// ResendVerification / ResetPassword
// ================================================================

func TestClient_ResendVerification_Success(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users/3/resend-verification" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := newTestClient(t, server.URL).ResendVerification(ctxWithJWT(), "3")
	assert.NoError(t, err)
}

func TestClient_ResetPassword_Success(t *testing.T) {
	server := newTestServer(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/apps/123/users/4/reset-password" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := newTestClient(t, server.URL).ResetPassword(ctxWithJWT(), "4")
	assert.NoError(t, err)
}

// ================================================================
// App ID resolution
// ================================================================

func TestClient_GetAppID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/apps" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AppListResponse{
				Apps: []App{{ID: 999, ClientID: "other-client"}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL).ListUsers(ctxWithJWT(), "", 1, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no app found with client_id")
}

func TestClient_GetAppID_CachedAcrossCalls(t *testing.T) {
	appResolveCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/apps" {
			appResolveCount++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AppListResponse{
				Apps: []App{{ID: 123, ClientID: "test-client-id"}},
			})
			return
		}
		if r.URL.Path == "/api/apps/123/users" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(UserListResponse{Users: []User{}, TotalCount: 0})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx := ctxWithJWT()

	_, _ = client.ListUsers(ctx, "", 1, 10)
	_, _ = client.ListUsers(ctx, "", 2, 10)
	_, _ = client.ListUsers(ctx, "", 3, 10)

	assert.Equal(t, 1, appResolveCount, "app ID should be resolved only once and then cached")
}

// ================================================================
// decodeUserResponse
// ================================================================

func TestDecodeUserResponse_FlatObject(t *testing.T) {
	body := []byte(`{"id":42,"email":"user@example.com","name":"Test","role":"user"}`)
	user, err := decodeUserResponse(body)
	require.NoError(t, err)
	assert.Equal(t, uint(42), user.ID)
	assert.Equal(t, "user@example.com", user.Email)
	assert.Equal(t, "user", user.Role)
}

func TestDecodeUserResponse_EnvelopeObject(t *testing.T) {
	body := []byte(`{"user":{"id":99,"email":"admin@example.com","name":"Admin","role":"admin"}}`)
	user, err := decodeUserResponse(body)
	require.NoError(t, err)
	assert.Equal(t, uint(99), user.ID)
	assert.Equal(t, "admin", user.Role)
}

func TestDecodeUserResponse_InvalidJSON(t *testing.T) {
	_, err := decodeUserResponse([]byte(`{not valid`))
	assert.Error(t, err)
}

func TestDecodeUserResponse_ZeroID(t *testing.T) {
	// If no ID is present, caller decides whether it is an error.
	user, err := decodeUserResponse([]byte(`{"email":"nobody@example.com"}`))
	require.NoError(t, err)
	assert.Equal(t, uint(0), user.ID)
}

// ================================================================
// Server error handling
// ================================================================

func TestClient_ServerError_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL).ListUsers(ctxWithJWT(), "", 1, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
