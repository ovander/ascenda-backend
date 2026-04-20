package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ovander/backendkit/ctxutil"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helper: request with plan in context
// ─────────────────────────────────────────────────────────────────────────────

func newTierGateRequest(plan string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/captable/company", nil)
	ctx := ctxutil.WithUserPlan(req.Context(), plan)
	return req.WithContext(ctx)
}

func newLogger() *logrus.Entry {
	return logrus.NewEntry(logrus.New())
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
// TierGateMiddleware — enterprise plan passes
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_EnterprisePlan_PassesEnterprise(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	nextCalled := false
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierEnterprise))

	assert.True(t, nextCalled, "handler should be called for enterprise plan")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTierGate_EnterprisePlan_PassesPro(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	nextCalled := false
	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierEnterprise))

	assert.True(t, nextCalled, "enterprise plan should pass a 'pro' gate")
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — freemium plan blocked at pro/enterprise gates
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_FreemiumPlan_BlockedAtPro(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for freemium plan at pro gate")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierFree))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_FreemiumPlan_BlockedAtEnterprise(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for freemium plan at enterprise gate")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierFree))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_ProPlan_BlockedAtEnterprise(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for pro plan at enterprise gate")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierPro))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — 403 JSON body contains correct error code
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_ForbiddenBody_ContainsUpgradeRequired(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierFree))

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
// TierGateMiddleware — missing plan defaults to freemium
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_NoPlanInContext_DefaultsToFreemium_BlockedAtPro(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())
	// No plan set in context — should default to freemium.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler with default freemium plan at pro gate")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_NoPlanInContext_DefaultsToFreemium_PassesFreemiumGate(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	nextCalled := false
	handler := mw.Require(TierFree)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — response headers
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_ContentTypeJSON_OnForbidden(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())
	handler := mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierFree))

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — free gate passes everyone
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_FreeGate_AllPlansPass(t *testing.T) {
	plans := []string{TierFree, TierPro, TierEnterprise}
	for _, plan := range plans {
		t.Run(plan, func(t *testing.T) {
			mw := NewTierGateMiddleware(newLogger())
			nextCalled := false
			handler := mw.Require(TierFree)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, newTierGateRequest(plan))

			assert.True(t, nextCalled, "all plans should pass a free gate (%s)", plan)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — plan is in context on successful pass
// ─────────────────────────────────────────────────────────────────────────────

func TestTierGate_ProPlan_StoresPlanInContext(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	var gotPlan string
	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPlan = ctxutil.GetUserPlan(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierPro))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierPro, gotPlan,
		"downstream handler must receive the user plan in context")
}

func TestTierGate_EnterprisePlan_StoresPlanInContext(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	var gotPlan string
	handler := mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPlan = ctxutil.GetUserPlan(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierEnterprise))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierEnterprise, gotPlan)
}

func TestTierGate_FreemiumPlan_StoresPlanInContext(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	var gotPlan string
	handler := mw.Require(TierFree)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPlan = ctxutil.GetUserPlan(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierFree))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierFree, gotPlan)
}

// ─────────────────────────────────────────────────────────────────────────────
// TierGateMiddleware — Resolve() enriches context without blocking
// ─────────────────────────────────────────────────────────────────────────────

func TestTierResolve_ProPlan_StoresPlanInContext(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	var gotPlan string
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPlan = ctxutil.GetUserPlan(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierPro))

	assert.Equal(t, http.StatusOK, w.Code, "Resolve must never block")
	assert.Equal(t, TierPro, gotPlan)
}

func TestTierResolve_FreemiumPlan_StoresPlanInContext(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	var gotPlan string
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPlan = ctxutil.GetUserPlan(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierFree))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierFree, gotPlan)
}

func TestTierResolve_EnterprisePlan_StoresPlanInContext(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	var gotPlan string
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPlan = ctxutil.GetUserPlan(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierEnterprise))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierEnterprise, gotPlan)
}

func TestTierResolve_NeverBlocks(t *testing.T) {
	// Resolve must forward even when no plan is set (defaults to freemium).
	mw := NewTierGateMiddleware(newLogger())

	nextCalled := false
	handler := mw.Resolve()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	// No plan in context.
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.True(t, nextCalled, "Resolve must always call next")
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Resolve() + Require() chained
// ─────────────────────────────────────────────────────────────────────────────

func TestTierResolve_ThenRequire_PassesProPlan(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	var gotPlan string
	// Chain: Resolve → Require(Pro) → handler
	handler := mw.Resolve()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mw.Require(TierPro)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPlan = ctxutil.GetUserPlan(r.Context())
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(w, r)
		}),
	)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierPro))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierPro, gotPlan)
}

func TestTierResolve_ThenRequire_HigherPlanBlockedCorrectly(t *testing.T) {
	mw := NewTierGateMiddleware(newLogger())

	// Chain: Resolve → Require(Enterprise) — should block Pro plan.
	handler := mw.Resolve()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mw.Require(TierEnterprise)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("handler must not be called: Pro plan blocked at Enterprise gate")
			})).ServeHTTP(w, r)
		}),
	)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newTierGateRequest(TierPro))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTierGate_AllPlans_ContextPlanMatchesInput(t *testing.T) {
	plans := []string{TierFree, TierPro, TierEnterprise}
	for _, plan := range plans {
		t.Run(plan, func(t *testing.T) {
			mw := NewTierGateMiddleware(newLogger())

			var gotPlan string
			handler := mw.Require(TierFree)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPlan = ctxutil.GetUserPlan(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, newTierGateRequest(plan))

			require.Equal(t, http.StatusOK, w.Code, "all plans pass a free gate")
			assert.Equal(t, plan, gotPlan,
				"context plan must equal input plan for plan=%s", plan)
		})
	}
}
