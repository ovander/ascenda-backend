package middleware

import (
	"net/http"
)

// BodyLimitMiddleware caps the size of incoming request bodies to protect
// against memory-exhaustion (DoS) attacks. Requests that exceed the limit
// receive a 413 Content Too Large response before the body is read.
//
// Usage — global default applied in router.go:
//
//	r.Use(middleware.BodyLimitMiddleware(cfg.MaxRequestBodyBytes))
//
// Per-route override for endpoints that legitimately accept large payloads:
//
//	r.With(middleware.BodyLimitMiddleware(50 << 20)).Post("/upload", handler)
//
// The limit is enforced using http.MaxBytesReader, which sets an internal cap
// on the response body reader; the framework returns a 413 when the limit is
// exceeded and the handler tries to read past it.
func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
