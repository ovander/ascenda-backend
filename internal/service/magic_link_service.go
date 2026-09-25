package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// SocrateMailer can dispatch a passwordless sign-in email via Socrate.
// Satisfied by *socrate.Client. Socrate generates the magic link server-side;
// the verify URL is configured on the Socrate application, not passed per-call.
type SocrateMailer interface {
	SendMagicLink(ctx context.Context, email string) (*socrate.MagicLinkResponse, error)
}

const (
	magicLinkTTL      = 15 * time.Minute
	magicLinkRawBytes = 32 // 256-bit entropy → 43-char base64url token
)

// magicLinkTokenRepo is the narrow storage interface consumed by MagicLinkService.
// Satisfied by *repo.MagicLinkRepo; can be replaced with an in-memory stub in tests.
type magicLinkTokenRepo interface {
	Create(t *model.MagicLinkToken) error
	FindByTokenHash(hash string) (*model.MagicLinkToken, error)
	MarkUsed(id uuid.UUID) error
}

// MagicLinkService handles generation, delivery, and verification of single-use
// passwordless sign-in tokens.
type MagicLinkService struct {
	mailer     SocrateMailer // nil → skip email (dev mode)
	tokenRepo  magicLinkTokenRepo
	appBaseURL string // e.g. "https://api.ascenda.io" — used to build verify URLs
	logger     *logrus.Entry
}

// NewMagicLinkService creates a MagicLinkService.
// mailer may be nil; tokens are still generated/stored but no email is dispatched.
func NewMagicLinkService(
	mailer SocrateMailer,
	tokenRepo magicLinkTokenRepo,
	appBaseURL string,
	logger *logrus.Entry,
) *MagicLinkService {
	return &MagicLinkService{
		mailer:     mailer,
		tokenRepo:  tokenRepo,
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
		logger:     logger,
	}
}

// SendMagicLink generates a sign-in token, persists it, and asks Socrate to
// deliver an email containing the verify link to the given address.
// The response is always 202 regardless of whether the address is registered —
// this prevents account enumeration.
func (s *MagicLinkService) SendMagicLink(ctx context.Context, email, redirectURL string) error {
	rawToken, tokenHash, err := generateToken()
	if err != nil {
		s.logger.WithError(err).Error("magic-link: failed to generate token")
		return apierror.Internal("failed to generate sign-in link")
	}

	t := &model.MagicLinkToken{
		ID:          uuid.New(),
		Email:       strings.ToLower(strings.TrimSpace(email)),
		TokenHash:   tokenHash,
		RedirectURL: redirectURL,
		ExpiresAt:   time.Now().Add(magicLinkTTL),
	}
	if err := s.tokenRepo.Create(t); err != nil {
		s.logger.WithError(err).WithField("email", email).Error("magic-link: failed to store token")
		return apierror.Internal("failed to generate sign-in link")
	}

	// Build the local verify URL for dev logging (Socrate sends its own link in production).
	verifyURL := fmt.Sprintf("%s/auth/magic-link/verify?token=%s", s.appBaseURL, rawToken)

	if s.mailer != nil {
		if _, err := s.mailer.SendMagicLink(ctx, email); err != nil {
			// Log but do NOT fail — token is stored, a retry or manual link is possible.
			s.logger.WithError(err).WithField("email", email).Warn("magic-link: Socrate email dispatch failed (non-fatal)")
		}
	} else {
		// Dev mode: log the verify link so engineers can use it without an email server.
		s.logger.WithFields(logrus.Fields{
			"email":      email,
			"verify_url": verifyURL,
		}).Info("magic-link: mailer not configured — verify URL logged for dev use")
	}

	return nil
}

// MagicLinkVerifyResult carries the outcome of a successful token verification.
type MagicLinkVerifyResult struct {
	Email       string
	RedirectURL string // may be empty
}

// VerifyMagicLink validates a raw token, marks it used, and returns the
// associated email and post-auth redirect URL.
func (s *MagicLinkService) VerifyMagicLink(ctx context.Context, rawToken string) (*MagicLinkVerifyResult, error) {
	hash := hashToken(rawToken)
	t, err := s.tokenRepo.FindByTokenHash(hash)
	if err != nil {
		s.logger.WithError(err).Error("magic-link: DB error during verification")
		return nil, apierror.Internal("verification failed")
	}
	if t == nil {
		// Token not found, expired, or already used — return 401, not 404.
		return nil, apierror.Unauthorized("sign-in link is invalid or has already been used")
	}

	if err := s.tokenRepo.MarkUsed(t.ID); err != nil {
		s.logger.WithError(err).WithField("token_id", t.ID).Error("magic-link: failed to mark token used")
		return nil, apierror.Internal("verification failed")
	}

	s.logger.WithField("email", t.Email).Info("magic-link: token verified successfully")

	return &MagicLinkVerifyResult{
		Email:       t.Email,
		RedirectURL: t.RedirectURL,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// generateToken produces a cryptographically random raw token and its SHA-256
// hex hash.  The raw token is sent in the email link; only the hash is stored.
func generateToken() (rawToken, tokenHash string, err error) {
	b := make([]byte, magicLinkRawBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	rawToken = hex.EncodeToString(b) // 64-char hex string
	tokenHash = hashToken(rawToken)
	return rawToken, tokenHash, nil
}

// hashToken returns the hex-encoded SHA-256 of a raw token string.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
