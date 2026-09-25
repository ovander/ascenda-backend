package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// DefaultTenantID is the seeded "Default Workspace" tenant (migration 000013).
// It is only used when AllowDefaultTenantFallback is enabled (local development).
var DefaultTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// UserProfileFetcher retrieves the currently authenticated user's profile from
// the identity provider. ctx must already contain the user's JWT (set by
// AuthMiddleware via socrate.WithJWT).
// Returns empty strings (not an error) when the IdP has no profile data.
type UserProfileFetcher interface {
	GetCurrentUserProfile(ctx context.Context) (email, name string, err error)
}

// TenantMiddleware resolves the authenticated user's tenant, Ascenda role and
// commercial plan and stores them in the request context.
//
// Resolution order (the user record in the Ascenda DB is authoritative):
//
//  1. JWT role "admin" → platform operator, no tenant context, pass through.
//  2. User record found by JWT subject → tenant, role and plan come from the
//     record. A tenant_id claim that disagrees with the record is logged and
//     ignored. Deactivated users are rejected.
//  3. No record, but a pending invitation (empty external_id) matches the
//     user's e-mail → the invitation is claimed and its tenant/role apply.
//  4. No record, JWT carries a tenant_id that exists → the user is provisioned
//     into that tenant (first member becomes owner, others editor).
//  5. No record, no usable tenant_id → 403, unless AllowDefaultTenantFallback
//     is enabled (development only), in which case the user is provisioned
//     into DefaultTenantID.
type TenantMiddleware struct {
	userRepo           repo.UserRepository
	tenantRepo         repo.TenantRepository
	logger             *logrus.Entry
	profiler           UserProfileFetcher // optional; nil = skip IdP enrichment
	allowDefaultTenant bool
}

// NewTenantMiddleware creates a new TenantMiddleware.
// tenantRepo is used to verify tenants exist and to resolve the effective
// commercial plan for enterprise tenants (tenant.Plan overrides user.Plan).
// profiler is optional; pass nil to skip identity-provider profile enrichment.
// allowDefaultTenant enables the development-only default-tenant fallback.
func NewTenantMiddleware(userRepo repo.UserRepository, tenantRepo repo.TenantRepository, logger *logrus.Entry, profiler UserProfileFetcher, allowDefaultTenant bool) *TenantMiddleware {
	return &TenantMiddleware{
		userRepo:           userRepo,
		tenantRepo:         tenantRepo,
		logger:             logger,
		profiler:           profiler,
		allowDefaultTenant: allowDefaultTenant,
	}
}

