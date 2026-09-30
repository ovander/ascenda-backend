package service

import (
	"context"
	"errors"
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTokens struct {
	ts        *socrate.TokenSet
	err       error
	revoked   []string
	revokeErr error
}

func (f *fakeTokens) ExchangeCode(context.Context, string, string, string) (*socrate.TokenSet, error) {
	return f.ts, f.err
}
func (f *fakeTokens) RefreshToken(context.Context, string) (*socrate.TokenSet, error) {
	return f.ts, f.err
}
func (f *fakeTokens) RevokeToken(_ context.Context, token string) error {
	f.revoked = append(f.revoked, token)
	return f.revokeErr
}

type fakeUserInfo struct {
	p      *socrate.ProfileInfo
	err    error
	gotJWT string
}

func (f *fakeUserInfo) GetCurrentUserProfile(ctx context.Context) (*socrate.ProfileInfo, error) {
	f.gotJWT = ctxutil.GetRawJWT(ctx)
	return f.p, f.err
}

func newSessionAuth(tok *fakeTokens, ui SocrateUserInfo, users *MockUserRepo) *SessionAuthService {
	le := logrus.NewEntry(logrus.New())
	if users == nil {
		// A typed nil *MockUserRepo would not be a nil interface.
		return NewSessionAuthService(NewTokenService(tok, le), NewMagicLinkService(nil, le), ui, nil, le)
	}
	return NewSessionAuthService(NewTokenService(tok, le), NewMagicLinkService(nil, le), ui, users, le)
}

func TestSessionAuth_SignInReadsProfileWithTheNewAccessToken(t *testing.T) {
	tok := &fakeTokens{ts: &socrate.TokenSet{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 900}}
	ui := &fakeUserInfo{p: &socrate.ProfileInfo{Sub: "42", Email: "new@example.test", GivenName: "Ada", FamilyName: "Lovelace", FirstName: "Ada", LastName: "Lovelace"}}
	users := NewMockUserRepo()
	u := &model.User{ID: uuid.New(), TenantID: uuid.New(), ExternalID: "42", Email: "old@example.test", Name: "Old"}
	require.NoError(t, users.Create(u))

	ts, id, err := newSessionAuth(tok, ui, users).SignIn(context.Background(), "code", "https://app/bff/callback", "verifier")

	require.NoError(t, err)
	assert.Equal(t, "rt", ts.RefreshToken)
	assert.Equal(t, "at", ui.gotJWT)
	assert.Equal(t, SessionIdentity{Sub: "42", Email: "new@example.test", Name: "Ada Lovelace"}, id)
	assert.Equal(t, "new@example.test", u.Email, "the cached profile follows Socrate")
	assert.Equal(t, "Ada Lovelace", u.Name)
}

func TestSessionAuth_SignInFailures(t *testing.T) {
	ok := &socrate.TokenSet{AccessToken: "at", RefreshToken: "rt"}
	cases := map[string]struct {
		tok    *fakeTokens
		ui     SocrateUserInfo
		status int
	}{
		"code refused":        {&fakeTokens{err: &socrate.OAuthError{StatusCode: 400, Code: "invalid_grant"}}, &fakeUserInfo{}, 401},
		"no access token":     {&fakeTokens{ts: &socrate.TokenSet{}}, &fakeUserInfo{}, 500},
		"userinfo fails":      {&fakeTokens{ts: ok}, &fakeUserInfo{err: errors.New("down")}, 500},
		"userinfo 401":        {&fakeTokens{ts: ok}, &fakeUserInfo{}, 500},
		"profile without sub": {&fakeTokens{ts: ok}, &fakeUserInfo{p: &socrate.ProfileInfo{Email: "x@y"}}, 500},
		"no userinfo client":  {&fakeTokens{ts: ok}, nil, 503},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := newSessionAuth(tc.tok, tc.ui, nil).SignIn(context.Background(), "c", "r", "v")
			var ae *apierror.AppError
			require.ErrorAs(t, err, &ae)
			assert.Equal(t, tc.status, ae.StatusCode)
		})
	}
}

func TestSessionAuth_SignOutRevokesTheRefreshToken(t *testing.T) {
	tok := &fakeTokens{}
	svc := newSessionAuth(tok, &fakeUserInfo{}, nil)
	require.NoError(t, svc.SignOut(context.Background(), "rt-9"))
	require.NoError(t, svc.SignOut(context.Background(), ""), "nothing to revoke")
	assert.Equal(t, []string{"rt-9"}, tok.revoked)

	tok.revokeErr = errors.New("down")
	assert.Error(t, svc.SignOut(context.Background(), "rt-10"))
}

func TestSessionAuth_MagicLinkNotConfigured(t *testing.T) {
	_, _, err := newSessionAuth(&fakeTokens{}, &fakeUserInfo{}, nil).SignInWithMagicLink(context.Background(), "tok")
	var ae *apierror.AppError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 503, ae.StatusCode)
}

func TestProfileName(t *testing.T) {
	assert.Equal(t, "Full", profileName(&socrate.ProfileInfo{Name: " Full ", DisplayName: "D"}))
	assert.Equal(t, "D", profileName(&socrate.ProfileInfo{DisplayName: "D"}))
	assert.Equal(t, "A B", profileName(&socrate.ProfileInfo{FirstName: "A", LastName: "B"}))
	assert.Equal(t, "", profileName(&socrate.ProfileInfo{}))
}

func TestSyncProfile_UnknownUserIsLeftToTheTenantMiddleware(t *testing.T) {
	users := NewMockUserRepo()
	svc := newSessionAuth(&fakeTokens{}, &fakeUserInfo{}, users)
	svc.syncProfile(SessionIdentity{Sub: "404", Email: "a@b"})
	assert.Empty(t, users.users, "no record created")
}
