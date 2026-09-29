package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/socrate"
)

// SocrateClientAttribution puts the browser's address and User-Agent on the
// request context, so the socrate.Client calls made on the user's behalf (code
// exchange, refresh, revocation, magic-link redemption) send them to Socrate as
// X-Forwarded-For and User-Agent. Socrate v1.5.0+ then audits, rate-limits and
// blocks by the browser instead of by this server's address. Older Socrate
// versions ignore the headers.
//
// The address comes from ClientIP with the configured trusted proxies, never
// from a raw request header: Socrate trusts what this server sends, so a
// browser-chosen value would let the browser pick the address it is audited as.
func SocrateClientAttribution(trusted TrustedProxies) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := socrate.WithClientAttribution(r.Context(), socrate.ClientAttribution{
				IP:        ClientIP(r, trusted),
				UserAgent: r.UserAgent(),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
