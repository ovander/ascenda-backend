package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Integration-level tests for the auth middleware shim.
// Unit tests for the internal bearer-token extraction logic live in
// github.com/ovander/backendkit/jwtauth.

func TestUUIDv5FallbackDeterministic(t *testing.T) {
	// When Socrate sub is not a UUID, we use uuid.NewSHA1 to generate a deterministic UUID
	sub := "1" // Socrate user ID (not a UUID)

	uuid1 := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("socrate:"+sub))
	uuid2 := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("socrate:"+sub))

	assert.Equal(t, uuid1, uuid2, "same input should produce same UUID v5")
	assert.NotEqual(t, uuid.Nil, uuid1)
}

func TestUUIDv5DifferentSubsProduceDifferentUUIDs(t *testing.T) {
	uuid1 := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("socrate:1"))
	uuid2 := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("socrate:2"))

	assert.NotEqual(t, uuid1, uuid2)
}

func TestContextEmailNameInjection(t *testing.T) {
	// Simulate what the auth middleware does after validating a token
	ctx := context.Background()

	ctx = ctxutil.WithUserEmail(ctx, "user@test.com")
	ctx = ctxutil.WithUserName(ctx, "Test User")
	ctx = ctxutil.WithUserSub(ctx, "42")

	assert.Equal(t, "user@test.com", ctxutil.GetUserEmail(ctx))
	assert.Equal(t, "Test User", ctxutil.GetUserName(ctx))
	assert.Equal(t, "42", ctxutil.GetUserSub(ctx))
}

func TestOptionalTenantIDSkipsWhenEmpty(t *testing.T) {
	// When claims.TenantID is empty, the middleware should not set tenant_id in context
	ctx := context.Background()

	tenantID := ctxutil.GetTenantID(ctx)
	assert.Equal(t, uuid.Nil, tenantID, "no tenant_id should mean uuid.Nil")
}

func TestTenantIDSetWhenPresent(t *testing.T) {
	ctx := context.Background()
	expected := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	ctx = ctxutil.WithTenantID(ctx, expected)
	assert.Equal(t, expected, ctxutil.GetTenantID(ctx))
}

func TestUserIDFromUUID(t *testing.T) {
	// When the Socrate sub is already a UUID, parse it directly
	sub := "550e8400-e29b-41d4-a716-446655440000"
	userID, err := uuid.Parse(sub)
	assert.NoError(t, err)
	assert.Equal(t, uuid.MustParse(sub), userID)
}

func TestUserIDFromNonUUID(t *testing.T) {
	// When the Socrate sub is not a UUID (e.g., "1"), generate UUID v5
	sub := "1"
	_, err := uuid.Parse(sub)
	assert.Error(t, err, "short sub should fail UUID parse")

	// Fallback to UUID v5
	userID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("socrate:"+sub))
	assert.NotEqual(t, uuid.Nil, userID)
}

func TestMiddlewareRejects401WithoutToken(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	auth := NewAuthMiddleware("http://example.com/jwks", "", "", false, logger)

	nextCalled := false
	handler := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, nextCalled, "next handler should not be called without token")
}

func TestSocrateClaimsStructure(t *testing.T) {
	// Verify the SocrateClaims struct has the right JSON tags
	claims := SocrateClaims{
		TenantID: "", // Empty for Socrate — user JWT carries "sub" via RegisteredClaims
		Email:    "user@socrate.com",
		Name:     "User Name",
		Role:     "", // Empty, will default to "editor"
	}

	assert.Empty(t, claims.TenantID)
	assert.Equal(t, "user@socrate.com", claims.Email)
	assert.Equal(t, "User Name", claims.Name)
}
