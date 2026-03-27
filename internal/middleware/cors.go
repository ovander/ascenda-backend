package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORSMiddleware configures CORS for the API using origins supplied by the caller.
// Origins are read from cfg.AllowedOrigins (env: CORS_ORIGINS) in Bootstrap so
// that production deployments never rely on hardcoded localhost values.
//
// Note: AllowCredentials=true is only safe when AllowedOrigins is an explicit
// list (no wildcard "*"). Wildcard patterns like "https://*.example.com" are
// accepted by go-chi/cors but operators should prefer explicit origins in prod.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
