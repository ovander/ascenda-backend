package middleware

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"ascenda/internal/pkg/ctxutil"
)

// Permission represents an action permission.
type Permission string

const (
	PermViewPlan      Permission = "view:plan"
	PermEditPlan      Permission = "edit:plan"
	PermManagePlan    Permission = "manage:plan"
	PermManageUsers   Permission = "manage:users"   // tenant-scoped user management (owner)
	PermManageTenant  Permission = "manage:tenant"
	PermPlatformAdmin Permission = "platform:admin" // platform-wide admin (not tenant-scoped)
	PermSelfService   Permission = "self:service"   // all authenticated users can access /users/me
)

// RolePermissions maps roles to their permissions.
//
// Roles:
//   - user   — regular business user; accesses plans via plan memberships
//   - owner  — tenant owner; manages their workspace (users, tenant settings) + creates plans
//   - admin  — platform-wide Ascenda operator; sees all tenants; not tied to any tenant
//
// Plan-level roles (editor|viewer) are handled via the plan_members table.
var RolePermissions = map[string][]Permission{
	"user": {
		PermSelfService,
	},
	"owner": {
		PermViewPlan,
		PermEditPlan,
		PermManagePlan,
		PermManageUsers,
		PermManageTenant,
		PermSelfService,
	},
	"admin": {
		PermPlatformAdmin,
		PermSelfService,
	},
}

// RBACMiddleware provides role-based access control.
type RBACMiddleware struct {
	logger *logrus.Entry
}

// NewRBACMiddleware creates a new RBACMiddleware.
func NewRBACMiddleware(logger *logrus.Entry) *RBACMiddleware {
	return &RBACMiddleware{
		logger: logger,
	}
}

// RequirePermission returns middleware that enforces a permission requirement.
func (m *RBACMiddleware) RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := ctxutil.GetUserRole(r.Context())

			if !m.hasPermission(role, perm) {
				m.logger.WithField("role", role).
					WithField("required_permission", perm).
					Warn("permission denied")
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hasPermission checks if a role has a specific permission.
func (m *RBACMiddleware) hasPermission(role string, perm Permission) bool {
	permissions, ok := RolePermissions[role]
	if !ok {
		return false
	}

	for _, p := range permissions {
		if p == perm {
			return true
		}
	}

	return false
}
