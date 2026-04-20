package middleware

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/ovander/backendkit/ctxutil"
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

// RolePermissions maps Ascenda tenant roles to their permissions.
//
// Socrate issues only "user" or "admin" in the JWT. The tenant middleware
// enriches "user" into one of the three Ascenda roles below via a DB lookup.
//
//   - editor  — full plan access (operate + analyse); no admin menus
//   - reader  — read-only plan access (analyse only); no operate or admin menus
//   - owner   — manages workspace (users, tenant settings) + full plan access
//   - admin   — platform-wide Ascenda operator; JWT role preserved as-is (no DB lookup)
//
// Plan-level roles (editor|viewer) are handled separately via the plan_members table.
var RolePermissions = map[string][]Permission{
	// editor: full plan access (operate + analyse), no admin menus
	"editor": {
		PermViewPlan,
		PermEditPlan,
		PermSelfService,
	},
	// reader: read-only plan access (analyse only), no operate or admin menus
	"reader": {
		PermViewPlan,
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
