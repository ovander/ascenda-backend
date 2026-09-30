package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/ovander/backendkit/bff"
)

// SocrateClientAttribution puts the browser's address and User-Agent on the
// request context (bff.WithClientAttribution), so the socrate.Client calls made
// on the user's behalf (code exchange, refresh, revocation, magic-link
// redemption) send them to Socrate: backendkit's ApplyClientAttribution sets
// X-Forwarded-For to exactly that one address, replacing any value, and removes
// X-Real-IP. Socrate's Caddy trusts X-Forwarded-For from the apps VPS only, so
// Socrate audits, rate-limits and blocks by the browser.
//
// The address is AttributionIP's, never a header copied from the browser.
func SocrateClientAttribution() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, bff.WithClientAttribution(r, AttributionIP(r)))
		})
	}
}

// AttributionIP resolves the browser's address to send to Socrate. It trusts
// X-Forwarded-For only when the TCP peer is loopback, i.e. Caddy on this host,
// and then takes the rightmost entry that is not loopback (the one Caddy
// appended; entries to its left came from the browser), or nothing when that
// entry does not parse. From any other peer it returns the peer's own address
// and ignores the headers. X-Real-IP is never read. A loopback peer without a
// forwarded address yields "", so nothing is attributed: the request came from
// this host, not from a browser.
//
// It is deliberately stricter than ClientIP, which the rate limiters use: that
// one honours TRUSTED_PROXY_CIDRS and X-Real-IP, while Socrate records
// whatever this server sends.
func AttributionIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer == nil {
		return ""
	}
	if !peer.IsLoopback() {
		return peer.String()
	}
	// Several X-Forwarded-For headers read as one list, rightmost last. Walk
	// from the right past loopback hops; the first other entry is the one Caddy
	// appended. If it does not parse, stop: never fall back to an entry further
	// left, which the browser wrote.
	entries := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(entries) - 1; i >= 0; i-- {
		entry := strings.TrimSpace(entries[i])
		if entry == "" {
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return ""
		}
		if !ip.IsLoopback() {
			return ip.String()
		}
	}
	return ""
}
