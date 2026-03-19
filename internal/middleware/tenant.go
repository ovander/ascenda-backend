package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"kerplan/internal/model"
	"kerplan/internal/pkg/ctxutil"
	"kerplan/internal/repo"
)

type contextKey string

const dbContextKey contextKey = "db"

// TenantMiddleware injects tenant context, sets up RLS, and resolves KerPlan user role.
type TenantMiddleware struct {
	db       *gorm.DB
	userRepo repo.UserRepository
	logger   *logrus.Entry
}

// NewTenantMiddleware creates a new TenantMiddleware.
func NewTenantMiddleware(db *gorm.DB, userRepo repo.UserRepository, logger *logrus.Entry) *TenantMiddleware {
	return &TenantMiddleware{
		db:       db,
		userRepo: userRepo,
		logger:   logger,
	}
}

// Handler wraps an HTTP handler with tenant context injection and user auto-provisioning.
func (m *TenantMiddleware) Handler(next http.Handler) http.Handler {
	defaultTenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := ctxutil.GetTenantID(r.Context())
		userID := ctxutil.GetUserID(r.Context())
		email := ctxutil.GetUserEmail(r.Context())
		name := ctxutil.GetUserName(r.Context())
		sub := ctxutil.GetUserSub(r.Context())

		if tenantID == uuid.Nil {
			m.logger.Warn("no tenant_id in JWT claims, using default tenant")
			tenantID = defaultTenantID
		}

		// Set PostgreSQL RLS variable
		ctx := r.Context()
		tx := m.db.WithContext(ctx).Exec("SELECT set_config('app.tenant_id', @tenantID, true)", map[string]interface{}{"tenantID": tenantID.String()})
		if tx.Error != nil {
			m.logger.WithError(tx.Error).Error("failed to set tenant_id RLS variable")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Resolve KerPlan user role from users table (auto-provision on first login)
		userRole := ""
		if m.userRepo != nil && sub != "" {
			user, err := m.userRepo.GetByExternalID(sub)
			if err != nil || user == nil {
				// Auto-provision: first user is owner, rest are base "user"
				count, _ := m.userRepo.CountByTenant(tenantID)
				role := "user"
				if count == 0 {
					role = "owner"
				}
				now := time.Now()
				newUser := &model.User{
					ID:         userID,
					TenantID:   tenantID,
					ExternalID: sub,
					Email:      email,
					Name:       name,
					Role:       role,
					IsActive:   true,
					JoinedAt:   &now,
				}
				if createErr := m.userRepo.Create(newUser); createErr != nil {
					m.logger.WithError(createErr).Warn("failed to auto-provision user")
				} else {
					m.logger.WithField("email", email).WithField("role", role).Info("user auto-provisioned")
				}
				userRole = role
			} else {
				userRole = user.Role
				// Update cached fields if changed
				changed := false
				if user.Email != email && email != "" {
					user.Email = email
					changed = true
				}
				if user.Name != name && name != "" {
					user.Name = name
					changed = true
				}
				if changed {
					m.userRepo.Update(user)
				}
				// Use the user's actual ID from the database
				userID = user.ID
			}
		}

		ctx = ctxutil.WithTenantID(ctx, tenantID)
		ctx = ctxutil.WithUserID(ctx, userID)
		ctx = ctxutil.WithUserRole(ctx, userRole)

		// Pass database transaction in context for subsequent handlers
		ctx = context.WithValue(ctx, dbContextKey, m.db.WithContext(ctx))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
