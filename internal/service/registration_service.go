package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// SocrateRegistrar can create a user in Socrate via two strategies:
//
//   - CreateUser  — forwards the caller's JWT already stored in ctx by AuthMiddleware
//     (mirrors GPWA's socrateContext pattern). Works when the inviter is
//     a Socrate admin. Used as the primary invite path in InviteUser.
//   - RegisterUser — uses the backend service-account (client_credentials) token.
//     Used for self-service registration where no user JWT is present.
//
// Both methods are satisfied by *socrate.Client.
type SocrateRegistrar interface {
	CreateUser(ctx context.Context, req socrate.CreateUserRequest) (*socrate.CreateUserResult, error)
	RegisterUser(ctx context.Context, req socrate.CreateUserRequest) (*socrate.CreateUserResult, error)
}

// validCountries is the allow-list shown on the registration form.
var validCountries = map[string]bool{
	"BE": true, "FR": true, "LU": true, "NL": true, "DE": true,
	"GB": true, "CH": true, "ES": true, "IT": true, "PT": true,
	"IE": true, "US": true, "CA": true,
}

// slugRe strips characters that are not alphanumeric or hyphens.
var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

// RegistrationService handles self-service account creation.
type RegistrationService struct {
	socrateClient SocrateRegistrar // nil → skip Socrate (local dev without IdP)
	userRepo      repo.UserRepository
	tenantRepo    repo.AdminTenantRepository
	seedService   *SeedService
	logger        *logrus.Entry
}

// NewRegistrationService creates a RegistrationService.
// socrateClient may be nil; the service degrades gracefully (tenant + local user
// are still created, but no Socrate call is made and no email is sent).
func NewRegistrationService(
	socrateClient SocrateRegistrar,
	userRepo repo.UserRepository,
	tenantRepo repo.AdminTenantRepository,
	seedService *SeedService,
	logger *logrus.Entry,
) *RegistrationService {
	return &RegistrationService{
		socrateClient: socrateClient,
		userRepo:      userRepo,
		tenantRepo:    tenantRepo,
		seedService:   seedService,
		logger:        logger,
	}
}

// RegisterRequest is the payload from the public registration form.
type RegisterRequest struct {
	FirstName   string `json:"firstName"   validate:"required"`
	LastName    string `json:"lastName"    validate:"required"`
	CompanyName string `json:"companyName" validate:"required"`
	Email       string `json:"email"       validate:"required,email"`
	Country     string `json:"country"     validate:"required"`
}

// RegisterResult is returned to the caller on success.
type RegisterResult struct {
	Message string `json:"message"`
}

// Register creates a new free-tier account:
//  1. Validate inputs (country allow-list, non-empty required fields).
//  2. Create the user in Socrate (triggers verification email) — skipped when
//     Socrate is not configured (dev mode).
//  3. Create a Tenant (free tier, MaxPlans=1, MaxUsers=2).
//  4. Create a local User record (role=owner, externalID linked to Socrate).
//  5. Seed the three demo plans.
func (s *RegistrationService) Register(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	// ── 1. Validate ────────────────────────────────────────────────────────
	if !validCountries[strings.ToUpper(req.Country)] {
		return nil, apierror.BadRequest(fmt.Sprintf("unsupported country: %s", req.Country))
	}

	fullName := strings.TrimSpace(req.FirstName + " " + req.LastName)
	slug := buildSlug(req.CompanyName)

	// ── 2. Socrate user creation ────────────────────────────────────────────
	var socrateExternalID string
	if s.socrateClient != nil {
		socrateResult, err := s.socrateClient.RegisterUser(ctx, socrate.CreateUserRequest{
			Email: req.Email,
			Name:  fullName,
			Role:  "user",
		})
		if err != nil {
			if errors.Is(err, socrate.ErrUserAlreadyExists) {
				return nil, apierror.Conflict("an account with this email already exists")
			}
			s.logger.WithError(err).WithField("email", req.Email).Error("Socrate RegisterUser failed")
			return nil, apierror.Internal("failed to create account in identity provider")
		}
		socrateExternalID = fmt.Sprintf("%d", socrateResult.UserID)
		s.logger.WithFields(logrus.Fields{
			"email":     req.Email,
			"socrateID": socrateResult.UserID,
		}).Info("registration: Socrate user created")
	} else {
		s.logger.WithField("email", req.Email).Debug("registration: Socrate not configured, skipping IdP step")
	}

	// ── 3. Create tenant ────────────────────────────────────────────────────
	tenantID := uuid.New()
	tenant := &model.Tenant{
		ID:       tenantID,
		Name:     req.CompanyName,
		Slug:     slug,
		IsActive: true,
	}
	if err := s.tenantRepo.Create(tenant); err != nil {
		s.logger.WithError(err).WithField("email", req.Email).Error("registration: failed to create tenant")
		return nil, apierror.Internal("failed to create account")
	}
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"slug":      slug,
	}).Info("registration: tenant created")

	// ── 4. Create local user record (owner of the new tenant) ───────────────
	now := time.Now()
	localUser := &model.User{
		ID:         uuid.New(),
		TenantID:   tenantID,
		ExternalID: socrateExternalID, // empty string until first login if Socrate skipped
		Email:      req.Email,
		Name:       fullName,
		Role:       "owner",
		Plan:       "freemium",
		IsActive:   true,
		JoinedAt:   &now,
	}
	if err := s.userRepo.Create(localUser); err != nil {
		s.logger.WithError(err).WithField("email", req.Email).Error("registration: failed to create local user")
		return nil, apierror.Internal("failed to create account")
	}
	s.logger.WithField("user_id", localUser.ID).Info("registration: local user created")

	// ── 5. Seed demo plans (non-fatal on failure) ────────────────────────────
	if s.seedService != nil {
		if err := s.seedService.EnsureDemoPlans(ctx, tenantID, localUser.ID); err != nil {
			// Seeding failure is non-fatal: the user can still log in and create plans.
			s.logger.WithError(err).WithField("tenant_id", tenantID).Warn("registration: demo plan seeding failed (non-fatal)")
		}
	}

	s.logger.WithFields(logrus.Fields{
		"email":     req.Email,
		"tenant_id": tenantID,
	}).Info("registration: account created successfully")

	return &RegisterResult{
		Message: "Account created. Check your email for the sign-in link.",
	}, nil
}

// buildSlug converts a company name to a URL-safe slug.
// e.g. "Acme Corp." → "acme-corp", collisions get a UUID suffix appended by the caller.
func buildSlug(companyName string) string {
	s := strings.ToLower(strings.TrimSpace(companyName))
	// Replace spaces and punctuation with hyphens.
	s = strings.ReplaceAll(s, " ", "-")
	// Strip anything that isn't a-z 0-9 or hyphen.
	s = slugRe.ReplaceAllString(s, "")
	// Collapse multiple consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "company"
	}
	// Append a short UUID fragment to guarantee uniqueness.
	suffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	return s + "-" + suffix
}
