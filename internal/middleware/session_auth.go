package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/bff"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// sessionTouchInterval is how often a request slides the session's idle
// window (bff.DefaultTouchInterval): enough for an idle timeout in tens of
// minutes.
const sessionTouchInterval = bff.DefaultTouchInterval

// SessionAuth puts the BFF session in front of the API. It runs before
// AuthMiddleware and turns a session cookie into the bearer the rest of the
// chain already understands:
//
//   - no session → 401, and nothing further runs;
//   - POST, PUT, PATCH or DELETE without the session's X-CSRF-Token → 403;
//   - an access token about to expire is refreshed (bff.Gateway.EnsureFresh:
//     once per session under concurrent requests, the rotated refresh token
//     kept); a refresh Socrate rejects ends the session → 401; Socrate
//     unreachable → 502, the session kept;
//   - otherwise the request goes on with Authorization: Bearer <access token>,
//     replacing any the browser sent, so jwtauth, the tenant middleware and
//     the handlers run unchanged.
//
// A bearer the browser sends is never enough on its own: without a session the
// request is refused before AuthMiddleware.
type SessionAuth struct {
	gw     *bff.Gateway
	logger *logrus.Entry
	now    func() time.Time
}

// NewSessionAuth creates the session middleware. gw nil (BFF not configured,
// development without Socrate) leaves requests to AuthMiddleware alone.
func NewSessionAuth(gw *bff.Gateway, logger *logrus.Entry) *SessionAuth {
	return &SessionAuth{gw: gw, logger: logger, now: time.Now}
}

// Handler is the chi-compatible middleware function.
func (m *SessionAuth) Handler(next http.Handler) http.Handler {
	if m.gw == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := m.gw.SessionFromRequest(r)
		if !ok {
			if _, had := m.gw.Cookie.SessionID(r); had {
				m.gw.Cookie.ClearSession(w)
			}
			apierror.Unauthorized("authentication required").WriteJSON(w)
			return
		}
		if !m.gw.CheckCSRF(r, s) {
			apierror.Forbidden("missing or invalid CSRF token").WriteJSON(w)
			return
		}
		access, err := m.gw.EnsureFresh(r.Context(), s)
		if err != nil {
			if bff.IsFatalRefreshError(err) {
				m.logger.WithField("sub", s.User().Sub).Info("bff: refresh token rejected by Socrate; session ended")
				m.gw.Store.Delete(s.ID())
				m.gw.Cookie.ClearSession(w)
				apierror.Unauthorized("session expired").WriteJSON(w)
				return
			}
			m.logger.WithError(err).Warn("bff: token refresh failed; session kept")
			apierror.BadGateway("token refresh unavailable").WriteJSON(w)
			return
		}
		if now := m.now(); now.Sub(s.LastSeen()) >= sessionTouchInterval {
			s.Touch(now)
			m.gw.Store.Put(s)
		}
		r.Header.Set("Authorization", "Bearer "+access)
		next.ServeHTTP(w, r)
	})
}

// EndWithoutRefreshToken wraps the gateway's refresher so that a session
// holding no refresh token (Socrate issued none) ends when its access token
// does, instead of sending Socrate an empty refresh token and answering 502
// until the idle timeout.
func EndWithoutRefreshToken(r bff.TokenRefresher) bff.TokenRefresher {
	return refreshOrEnd{r}
}

type refreshOrEnd struct{ bff.TokenRefresher }

func (r refreshOrEnd) RefreshToken(ctx context.Context, refreshToken string) (*socrate.TokenSet, error) {
	if refreshToken == "" {
		return nil, &socrate.OAuthError{Code: "invalid_grant", Description: "session has no refresh token"}
	}
	return r.TokenRefresher.RefreshToken(ctx, refreshToken)
}
