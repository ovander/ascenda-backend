package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSocrateMagicLink satisfies SocrateMagicLink.
type fakeSocrateMagicLink struct {
	sentTo    []string
	sendErr   error
	sendResp  *socrate.MagicLinkResponse
	verified  []string
	verifyErr error
	result    *socrate.LoginResult
}

func (f *fakeSocrateMagicLink) SendMagicLink(_ context.Context, email string) (*socrate.MagicLinkResponse, error) {
	f.sentTo = append(f.sentTo, email)
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	if f.sendResp != nil {
		return f.sendResp, nil
	}
	return &socrate.MagicLinkResponse{}, nil
}

func (f *fakeSocrateMagicLink) VerifyMagicLink(_ context.Context, token string) (*socrate.LoginResult, error) {
	f.verified = append(f.verified, token)
	return f.result, f.verifyErr
}

func newMagicLinkTestService(f SocrateMagicLink) *MagicLinkService {
	return NewMagicLinkService(f, logrus.NewEntry(logrus.New()))
}

func appStatus(t *testing.T, err error) int {
	t.Helper()
	var appErr *apierror.AppError
	require.True(t, errors.As(err, &appErr), "expected *apierror.AppError, got %T", err)
	return appErr.StatusCode
}

func TestMagicLinkService_Send_AsksSocrateWithNormalisedEmail(t *testing.T) {
	f := &fakeSocrateMagicLink{}
	newMagicLinkTestService(f).SendMagicLink(context.Background(), "  User@Example.COM ")
	assert.Equal(t, []string{"user@example.com"}, f.sentTo)
}

func TestMagicLinkService_Send_SwallowsSocrateErrors(t *testing.T) {
	// Send must look the same whatever happens, so the endpoint cannot be used
	// to tell registered addresses from unknown ones.
	for _, err := range []error{socrate.ErrMagicLinkRateLimited, errors.New("socrate down")} {
		f := &fakeSocrateMagicLink{sendErr: err}
		assert.NotPanics(t, func() {
			newMagicLinkTestService(f).SendMagicLink(context.Background(), "u@x.com")
		})
		assert.Len(t, f.sentTo, 1)
	}
}

func TestMagicLinkService_Send_DevelopmentLink(t *testing.T) {
	f := &fakeSocrateMagicLink{sendResp: &socrate.MagicLinkResponse{MagicURL: "http://localhost:5173/magic-link?token=t"}}
	assert.NotPanics(t, func() {
		newMagicLinkTestService(f).SendMagicLink(context.Background(), "u@x.com")
	})
}

func TestMagicLinkService_Send_NotConfigured(t *testing.T) {
	assert.NotPanics(t, func() {
		NewMagicLinkService(nil, logrus.NewEntry(logrus.New())).SendMagicLink(context.Background(), "u@x.com")
	})
}

func TestMagicLinkService_Verify_ReturnsSocrateTokens(t *testing.T) {
	f := &fakeSocrateMagicLink{result: &socrate.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 900}}
	got, err := newMagicLinkTestService(f).VerifyMagicLink(context.Background(), " tok ")
	require.NoError(t, err)
	assert.Equal(t, "at", got.AccessToken)
	assert.Equal(t, "rt", got.RefreshToken)
	assert.Equal(t, []string{"tok"}, f.verified)
}

func TestMagicLinkService_Verify_InvalidOrUsedTokenIs401(t *testing.T) {
	for _, err := range []error{socrate.ErrMagicLinkInvalid, socrate.ErrMagicLinkAlreadyUsed} {
		f := &fakeSocrateMagicLink{verifyErr: err}
		_, got := newMagicLinkTestService(f).VerifyMagicLink(context.Background(), "tok")
		assert.Equal(t, http.StatusUnauthorized, appStatus(t, got), "for %v", err)
	}
}

func TestMagicLinkService_Verify_SocrateFailureIs500(t *testing.T) {
	f := &fakeSocrateMagicLink{verifyErr: errors.New("connection refused")}
	_, err := newMagicLinkTestService(f).VerifyMagicLink(context.Background(), "tok")
	assert.Equal(t, http.StatusInternalServerError, appStatus(t, err))
}

func TestMagicLinkService_Verify_EmptyResultIs500(t *testing.T) {
	f := &fakeSocrateMagicLink{result: &socrate.LoginResult{}}
	_, err := newMagicLinkTestService(f).VerifyMagicLink(context.Background(), "tok")
	assert.Equal(t, http.StatusInternalServerError, appStatus(t, err))
}

func TestMagicLinkService_Verify_NotConfiguredIs503(t *testing.T) {
	_, err := NewMagicLinkService(nil, logrus.NewEntry(logrus.New())).VerifyMagicLink(context.Background(), "tok")
	assert.Equal(t, http.StatusServiceUnavailable, appStatus(t, err))
}
