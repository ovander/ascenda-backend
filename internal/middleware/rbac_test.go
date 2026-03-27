package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"ascenda/internal/pkg/ctxutil"
)

func TestRBACMiddlewarePermissions(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	rbac := NewRBACMiddleware(logger)

	tests := []struct {
		name           string
		role           string
		requiredPerm   Permission
		expectedStatus int
	}{
		// user: only self:service
		{name: "user can self-service", role: "user", requiredPerm: PermSelfService, expectedStatus: http.StatusOK},
		{name: "user cannot view plans (global)", role: "user", requiredPerm: PermViewPlan, expectedStatus: http.StatusForbidden},
		{name: "user cannot edit plans (global)", role: "user", requiredPerm: PermEditPlan, expectedStatus: http.StatusForbidden},
		{name: "user cannot manage users", role: "user", requiredPerm: PermManageUsers, expectedStatus: http.StatusForbidden},

		// admin: platform:admin + self:service only — NO tenant or business data access
		{name: "admin has platform:admin", role: "admin", requiredPerm: PermPlatformAdmin, expectedStatus: http.StatusOK},
		{name: "admin can self-service", role: "admin", requiredPerm: PermSelfService, expectedStatus: http.StatusOK},
		{name: "admin CANNOT manage users (tenant-scoped)", role: "admin", requiredPerm: PermManageUsers, expectedStatus: http.StatusForbidden},
		{name: "admin CANNOT view plans", role: "admin", requiredPerm: PermViewPlan, expectedStatus: http.StatusForbidden},
		{name: "admin CANNOT edit plans", role: "admin", requiredPerm: PermEditPlan, expectedStatus: http.StatusForbidden},
		{name: "admin CANNOT manage plans", role: "admin", requiredPerm: PermManagePlan, expectedStatus: http.StatusForbidden},
		{name: "admin CANNOT manage tenant", role: "admin", requiredPerm: PermManageTenant, expectedStatus: http.StatusForbidden},

		// owner: everything
		{name: "owner can view plans", role: "owner", requiredPerm: PermViewPlan, expectedStatus: http.StatusOK},
		{name: "owner can edit plans", role: "owner", requiredPerm: PermEditPlan, expectedStatus: http.StatusOK},
		{name: "owner can manage plans", role: "owner", requiredPerm: PermManagePlan, expectedStatus: http.StatusOK},
		{name: "owner can manage users", role: "owner", requiredPerm: PermManageUsers, expectedStatus: http.StatusOK},
		{name: "owner can manage tenant", role: "owner", requiredPerm: PermManageTenant, expectedStatus: http.StatusOK},
		{name: "owner can self-service", role: "owner", requiredPerm: PermSelfService, expectedStatus: http.StatusOK},

		// Plan-level roles used inside plan routes (set by PlanAccessMiddleware)
		// Note: editor/viewer are NOT in global RolePermissions - they're injected per-plan
		{name: "editor role (plan-level) has no global perms", role: "editor", requiredPerm: PermManageUsers, expectedStatus: http.StatusForbidden},
		{name: "viewer role (plan-level) has no global perms", role: "viewer", requiredPerm: PermManageUsers, expectedStatus: http.StatusForbidden},

		// Unknown role: nothing
		{name: "unknown role denied", role: "unknown", requiredPerm: PermViewPlan, expectedStatus: http.StatusForbidden},
		{name: "empty role denied", role: "", requiredPerm: PermViewPlan, expectedStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := rbac.RequirePermission(tt.requiredPerm)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequest("GET", "/api/test", nil)
			ctx := ctxutil.WithUserRole(req.Context(), tt.role)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRBACMiddlewareNoRole(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	rbac := NewRBACMiddleware(logger)

	handler := rbac.RequirePermission(PermViewPlan)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRBACHasPermission(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	rbac := NewRBACMiddleware(logger)

	tests := []struct {
		name           string
		role           string
		permission     Permission
		expectedResult bool
	}{
		{name: "user has self-service", role: "user", permission: PermSelfService, expectedResult: true},
		{name: "user no view plan", role: "user", permission: PermViewPlan, expectedResult: false},
		{name: "admin has platform:admin", role: "admin", permission: PermPlatformAdmin, expectedResult: true},
		{name: "admin no manage users (tenant-scoped)", role: "admin", permission: PermManageUsers, expectedResult: false},
		{name: "admin no view plan", role: "admin", permission: PermViewPlan, expectedResult: false},
		{name: "owner has manage tenant", role: "owner", permission: PermManageTenant, expectedResult: true},
		{name: "owner has self-service", role: "owner", permission: PermSelfService, expectedResult: true},
		{name: "unknown no permissions", role: "guest", permission: PermViewPlan, expectedResult: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rbac.hasPermission(tt.role, tt.permission)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestRBACRolePermissionMapping(t *testing.T) {
	for role, permissions := range RolePermissions {
		assert.NotEmpty(t, permissions, "Role %s has no permissions", role)
		for _, perm := range permissions {
			assert.NotEmpty(t, string(perm))
		}
	}

	// Verify expected tenant-level roles exist
	expectedRoles := []string{"user", "admin", "owner"}
	for _, role := range expectedRoles {
		_, exists := RolePermissions[role]
		assert.True(t, exists, "Expected role %s not found", role)
	}

	// editor and viewer are plan-level, NOT in global map
	_, editorExists := RolePermissions["editor"]
	_, viewerExists := RolePermissions["viewer"]
	assert.False(t, editorExists, "editor should not be a global role")
	assert.False(t, viewerExists, "viewer should not be a global role")

	// admin (platform-level) has NO tenant or plan permissions — only platform:admin + self:service
	for _, perm := range RolePermissions["admin"] {
		assert.NotEqual(t, PermViewPlan, perm, "admin should not have view:plan")
		assert.NotEqual(t, PermEditPlan, perm, "admin should not have edit:plan")
		assert.NotEqual(t, PermManagePlan, perm, "admin should not have manage:plan")
		assert.NotEqual(t, PermManageUsers, perm, "admin should not have manage:users (tenant-scoped)")
		assert.NotEqual(t, PermManageTenant, perm, "admin should not have manage:tenant")
	}

	// admin must have platform:admin permission
	adminPerms := RolePermissions["admin"]
	hasPlatformAdmin := false
	for _, p := range adminPerms {
		if p == PermPlatformAdmin {
			hasPlatformAdmin = true
			break
		}
	}
	assert.True(t, hasPlatformAdmin, "admin must have platform:admin permission")
}