// Handler wraps an HTTP handler with tenant context injection and user provisioning.
func (m *TenantMiddleware) Handler(next http.Handler) http.Handler {
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

		ctx := r.Context()
		sub := ctxutil.GetUserSub(ctx)
		if sub == "" || m.userRepo == nil {
			m.logger.Warn("tenant middleware: request without a JWT subject")
			apierror.Unauthorized("missing subject claim").WriteJSON(w)
			return
		}

		user, err := m.resolveUser(ctx, sub)
		if err != nil {
			if appErr, ok := err.(*apierror.AppError); ok {
				appErr.WriteJSON(w)
				return
			}
			m.logger.WithError(err).Error("tenant middleware: user resolution failed")
			apierror.Internal("failed to resolve user").WriteJSON(w)
			return
		}

		if !user.IsActive {
			m.logger.WithField("user_id", user.ID).Warn("tenant middleware: deactivated user rejected")
			apierror.Forbidden("account is deactivated").WriteJSON(w)
			return
		}

		tenantID := user.TenantID
		userPlan := user.Plan
		if userPlan == "" {
			userPlan = "freemium" // commercial plan default
		}

		// Enterprise tenants: tenant plan takes precedence over individual user plan.
		if m.tenantRepo != nil {
			if t, err := m.tenantRepo.GetByID(tenantID); err == nil && t != nil {
				if t.Type == model.TenantTypeEnterprise && t.Plan != "" {
					userPlan = t.Plan
				}
			} else {
				m.logger.WithError(err).WithField("tenant_id", tenantID).Warn("tenant middleware: tenant record not found for user")
			}
		}

		ctx = ctxutil.WithTenantID(ctx, tenantID)
		ctx = ctxutil.WithUserID(ctx, user.ID)
		ctx = ctxutil.WithUserRole(ctx, user.Role)
		ctx = ctxutil.WithUserPlan(ctx, userPlan)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveUser returns the Ascenda user record for the authenticated subject,
// claiming a pending invitation or provisioning a new record when allowed.
// Errors of type *apierror.AppError carry the HTTP response to send.
func (m *TenantMiddleware) resolveUser(ctx context.Context, sub string) (*model.User, error) {
	jwtTenantID := ctxutil.GetTenantID(ctx)
	email := ctxutil.GetUserEmail(ctx)
	name := ctxutil.GetUserName(ctx)

	// ── 1. Existing user by identity-provider subject (authoritative) ────────
	user, err := m.userRepo.GetByExternalID(sub)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// A transient DB failure must not fall through to provisioning, or a
		// retry could create a duplicate record for the same subject.
		m.logger.WithError(err).Error("tenant middleware: user lookup failed")
		return nil, apierror.Internal("failed to resolve user")
	}
	if user != nil {
		if jwtTenantID != uuid.Nil && jwtTenantID != user.TenantID {
			m.logger.WithFields(logrus.Fields{
				"user_id":       user.ID,
				"jwt_tenant_id": jwtTenantID,
				"db_tenant_id":  user.TenantID,
			}).Warn("tenant middleware: JWT tenant_id disagrees with user record — using record")
		}
		m.refreshCachedProfile(ctx, user, email, name)
		return user, nil
	}

	// ── 2. Pending invitation matched by e-mail ──────────────────────────────
	if email == "" && m.profiler != nil {
		if profEmail, profName, profErr := m.profiler.GetCurrentUserProfile(ctx); profErr == nil {
			email, name = profEmail, profName
		} else {
			m.logger.WithError(profErr).Debug("tenant middleware: userinfo profile fetch failed")
		}
	}
	if email != "" {
		invited, err := m.userRepo.GetPendingInviteByEmail(email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			m.logger.WithError(err).Error("tenant middleware: invitation lookup failed")
			return nil, apierror.Internal("failed to resolve user")
		}
		if invited != nil {
			invited.ExternalID = sub
			if name != "" {
				invited.Name = name // IdP profile is more authoritative than what the inviter typed
			}
			now := time.Now()
			invited.JoinedAt = &now
			if err := m.userRepo.Update(invited); err != nil {
				m.logger.WithError(err).WithField("user_id", invited.ID).Error("tenant middleware: failed to claim invitation")
				return nil, apierror.Internal("failed to claim invitation")
			}
			m.logger.WithFields(logrus.Fields{
				"user_id":   invited.ID,
				"tenant_id": invited.TenantID,
				"role":      invited.Role,
			}).Info("tenant middleware: invitation claimed on first login")
			return invited, nil
		}
	}

	// ── 3. Provision into the tenant declared by the JWT, if it exists ───────
	if jwtTenantID != uuid.Nil {
		if m.tenantRepo != nil {
			if t, err := m.tenantRepo.GetByID(jwtTenantID); err != nil || t == nil {
				m.logger.WithField("tenant_id", jwtTenantID).Warn("tenant middleware: JWT tenant_id does not exist")
				return nil, apierror.Forbidden("tenant not found")
			}
		}
		return m.provision(jwtTenantID, sub, email, name)
	}

	// ── 4. No tenant resolvable ───────────────────────────────────────────────
	if m.allowDefaultTenant {
		m.logger.WithField("email", maskEmail(email)).Warn("tenant middleware: no tenant_id in JWT — provisioning into default tenant (development fallback)")
		return m.provision(DefaultTenantID, sub, email, name)
	}

	m.logger.WithField("email", maskEmail(email)).Warn("tenant middleware: authenticated user has no workspace — rejected")
	return nil, apierror.Forbidden("account is not linked to a workspace — register or ask a workspace owner for an invitation")
}

// provision creates a user record in tenantID. The first member of a tenant
// becomes owner; everyone else defaults to editor.
func (m *TenantMiddleware) provision(tenantID uuid.UUID, sub, email, name string) (*model.User, error) {
	count, err := m.userRepo.CountByTenant(tenantID)
	if err != nil {
		m.logger.WithError(err).WithField("tenant_id", tenantID).Error("tenant middleware: failed to count tenant users")
		return nil, apierror.Internal("failed to provision user")
	}
	role := "editor"
	if count == 0 {
		role = "owner"
	}
	now := time.Now()
	newUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: sub,
		Email:      email,
		Name:       name,
		Role:       role,
		Plan:       "freemium",
		IsActive:   true,
		JoinedAt:   &now,
	}
	if err := m.userRepo.Create(newUser); err != nil {
		m.logger.WithError(err).WithField("tenant_id", tenantID).Error("tenant middleware: failed to auto-provision user")
		return nil, apierror.Internal("failed to provision user")
	}
	m.logger.WithFields(logrus.Fields{
		"user_id":   newUser.ID,
		"tenant_id": tenantID,
		"role":      role,
	}).Info("tenant middleware: user auto-provisioned")
	return newUser, nil
}

// refreshCachedProfile updates the e-mail/name cached on the user record from
// the JWT claims, falling back to one IdP userinfo call while the e-mail is
// still unknown (Socrate access tokens do not embed email/name).
func (m *TenantMiddleware) refreshCachedProfile(ctx context.Context, user *model.User, email, name string) {
	changed := false
	if email != "" && user.Email != email {
		user.Email = email
		changed = true
	}
	if name != "" && user.Name != name {
		user.Name = name
		changed = true
	}
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
			m.logger.WithError(profErr).Debug("tenant middleware: userinfo profile fetch failed")
		}
	}
	if changed {
		if err := m.userRepo.Update(user); err != nil {
			m.logger.WithError(err).WithField("user_id", user.ID).Warn("tenant middleware: failed to refresh cached profile")
		}
	}
}

// maskEmail hides the local part of an e-mail address for log output.
func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return ""
	}
	return email[:1] + "***" + email[at:]
}
