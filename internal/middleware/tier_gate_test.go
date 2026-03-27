package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
	"ascenda/internal/pkg/ctxutil"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock TenantRepository
// ─────────────────────────────────────────────────────────────────────────────

type mockTierTenantRepo struct {
	tenants     map[uuid.UUID]*model.Tenant
	err         error
	lookupCount int // counts GetByID calls — used to verify cache-reuse optimisation
}

func newMockTierTenantRepo() *mockTierTenantRepo {
	return &mockTierTenantRepo{tenants: make(map[uuid.UUID]*model.Tenant)}
}

func (m *mockTierTenantRepo) addTenant(id uuid.UUID, tier string) {
	m.tenants[id] = &model.Tenant{Tier: tier}
	m.tenants[id].ID = id
}

func (m *mockTierTenantRepo) Create(tenant *model.Tenant) error { return nil }

func (m *mockTierTenantRepo) GetByID(id uuid.UUID) (*model.Tenant, error) {
	m.lookupCount++
	if m.err != nil {
		return nil, m.err
	}
	t, ok := m.tenants[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (m *mockTierTenantRepo) GetBySlug(slug string) (*model.Tenant, error) { return nil, nil }
func (m *mockTierTenantRepo) ListActive(offset, limit int) ([]*model.Tenant, error) {
	return nil, nil
}
func (m *mockTierTenantRepo) Update(tenant *model.Tenant) error { return nil }
func (m *mockTierTenantRepo) Delete(id uuid.UUID) error         { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// Helper: request with tenant ID in context
// ─────────────────────────────────────────────────────────────────────────────

func newTierGateRequest(tenantID uuid.UUID) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/captable/company", nil)
	ctx := ctxutil.WithTenantID(req.Context(), tenantID)
	return req.WithContext(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierAtLeast — pure logic tests
// ─────────────────────────────────────────────────────────────────────────────

func TestTierAtLeast_SameTier(t *testing.T) {
	assert.True(t, TierAtLeast(TierFree, TierFree))
	assert.True(t, TierAtLeast(TierPro, TierPro))
	assert.True(t, TierAtLeast(TierEnterprise, TierEnterprise))
}

func TestTierAtLeast_HigherTierSatisfiesLower(t *testing.T) {
	assert.True(t, TierAtLeast(TierPro, TierFree))
	assert.True(t, TierAtLeast(TierEnterprise, TierFree))
	assert.True(t, TierAtLeast(TierEnterprise, TierPro))
}

func TestTierAtLeast_LowerTierFailsHigher(t *testing.T) {
	assert.False(t, TierAtLeast(TierFree, TierPro))
	assert.False(t, TierAtLeast(TierFree, TierEnterprise))
	assert.False(t, TierAtLeast(TierPro, TierEnterprise))
}

func TestTierAtLeast_UnknownTierReturnsFalse(t *testing.T) {
	assert.False(t, TierAtLeast("unknown", TierFree))
	assert.False(t, TierAtLeast(TierEnterprise, "unknown"))
	assert.False(t, TierAtLeast("", ""))
}

// ─────────────────────────────────────────────────────────────────────────────
// AllTiers — ordering guarantee
// ─────────────────────────────────────────────────────────────────────────────

func TestAllTiers_ReturnsThreeTiers(t *testing.T) {
	tiers := AllTiers()
	assert.Len(t, tiers, 3)
	assert.Equal(t, TierFree, tiers[0])
	assert.Equal(t, TierPro, tiers[1])
	assert.Equal(t, TierEnterprise, tiers[2])
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — enterprise tenant passes
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_EnterpriseTenant_PassesEnterprise(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierEnterprise)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.True(t, nextCalled, "handler should be called for enterprise tenant")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTierGate_EnterpriseTenant_PassesPro(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierEnterprise)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.True(t, nextCalled, "enterprise tenant should pass a 'pro' gate")
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — free tenant blocked at pro/enterprise gates
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_FreeTenant_BlockedAtPro(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierFree)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for free tenant at pro gate")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_FreeTenant_BlockedAtEnterprise(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierFree)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for free tenant at enterprise gate")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_ProTenant_BlockedAtEnterprise(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierPro)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for pro tenant at enterprise gate")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — 403 JSON body contains correct error code
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_ForbiddenBody_ContainsUpgradeRequired(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierFree)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	require.Equal(t, http.StatusForbidden, w.Code)

	var body tierGateErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "response body should be valid JSON")

	assert.Equal(t, "upgrade_required", body.Error)
	assert.Equal(t, TierFree, body.Tier)
	assert.Equal(t, TierEnterprise, body.RequiredTier)
	assert.NotEmpty(t, body.UpgradeURL)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — tenant not found returns 403
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_TenantNotFound_Returns403(t *testing.T) {
	repo := newMockTierTenantRepo()
	// No tenant added — lookup will fail

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler when tenant not found")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(uuid.New()))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_RepoError_Returns403(t *testing.T) {
	repo := newMockTierTenantRepo()
	repo.err = errors.New("database connection lost")

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler when repo errors")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(uuid.New()))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — response headers
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_ContentTypeJSON_OnForbidden(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierFree)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — free tier gate passes everyone
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_FreeGate_AllTenantsTierPass(t *testing.T) {
	tiers := []string{TierFree, TierPro, TierEnterprise}
	for _, tier := range tiers {
		t.Run(tier, func(t *testing.T) {
			repo := newMockTierTenantRepo()
			tenantID := uuid.New()
			repo.addTenant(tenantID, tier)

			mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))
			nextCalled := false
			handler := mw.Require(TierFree)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, newTierGateRequest(tenantID))

			assert.True(t, nextCalled, "all tiers should pass a free gate (%s)", tier)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — tier is stored in context on successful pass
// These tests verify the fix for the 402 regression where AIAccessMiddleware
// always read model.AITierStandard instead of the actual tenant tier.
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_ProTenant_StoresTierInContext(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierPro)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	var gotTier string
	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTier = ctxutil.GetTenantTier(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierPro, gotTier,
		"downstream handler must receive the verified tenant tier in context")
}

func TestTierGate_EnterpriseTenant_StoresTierInContext(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierEnterprise)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	var gotTier string
	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTier = ctxutil.GetTenantTier(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierEnterprise, gotTier)
}

func TestTierGate_FreeTenant_StoresTierInContext(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierFree)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	var gotTier string
	handler := mw.Require(TierFree)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTier = ctxutil.GetTenantTier(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierFree, gotTier)
}

func TestTierGate_BlockedTenant_DoesNotStoreTierInContext(t *testing.T) {
	// When the gate blocks, no context modification should happen.
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierFree)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called for blocked tenant")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusForbidden, w.Code)
	// No handler ran, so no tier could have leaked into context — the test
	// passing without a Fatal call is the assertion.
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — Resolve() enriches context without blocking
// ─────────────────────────────────────────────────────────────────────────────

func TestTierResolve_ProTenant_StoresTierInContext(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierPro)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	var gotTier string
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTier = ctxutil.GetTenantTier(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusOK, w.Code, "Resolve must never block")
	assert.Equal(t, TierPro, gotTier)
}

func TestTierResolve_FreeTenant_StoresTierInContext(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierFree)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	var gotTier string
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTier = ctxutil.GetTenantTier(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierFree, gotTier)
}

