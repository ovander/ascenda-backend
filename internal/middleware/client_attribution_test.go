package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAttributionIP(t *testing.T) {
	cases := []struct {
		name string
		peer string
		xff  []string // one X-Forwarded-For header per element
		xri  string
		want string
	}{
		{"direct peer, headers ignored", "198.51.100.9:5555", []string{"6.6.6.6"}, "6.6.6.6", "198.51.100.9"},
		{"direct IPv6 peer", "[2001:db8::1]:5555", nil, "", "2001:db8::1"},
		{"loopback peer: forwarded client", "127.0.0.1:1", []string{"203.0.113.7"}, "", "203.0.113.7"},
		{"loopback peer: rightmost entry wins", "127.0.0.1:1", []string{"6.6.6.6, 203.0.113.7"}, "", "203.0.113.7"},
		{"loopback peer: several headers read as one list", "127.0.0.1:1", []string{"6.6.6.6", "203.0.113.7"}, "", "203.0.113.7"},
		{"loopback peer: loopback hops skipped", "127.0.0.1:1", []string{"203.0.113.7, 127.0.0.1"}, "", "203.0.113.7"},
		{"loopback peer: unparsable rightmost stops the walk", "127.0.0.1:1", []string{"6.6.6.6, not-an-ip"}, "", ""},
		{"loopback peer: no X-Forwarded-For, X-Real-IP ignored", "127.0.0.1:1", nil, "6.6.6.6", ""},
		{"IPv6 loopback peer", "[::1]:1", []string{"2001:db8::7"}, "", "2001:db8::7"},
		{"unparsable peer", "garbage", []string{"203.0.113.7"}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/auth/refresh", nil)
			r.RemoteAddr = tc.peer
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if tc.xri != "" {
				r.Header.Set("X-Real-IP", tc.xri)
			}
			assert.Equal(t, tc.want, AttributionIP(r))
		})
	}
}
