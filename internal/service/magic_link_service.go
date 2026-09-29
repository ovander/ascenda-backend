package service

import (
	"context"
	"errors"
	"strings"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// SocrateMagicLink sends and redeems Socrate's passwordless sign-in links.
// Satisfied by *socrate.Client.
//
// Socrate owns the whole link: it generates the single-use token, e-mails the
// link (to the magic-link URL configured on the Socrate application, i.e. the
// frontend's /magic-link page) and redeems the token for a token set. The
// redemption is not an authorization request, so PKCE does not apply.
type SocrateMagicLink interface {
	SendMagicLink(ctx context.Context, email string) (*socrate.MagicLinkResponse, error)
	VerifyMagicLink(ctx context.Context, token string) (*socrate.LoginResult, error)
}

// MagicLinkService handles passwordless sign-in through Socrate.
type MagicLinkService struct {
	socrate SocrateMagicLink // nil → Socrate not configured (development)
	logger  *logrus.Entry
}

// NewMagicLinkService creates a MagicLinkService. sl may be nil when Socrate is
// not configured: requests are then accepted and logged, and verification fails.
func NewMagicLinkService(sl SocrateMagicLink, logger *logrus.Entry) *MagicLinkService {
	return &MagicLinkService{socrate: sl, logger: logger}
}

// SendMagicLink asks Socrate to e-mail a sign-in link to the given address.
// It never fails towards the caller, whether or not the address is registered
// and even when Socrate refuses (rate limit, outage), so the endpoint cannot be
// used to enumerate accounts. Failures are logged.
func (s *MagicLinkService) SendMagicLink(ctx context.Context, email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if s.socrate == nil {
		s.logger.WithField("email", email).Warn("magic-link: Socrate not configured — no e-mail sent")
		return
	}
	resp, err := s.socrate.SendMagicLink(ctx, email)
	switch {
	case errors.Is(err, socrate.ErrMagicLinkRateLimited):
		s.logger.WithField("email", email).Warn("magic-link: Socrate rate limit reached for this address")
	case err != nil:
		s.logger.WithError(err).WithField("email", email).Warn("magic-link: Socrate e-mail dispatch failed")
	case resp != nil && resp.MagicURL != "":
		// Socrate returns the link itself only in development mode.
		s.logger.WithFields(logrus.Fields{"email": email, "magic_url": resp.MagicURL}).
			Info("magic-link: development link")
	}
}

// VerifyMagicLink redeems a Socrate magic-link token for a token set. An
// invalid, expired or already-used token is a 401.
func (s *MagicLinkService) VerifyMagicLink(ctx context.Context, token string) (*socrate.LoginResult, error) {
	if s.socrate == nil {
		return nil, apierror.ServiceUnavailable("magic-link sign-in is not configured")
	}
	result, err := s.socrate.VerifyMagicLink(ctx, strings.TrimSpace(token))
	switch {
	case errors.Is(err, socrate.ErrMagicLinkInvalid), errors.Is(err, socrate.ErrMagicLinkAlreadyUsed):
		return nil, apierror.Unauthorized("sign-in link is invalid, expired or already used")
	case err != nil:
		s.logger.WithError(err).Error("magic-link: Socrate verification failed")
		return nil, apierror.Internal("verification failed")
	case result == nil || result.AccessToken == "":
		s.logger.Error("magic-link: Socrate verification returned no access token")
		return nil, apierror.Internal("verification failed")
	}
	return result, nil
}
