package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubSocrateTokens struct {
	tokens *socrate.TokenSet
	err    error
}

func (s stubSocrateTokens) ExchangeCode(context.Context, string, string, string) (*socrate.TokenSet, error) {
	return s.tokens, s.err
}
func (s stubSocrateTokens) RefreshToken(context.Context, string) (*socrate.TokenSet, error) {
	return s.tokens, s.err
}
func (s stubSocrateTokens) RevokeToken(context.Context, string) error { return s.err }

func statusOf(t *testing.T, err error) int {
	t.Helper()
	var appErr *apierror.AppError
	require.True(t, errors.As(err, &appErr), "want *apierror.AppError, got %T: %v", err, err)
	return appErr.StatusCode
}

func TestTokenService_MapsSocrateErrors(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	rejected := &socrate.OAuthError{StatusCode: http.StatusBadRequest, Code: "invalid_grant"}
	transport := errors.New("dial tcp: connection refused")

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"Socrate refuses the grant", rejected, http.StatusUnauthorized},
		{"Socrate answers 5xx", &socrate.OAuthError{StatusCode: http.StatusBadGateway}, http.StatusUnauthorized},
		{"transport or decoding failure", transport, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewTokenService(stubSocrateTokens{err: tc.err}, logger)

			_, err := s.ExchangeCode(context.Background(), "code", "https://app/callback", "")
			assert.Equal(t, tc.want, statusOf(t, err), "exchange")

			_, err = s.RefreshToken(context.Background(), "rt")
			assert.Equal(t, tc.want, statusOf(t, err), "refresh")
		})
	}

	t.Run("revocation failure is 500", func(t *testing.T) {
		s := NewTokenService(stubSocrateTokens{err: rejected}, logger)
		assert.Equal(t, http.StatusInternalServerError, statusOf(t, s.RevokeToken(context.Background(), "rt")))
	})
}

func TestTokenService_PassesTokensThrough(t *testing.T) {
	want := &socrate.TokenSet{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 900}
	s := NewTokenService(stubSocrateTokens{tokens: want}, logrus.NewEntry(logrus.New()))

	got, err := s.ExchangeCode(context.Background(), "code", "https://app/callback", "v")
	require.NoError(t, err)
	assert.Same(t, want, got)

	got, err = s.RefreshToken(context.Background(), "rt-old")
	require.NoError(t, err)
	assert.Same(t, want, got)

	assert.NoError(t, s.RevokeToken(context.Background(), "rt"))
}

// deadlineSocrate reports the deadline each call's context carries.
type deadlineSocrate struct{ deadlines *[]time.Duration }

func (d deadlineSocrate) record(ctx context.Context) {
	if dl, ok := ctx.Deadline(); ok {
		*d.deadlines = append(*d.deadlines, time.Until(dl))
	}
}
func (d deadlineSocrate) ExchangeCode(ctx context.Context, _, _, _ string) (*socrate.TokenSet, error) {
	d.record(ctx)
	return &socrate.TokenSet{}, nil
}
func (d deadlineSocrate) RefreshToken(ctx context.Context, _ string) (*socrate.TokenSet, error) {
	d.record(ctx)
	return &socrate.TokenSet{}, nil
}
func (d deadlineSocrate) RevokeToken(ctx context.Context, _ string) error {
	d.record(ctx)
	return nil
}

func TestTokenService_BoundsEachCallTo10Seconds(t *testing.T) {
	var deadlines []time.Duration
	s := NewTokenService(deadlineSocrate{&deadlines}, logrus.NewEntry(logrus.New()))

	_, _ = s.ExchangeCode(context.Background(), "code", "uri", "")
	_, _ = s.RefreshToken(context.Background(), "rt")
	_ = s.RevokeToken(context.Background(), "rt")

	require.Len(t, deadlines, 3, "every call carries a deadline")
	for _, d := range deadlines {
		assert.InDelta(t, tokenCallTimeout.Seconds(), d.Seconds(), 1)
	}
}

func TestTokenService_WithoutSocrateIs503(t *testing.T) {
	s := NewTokenService(nil, logrus.NewEntry(logrus.New()))

	_, err := s.ExchangeCode(context.Background(), "code", "uri", "")
	assert.Equal(t, http.StatusServiceUnavailable, statusOf(t, err))
	_, err = s.RefreshToken(context.Background(), "rt")
	assert.Equal(t, http.StatusServiceUnavailable, statusOf(t, err))
	assert.Equal(t, http.StatusServiceUnavailable, statusOf(t, s.RevokeToken(context.Background(), "rt")))
}
