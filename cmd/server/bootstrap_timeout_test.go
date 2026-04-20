package main

import (
	"testing"
	"time"
)

// TestWriteTimeoutExceedsAIRouteTimeout guards against the ERR_EMPTY_RESPONSE
// regression caused by WriteTimeout ≤ aiRouteTimeout.
//
// Root cause (fixed): the Go http.Server.WriteTimeout was 30 s while the
// application-level aiTimeout context was 120 s.  When an AI inference call
// took longer than 30 s, the server closed the TCP connection before any bytes
// were written — the client received ERR_EMPTY_RESPONSE and no HTTP status code
// was logged, because the Go runtime closed the connection at the transport
// layer before the logger middleware's deferred write could record a response.
//
// The invariant: httpWriteTimeout > aiRouteTimeout.
// If this test fails, update httpWriteTimeout in bootstrap.go to be at least
// (aiRouteTimeout + 30 s).
func TestWriteTimeoutExceedsAIRouteTimeout(t *testing.T) {
	if httpWriteTimeout <= aiRouteTimeout {
		t.Errorf(
			"httpWriteTimeout (%s) must be strictly greater than aiRouteTimeout (%s): "+
				"if WriteTimeout ≤ aiRouteTimeout the Go HTTP server closes the TCP connection "+
				"before the handler can write any response bytes, causing ERR_EMPTY_RESPONSE "+
				"on the client with no logged HTTP status code",
			httpWriteTimeout, aiRouteTimeout,
		)
	}
}

// TestWriteTimeoutMinimumBuffer verifies there is at least a 20-second safety
// margin between aiRouteTimeout and httpWriteTimeout.  A very small gap risks
// a race where the server transport timeout fires fractionally before the
// context-based timeout can flush the response to the client.
func TestWriteTimeoutMinimumBuffer(t *testing.T) {
	const wantMinBuffer = 20 * time.Second
	buffer := httpWriteTimeout - aiRouteTimeout
	if buffer < wantMinBuffer {
		t.Errorf(
			"safety buffer between aiRouteTimeout (%s) and httpWriteTimeout (%s) is %s, "+
				"want ≥ %s to avoid a race between the transport timeout and the context cancel",
			aiRouteTimeout, httpWriteTimeout, buffer, wantMinBuffer,
		)
	}
}
