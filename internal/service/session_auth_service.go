package service

import (
	"context"
	"errors"
	"strings"

	"ascenda/internal/repo"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SocrateUserInfo reads the signed-in user's profile from Socrate's userinfo
// endpoint with the access token in ctx (socrate.WithJWT). Satisfied by
// *socrate.Client. Socrate's access tokens carry no e-mail or name, so the BFF
// asks once per sign-in.
type SocrateUserInfo interface {
	GetCurrentUserProfile(ctx context.Context) (*socrate.ProfileInfo, error)
}

// SessionIdentity is who signed in, as the BFF keeps it on the server-side
// session and shows it to the SPA. It holds no token.
type SessionIdentity struct {
	Sub   string
	Email string
	Name  string
}

// SessionAuthService runs the Backend-for-Frontend sign-in: it redeems an
// authorization code or a magic-link token for a token set, reads the user's
// profile, and keeps Ascenda's user record in step with it. The tokens it
// returns go on the server-side session only, never to the browser.
type SessionAuthService struct {
	tokens   *TokenService
	magic    *MagicLinkService
	profiles SocrateUserInfo     // nil → Socrate not configured
	users    repo.UserRepository // nil → no profile sync (tests)
	logger   *logrus.Entry
}

// NewSessionAuthService creates a SessionAuthService.
func NewSessionAuthService(tokens *TokenService, magic *MagicLinkService, profiles SocrateUserInfo, users repo.UserRepository, logger *logrus.Entry) *SessionAuthService {
	return &SessionAuthService{tokens: tokens, magic: magic, profiles: profiles, users: users, logger: logger}
}

// SignIn exchanges an authorization code and its PKCE verifier, then reads the
// user's profile. A code Socrate refuses is a 401.
func (s *SessionAuthService) SignIn(ctx context.Context, code, redirectURI, verifier string) (*socrate.TokenSet, SessionIdentity, error) {
	ts, err := s.tokens.ExchangeCode(ctx, code, redirectURI, verifier)
	if err != nil {
		return nil, SessionIdentity{}, err
	}
	return s.identify(ctx, ts)
}

// SignInWithMagicLink redeems a Socrate magic-link token, then reads the
// user's profile. An invalid, expired or spent link is a 401.
func (s *SessionAuthService) SignInWithMagicLink(ctx context.Context, token string) (*socrate.TokenSet, SessionIdentity, error) {
	res, err := s.magic.VerifyMagicLink(ctx, token)
	if err != nil {
		return nil, SessionIdentity{}, err
	}
	ts := &socrate.TokenSet{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		IDToken:      res.IDToken,
		TokenType:    res.TokenType,
		ExpiresIn:    res.ExpiresIn,
		Roles:        res.Roles,
		AppRoles:     res.AppRoles,
	}
	return s.identify(ctx, ts)
}

// SignOut revokes the session's refresh token, which ends its rotation chain at
// Socrate (Socrate has no end_session endpoint).
func (s *SessionAuthService) SignOut(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.tokens.RevokeToken(ctx, refreshToken)
}

// identify reads the profile for a fresh token set and updates the user's
// cached e-mail and name. A session needs a subject, so a token set without a
// readable profile fails the sign-in.
func (s *SessionAuthService) identify(ctx context.Context, ts *socrate.TokenSet) (*socrate.TokenSet, SessionIdentity, error) {
	if ts == nil || ts.AccessToken == "" {
		s.logger.Error("bff: Socrate returned no access token")
		return nil, SessionIdentity{}, apierror.Internal("sign-in failed")
	}
	if ts.RefreshToken == "" {
		// Without a refresh token the session would end at the first expiry.
		s.logger.Warn("bff: Socrate returned no refresh token; the session ends when the access token expires")
	}
	if s.profiles == nil {
		return nil, SessionIdentity{}, errSocrateNotConfigured()
	}
	p, err := s.profiles.GetCurrentUserProfile(socrate.WithJWT(ctx, ts.AccessToken))
	if err != nil || p == nil || p.Sub == "" {
		s.logger.WithError(err).Error("bff: userinfo failed after sign-in")
		return nil, SessionIdentity{}, apierror.Internal("sign-in failed")
	}
	id := SessionIdentity{Sub: p.Sub, Email: p.Email, Name: profileName(p)}
	s.syncProfile(id)
	return ts, id, nil
}

// profileName prefers the full name, then the given and family names.
func profileName(p *socrate.ProfileInfo) string {
	if n := strings.TrimSpace(p.Name); n != "" {
		return n
	}
	if n := strings.TrimSpace(p.DisplayName); n != "" {
		return n
	}
	return strings.TrimSpace(p.FirstName + " " + p.LastName)
}

// syncProfile copies the profile's e-mail and name onto the user record found
// by subject. A user without a record yet is left alone: the tenant middleware
// provisions it on the first API call, as before. Failures are logged only.
func (s *SessionAuthService) syncProfile(id SessionIdentity) {
	if s.users == nil || (id.Email == "" && id.Name == "") {
		return
	}
	user, err := s.users.GetByExternalID(id.Sub)
	if err != nil || user == nil {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.WithError(err).Warn("bff: user lookup failed during profile sync")
		}
		return
	}
	changed := false
	if id.Email != "" && user.Email != id.Email {
		user.Email = id.Email
		changed = true
	}
	if id.Name != "" && user.Name != id.Name {
		user.Name = id.Name
		changed = true
	}
	if !changed {
		return
	}
	if err := s.users.Update(user); err != nil {
		s.logger.WithError(err).WithField("sub", id.Sub).Warn("bff: failed to update the user's profile")
	}
}
