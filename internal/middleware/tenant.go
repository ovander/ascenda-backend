package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
)

type contextKey string

const dbContextKey contextKey = "db"

// UserProfileFetcher retrieves the currently authenticated user's profile from
// the identity provider. ctx must already contain the user's JWT (set by
// AuthMiddleware via socrate.WithJWT).
// Returns empty strings (not an error) when the IdP has no profile data.
type UserProfileFetcher interface {
	GetCurrentUserProfile(ctx context.Context) (email, name string, err error)
}

// TenantMiddleware injects tenant context, sets up RLS, and resolves Ascenda user role.
type TenantMiddleware struct {
	db         *gorm.DB
	userRepo   repo.UserRepository
	tenantRepo repo.TenantRepository // used to resolve effective plan for enterprise tenants
	logger     *logrus.Entry
	profiler   UserProfileFetcher // optional; nil = skip IdP enrichment
}

// NewTenantMiddleware creates a new TenantMiddleware.
// tenantRepo is used to load the tenant record to resolve the effective commercial plan
// for enterprise tenants (tenant.Plan overrides user.Plan in that case).
// profiler is optional; pass nil to skip identity-provider profile enrichment.
func NewTenantMiddleware(db *gorm.DB, userRepo repo.UserRepository, tenantRepo repo.TenantRepository, logger *logrus.Entry, profiler UserProfileFetcher) *TenantMiddleware {
	return &TenantMiddleware{
		db:         db,
		userRepo:   userRepo,
		tenantRepo: tenantRepo,
		logger:     logger,
		profiler:   profiler,
	}
}

// Handler wraps an HTTP handler with tenant context injection and user auto-provisioning.
func (m *TenantMiddleware) Handler(next http.Handler) http.Handler {
	defaultTenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ── Socrate role contract ──────────────────────────────────────────────
		// Socrate issues exactly two JWT roles:
		//   "admin" — platform-wide Ascenda operator; not tenant-scoped; fast-path here.
		//   "user"  — any regular authenticated user; the Ascenda DB enriches this into
		//             "owner", "editor", or "reader" via the lookup below.
		// The JWT "user" role is intentionally discarded and replaced with the DB role.
		if jwtRole := ctxutil.GetUserRole(r.Context()); jwtRole == "admin" {
			next.ServeHTTP(w, r) // JWT role preserved; no DB provisioning needed
			return
		}

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

		// ── Resolve effective plan ─────────────────────────────────────────────
		// For enterprise tenants the plan is stored on the tenant record and
		// applies to all members regardless of their individual User.Plan value.
		// For workspace tenants the per-user plan governs (existing behaviour).
		userRole := ""
		userPlan := "freemium" // commercial plan default

		// Load tenant to check type — fast indexed PK lookup.
		var tenantPlan string
		if m.tenantRepo != nil {
			if t, err := m.tenantRepo.GetByID(tenantID); err == nil && t != nil {
				if t.Type == model.TenantTypeEnterprise && t.Plan != "" {
					tenantPlan = t.Plan // enterprise: tenant plan overrides user plan
				}
			}
		}

		if m.userRepo != nil && sub != "" {
			user, err := m.userRepo.GetByExternalID(sub)
			if err != nil || user == nil {
				// Auto-provision: first user is owner, rest default to editor.
				count, _ := m.userRepo.CountByTenant(tenantID)
				role := "editor"
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
					Plan:       "freemium",
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
				if user.Plan != "" {
					userPlan = user.Plan
				}
				// Update cached fields if changed; prefer JWT claims, fall back to
				// IdP profile fetch when the JWT does not carry email/name.
				changed := false
				if user.Email != email && email != "" {
					user.Email = email
					changed = true
				}
				if user.Name != name && name != "" {
					user.Name = name
					changed = true
				}
				// If email is still missing, try the OIDC userinfo endpoint once.
				// This covers IdPs (like Socrate) whose access tokens do not embed
				// email/name claims, while keeping the per-request cost to one extra
				// HTTP call only until the profile has been populated.
				if user.Email == "" && m.profiler != nil {
					if profEmail, profName, profErr := m.profiler.GetCurrentUserProfile(ctx); profErr == nil {
						if profEmail != "" && user.Email != profEmail {
							user.Email = profEmail
							changed = true
						}
						if profName != "" && user.Name != profName {
							user.Name = profName
							changed = true
						}
					} else {
						m.logger.WithError(profErr).Debug("userinfo profile fetch failed")
					}
				}
				if changed {
					m.userRepo.Update(user)
				}
				// Use the user's actual ID from the database
				userID = user.ID
			}
		}

		// Enterprise tenants: tenant plan takes precedence over individual user plan.
		if tenantPlan != "" {
			userPlan = tenantPlan
		}

		ctx = ctxutil.WithTenantID(ctx, tenantID)
		ctx = ctxutil.WithUserID(ctx, userID)
		ctx = ctxutil.WithUserRole(ctx, userRole)
		ctx = ctxutil.WithUserPlan(ctx, userPlan)

		// Pass database transaction in context for subsequent handlers
		ctx = context.WithValue(ctx, dbContextKey, m.db.WithContext(ctx))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
