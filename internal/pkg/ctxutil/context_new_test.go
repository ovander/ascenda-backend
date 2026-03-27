package ctxutil

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Tests for the new context helpers added for Socrate integration:
// UserEmail, UserName, UserSub

func TestWithUserEmail(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserEmail(ctx, "user@example.com")
	assert.Equal(t, "user@example.com", GetUserEmail(ctx))
}

func TestGetUserEmailEmpty(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetUserEmail(ctx))
}

func TestWithUserName(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserName(ctx, "Olivier")
	assert.Equal(t, "Olivier", GetUserName(ctx))
}

func TestGetUserNameEmpty(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetUserName(ctx))
}

func TestWithUserSub(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserSub(ctx, "socrate-sub-123")
	assert.Equal(t, "socrate-sub-123", GetUserSub(ctx))
}

func TestGetUserSubEmpty(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetUserSub(ctx))
}

func TestContextCombinationsWithNewFields(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	ctx = WithTenantID(ctx, tenantID)
	ctx = WithUserID(ctx, userID)
	ctx = WithUserRole(ctx, "editor")
	ctx = WithUserEmail(ctx, "test@ascenda.io")
	ctx = WithUserName(ctx, "Test User")
	ctx = WithUserSub(ctx, "1")

	assert.Equal(t, tenantID, GetTenantID(ctx))
	assert.Equal(t, userID, GetUserID(ctx))
	assert.Equal(t, "editor", GetUserRole(ctx))
	assert.Equal(t, "test@ascenda.io", GetUserEmail(ctx))
	assert.Equal(t, "Test User", GetUserName(ctx))
	assert.Equal(t, "1", GetUserSub(ctx))
}

func TestNewFieldsDoNotInterfereWithExisting(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserRole(ctx, "admin")
	ctx = WithUserEmail(ctx, "admin@test.com")

	// Setting email should not overwrite role
	assert.Equal(t, "admin", GetUserRole(ctx))
	assert.Equal(t, "admin@test.com", GetUserEmail(ctx))
}
