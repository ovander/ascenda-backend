package ctxutil

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWithTenantID(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()

	ctx = WithTenantID(ctx, tenantID)
	retrieved := GetTenantID(ctx)

	assert.Equal(t, tenantID, retrieved)
}

func TestGetTenantID(t *testing.T) {
	tests := []struct {
		name           string
		ctx            context.Context
		expectedNil    bool
		expectedResult uuid.UUID
	}{
		{
			name:        "context with tenant ID",
			ctx:         WithTenantID(context.Background(), uuid.New()),
			expectedNil: false,
		},
		{
			name:           "context without tenant ID",
			ctx:            context.Background(),
			expectedNil:    true,
			expectedResult: uuid.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTenantID(tt.ctx)
			if tt.expectedNil {
				assert.Equal(t, uuid.Nil, result)
			} else {
				assert.NotEqual(t, uuid.Nil, result)
			}
		})
	}
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	ctx = WithUserID(ctx, userID)
	retrieved := GetUserID(ctx)

	assert.Equal(t, userID, retrieved)
}

func TestGetUserID(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectedNil bool
	}{
		{
			name:        "context with user ID",
			ctx:         WithUserID(context.Background(), uuid.New()),
			expectedNil: false,
		},
		{
			name:        "context without user ID",
			ctx:         context.Background(),
			expectedNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUserID(tt.ctx)
			if tt.expectedNil {
				assert.Equal(t, uuid.Nil, result)
			} else {
				assert.NotEqual(t, uuid.Nil, result)
			}
		})
	}
}

func TestWithUserRole(t *testing.T) {
	ctx := context.Background()
	role := "admin"

	ctx = WithUserRole(ctx, role)
	retrieved := GetUserRole(ctx)

	assert.Equal(t, role, retrieved)
}

func TestGetUserRole(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectedRole string
	}{
		{
			name:        "context with role",
			ctx:         WithUserRole(context.Background(), "editor"),
			expectedRole: "editor",
		},
		{
			name:        "context without role",
			ctx:         context.Background(),
			expectedRole: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUserRole(tt.ctx)
			assert.Equal(t, tt.expectedRole, result)
		})
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New().String()

	ctx = WithRequestID(ctx, requestID)
	retrieved := GetRequestID(ctx)

	assert.Equal(t, requestID, retrieved)
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name           string
		ctx            context.Context
		expectedID     string
	}{
		{
			name:           "context with request ID",
			ctx:            WithRequestID(context.Background(), "req-123"),
			expectedID:     "req-123",
		},
		{
			name:           "context without request ID",
			ctx:            context.Background(),
			expectedID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRequestID(tt.ctx)
			assert.Equal(t, tt.expectedID, result)
		})
	}
}

func TestGetTenantIDStr(t *testing.T) {
	tests := []struct {
		name           string
		ctx            context.Context
		expectedResult string
	}{
		{
			name: "context with tenant ID",
			ctx: func() context.Context {
				id := uuid.New()
				return WithTenantID(context.Background(), id)
			}(),
			expectedResult: "",  // Will be set during test
		},
		{
			name:           "context without tenant ID",
			ctx:            context.Background(),
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTenantIDStr(tt.ctx)
			tenantID := GetTenantID(tt.ctx)
			if tenantID == uuid.Nil {
				assert.Equal(t, "", result)
			} else {
				assert.Equal(t, tenantID.String(), result)
			}
		})
	}
}

func TestContextCombinations(t *testing.T) {
	t.Run("multiple context values", func(t *testing.T) {
		ctx := context.Background()
		tenantID := uuid.New()
		userID := uuid.New()
		role := "owner"
		requestID := "req-456"

		ctx = WithTenantID(ctx, tenantID)
		ctx = WithUserID(ctx, userID)
		ctx = WithUserRole(ctx, role)
		ctx = WithRequestID(ctx, requestID)

		assert.Equal(t, tenantID, GetTenantID(ctx))
		assert.Equal(t, userID, GetUserID(ctx))
		assert.Equal(t, role, GetUserRole(ctx))
		assert.Equal(t, requestID, GetRequestID(ctx))
	})
}

func TestContextIsolation(t *testing.T) {
	t.Run("contexts do not interfere", func(t *testing.T) {
		ctx1 := context.Background()
		ctx2 := context.Background()

		tenantID1 := uuid.New()
		tenantID2 := uuid.New()

		ctx1 = WithTenantID(ctx1, tenantID1)
		ctx2 = WithTenantID(ctx2, tenantID2)

		assert.Equal(t, tenantID1, GetTenantID(ctx1))
		assert.Equal(t, tenantID2, GetTenantID(ctx2))
		assert.NotEqual(t, GetTenantID(ctx1), GetTenantID(ctx2))
	})
}
