package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"kerplan/internal/pkg/apierror"
	"kerplan/internal/pkg/ctxutil"
)

// SocrateClaims represents JWT claims from Socrate authentication.
type SocrateClaims struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// jwksKey represents a single key from a JWKS endpoint.
type jwksKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwksResponse represents the full JWKS endpoint response.
type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// AuthMiddleware validates JWT tokens using RSA public keys from a JWKS endpoint.
type AuthMiddleware struct {
	jwksURL    string
	issuer     string
	logger     *logrus.Entry
	httpClient *http.Client

	mu         sync.RWMutex
	keys       map[string]*rsa.PublicKey
	lastFetch  time.Time
	cacheTTL   time.Duration
}

// NewAuthMiddleware creates a new AuthMiddleware that validates tokens using JWKS.
func NewAuthMiddleware(jwksURL string, issuer string, logger *logrus.Entry) *AuthMiddleware {
	return &AuthMiddleware{
		jwksURL:    jwksURL,
		issuer:     issuer,
		logger:     logger,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		keys:       make(map[string]*rsa.PublicKey),
		cacheTTL:   1 * time.Hour,
	}
}

// Handler wraps an HTTP handler with JWT validation.
func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := m.extractBearerToken(r)
		if err != nil {
			m.logger.WithError(err).Warn("failed to extract bearer token")
			writeErrorJSON(w, apierror.Unauthorized("missing or invalid authorization header"))
			return
		}

		claims, err := m.validateToken(token)
		if err != nil {
			m.logger.WithError(err).Warn("failed to validate token")
			writeErrorJSON(w, apierror.Unauthorized("invalid or expired token"))
			return
		}

		// Inject claims into context
		ctx := r.Context()

		// Parse tenant_id if present (may not exist in Socrate tokens)
		if claims.TenantID != "" {
			tenantID, err := uuid.Parse(claims.TenantID)
			if err != nil {
				m.logger.WithError(err).Warn("invalid tenant_id in claims")
				writeErrorJSON(w, apierror.Unauthorized("invalid tenant_id"))
				return
			}
			ctx = ctxutil.WithTenantID(ctx, tenantID)
		}

		// Use user_id claim if present, otherwise fall back to JWT subject (sub)
		userIDStr := claims.UserID
		if userIDStr == "" {
			userIDStr = claims.Subject
		}
		if userIDStr != "" {
			// Try to parse as UUID; if it fails, generate a deterministic UUID v5
			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				// Generate deterministic UUID from Socrate subject using UUID v5
				userID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("socrate:"+userIDStr))
			}
			ctx = ctxutil.WithUserID(ctx, userID)
			ctx = ctxutil.WithUserSub(ctx, userIDStr)
		}

		// Store email and name from claims
		if claims.Email != "" {
			ctx = ctxutil.WithUserEmail(ctx, claims.Email)
		}
		if claims.Name != "" {
			ctx = ctxutil.WithUserName(ctx, claims.Name)
		}

		if claims.Role != "" {
			ctx = ctxutil.WithUserRole(ctx, claims.Role)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearerToken extracts the JWT token from Authorization header.
func (m *AuthMiddleware) extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", apierror.Unauthorized("missing authorization header")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", apierror.Unauthorized("invalid authorization header format")
	}

	return parts[1], nil
}

// validateToken validates and parses the JWT token using RS256 public keys from JWKS.
func (m *AuthMiddleware) validateToken(tokenString string) (*SocrateClaims, error) {
	claims := &SocrateClaims{}

	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
	}
	if m.issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(m.issuer))
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Ensure the algorithm is RSA
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from the token header
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("token has no kid header")
		}

		// Look up the key, fetching JWKS if needed
		key, err := m.getKey(kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get signing key: %w", err)
		}

		return key, nil
	}, parserOpts...)

	if err != nil || !token.Valid {
		return nil, apierror.Unauthorized("token validation failed")
	}

	return claims, nil
}

// getKey returns the RSA public key for the given kid, fetching from JWKS if needed.
func (m *AuthMiddleware) getKey(kid string) (*rsa.PublicKey, error) {
	// Try cache first
	m.mu.RLock()
	key, ok := m.keys[kid]
	expired := time.Since(m.lastFetch) > m.cacheTTL
	m.mu.RUnlock()

	if ok && !expired {
		return key, nil
	}

	// Fetch fresh JWKS
	if err := m.fetchJWKS(); err != nil {
		// If fetch fails but we have a cached key, use it
		if ok {
			m.logger.WithError(err).Warn("JWKS refresh failed, using cached key")
			return key, nil
		}
		return nil, err
	}

	// Look up again after fetch
	m.mu.RLock()
	key, ok = m.keys[kid]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}

	return key, nil
}

// fetchJWKS fetches and parses the JWKS endpoint, caching the RSA public keys.
func (m *AuthMiddleware) fetchJWKS() error {
	if m.jwksURL == "" {
		return fmt.Errorf("JWKS URL is not configured")
	}

	resp, err := m.httpClient.Get(m.jwksURL)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS from %s: %w", m.jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}

		pubKey, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			m.logger.WithError(err).WithField("kid", k.Kid).Warn("failed to parse JWKS key")
			continue
		}

		keys[k.Kid] = pubKey
	}

	if len(keys) == 0 {
		return fmt.Errorf("no usable RSA signing keys found in JWKS")
	}

	m.mu.Lock()
	m.keys = keys
	m.lastFetch = time.Now()
	m.mu.Unlock()

	m.logger.WithField("key_count", len(keys)).Debug("JWKS keys refreshed")
	return nil
}

// parseRSAPublicKey builds an *rsa.PublicKey from base64url-encoded modulus and exponent.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// writeErrorJSON is a helper to write error responses.
func writeErrorJSON(w http.ResponseWriter, err *apierror.AppError) {
	err.WriteJSON(w)
}