func TestTierResolve_EnterpriseTenant_StoresTierInContext(t *testing.T) {
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierEnterprise)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	var gotTier string
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTier = ctxutil.GetTenantTier(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierEnterprise, gotTier)
}

func TestTierResolve_TenantNotFound_NeverBlocks(t *testing.T) {
	// Resolve must forward even when the tenant lookup fails.
	// This keeps standard-tier endpoints available in degraded states.
	repo := newMockTierTenantRepo()
	// No tenant added — lookup will return an error.

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(uuid.New()))

	assert.True(t, nextCalled, "Resolve must always call next, even on lookup failure")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTierResolve_RepoError_NeverBlocks(t *testing.T) {
	repo := newMockTierTenantRepo()
	repo.err = errors.New("db timeout")

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	nextCalled := false
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(uuid.New()))

	assert.True(t, nextCalled, "Resolve must always call next, even on repo error")
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Resolve() + Require() chained — single DB lookup optimisation
// ─────────────────────────────────────────────────────────────────────────────

func TestTierResolve_ThenRequire_PassesWithSingleDBLookup(t *testing.T) {
	// When Resolve() runs before Require(), Require() must reuse the tier
	// already stored in context rather than making a second DB call.
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierPro)

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	lookupsBefore := 0
	lookupsAfterResolve := 0

	var gotTier string
	// Chain: Resolve → Require(Pro) → handler
	handler := mw.Resolve()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Snapshot lookup count after Resolve ran.
			lookupsAfterResolve = repo.lookupCount
			// Now apply Require inside the handler chain.
			mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotTier = ctxutil.GetTenantTier(r.Context())
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(w, r)
		}),
	)

	_ = lookupsBefore
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierPro, gotTier)
	// Resolve made 1 lookup; Require must not add another.
	assert.Equal(t, lookupsAfterResolve, repo.lookupCount,
		"Require must skip DB lookup when tier is already in context from Resolve")
}

func TestTierResolve_ThenRequire_HigherTierBlockedCorrectly(t *testing.T) {
	// Even with Resolve enriching context first, Require must still gate correctly.
	repo := newMockTierTenantRepo()
	tenantID := uuid.New()
	repo.addTenant(tenantID, TierPro) // Pro tenant

	mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

	// Chain: Resolve → Require(Enterprise) — should block Pro tenant.
	handler := mw.Resolve()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("handler must not be called: Pro tenant blocked at Enterprise gate")
			})).ServeHTTP(w, r)
		}),
	)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(tenantID))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_AllTiers_ContextTierMatchesTenantTier(t *testing.T) {
	tiers := []string{TierFree, TierPro, TierEnterprise}
	for _, tier := range tiers {
		t.Run(tier, func(t *testing.T) {
			repo := newMockTierTenantRepo()
			tenantID := uuid.New()
			repo.addTenant(tenantID, tier)

			mw := NewTierGateMiddleware(repo, logrus.NewEntry(logrus.New()))

			var gotTier string
			handler := mw.Require(TierFree)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotTier = ctxutil.GetTenantTier(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, newTierGateRequest(tenantID))

			require.Equal(t, http.StatusOK, w.Code, "all tiers pass a free gate")
			assert.Equal(t, tier, gotTier,
				"context tier must equal tenant.Tier for tier=%s", tier)
		})
	}
}
