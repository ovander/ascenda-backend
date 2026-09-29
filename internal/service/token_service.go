package service

import (
	"context"
	"errors"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// SocrateTokens issues, refreshes and revokes a user's tokens at Socrate's
// OAuth endpoints (/oauth/token, /oauth/revoke). Satisfied by *socrate.Client,
// which also forwards the browser's address and User-Agent when the context
// carries a socrate.ClientAttribution (see middleware.SocrateClientAttribution).
type SocrateTokens interface {
	ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*socrate.TokenSet, error)
	RefreshToken(ctx context.Context, refreshToken string) (*socrate.TokenSet, error)
	RevokeToken(ctx context.Context, token string) error
}

// tokenCallTimeout bounds each call to Socrate's token endpoints. The shared
// socrate.Client allows 30 s, which suits admin calls; a browser waiting on a
// sign-in or refresh gives up sooner.
const tokenCallTimeout = 10 * time.Second

// TokenService runs the token grants of the sign-in flow on the browser's
// behalf: code exchange, refresh and revocation at logout.
type TokenService struct {
	socrate SocrateTokens // nil → Socrate not configured (development)
	logger  *logrus.Entry
}

// NewTokenService creates a TokenService. st may be nil when Socrate is not
// configured; every call then fails with 503.
func NewTokenService(st SocrateTokens, logger *logrus.Entry) *TokenService {
	return &TokenService{socrate: st, logger: logger}
}

// ExchangeCode exchanges an authorization code (and its PKCE verifier, if any)
// for a token set. A grant Socrate rejects is a 401; a transport or decoding
// failure is a 500.
func (s *TokenService) ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*socrate.TokenSet, error) {
	if s.socrate == nil {
		return nil, errSocrateNotConfigured()
	}
	ctx, cancel := context.WithTimeout(ctx, tokenCallTimeout)
	defer cancel()
	ts, err := s.socrate.ExchangeCode(ctx, code, redirectURI, codeVerifier)
	if err != nil {
		return nil, s.grantError(err, "token exchange failed", "failed to exchange authorization code")
	}
	return ts, nil
}

// RefreshToken exchanges a refresh token for a new token set. Socrate rotates
// refresh tokens, so the caller must keep the one returned. A rejected refresh
// token (spent, expired, revoked) is a 401; any other failure is a 500.
func (s *TokenService) RefreshToken(ctx context.Context, refreshToken string) (*socrate.TokenSet, error) {
	if s.socrate == nil {
		return nil, errSocrateNotConfigured()
	}
	ctx, cancel := context.WithTimeout(ctx, tokenCallTimeout)
	defer cancel()
	ts, err := s.socrate.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, s.grantError(err, "token refresh failed", "failed to refresh token")
	}
	return ts, nil
}

// RevokeToken revokes an access or refresh token (RFC 7009).
func (s *TokenService) RevokeToken(ctx context.Context, token string) error {
	if s.socrate == nil {
		return errSocrateNotConfigured()
	}
	ctx, cancel := context.WithTimeout(ctx, tokenCallTimeout)
	defer cancel()
	if err := s.socrate.RevokeToken(ctx, token); err != nil {
		s.logger.WithError(err).Warn("socrate: token revocation failed")
		return apierror.Internal("token revocation failed")
	}
	return nil
}

// grantError maps a /oauth/token failure: Socrate's refusal (*socrate.OAuthError)
// becomes 401 with rejected, anything else 500 with failed. The cause is logged,
// never returned to the browser.
func (s *TokenService) grantError(err error, rejected, failed string) error {
	var oe *socrate.OAuthError
	if errors.As(err, &oe) {
		s.logger.WithFields(logrus.Fields{"status": oe.StatusCode, "oauth_error": oe.Code}).
			Warn("socrate: " + rejected)
		return apierror.Unauthorized(rejected)
	}
	s.logger.WithError(err).Error("socrate: " + failed)
	return apierror.Internal(failed)
}

func errSocrateNotConfigured() error {
	return apierror.ServiceUnavailable("sign-in is not configured")
}
