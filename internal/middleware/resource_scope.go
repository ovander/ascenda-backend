package middleware

import (
	"net/http"

	"ascenda/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"
)

// Narrow lookup interfaces: the guards only need single-row reads, so the
// concrete repositories satisfy them and tests can use tiny fakes.

// ProductLookup loads a product by (tenant, id).
type ProductLookup interface {
	GetByID(tenantID, productID uuid.UUID) (*model.Product, error)
}

// BEPLookup loads BEP snapshots and optimisation plans by (tenant, id).
type BEPLookup interface {
	GetSnapshot(tenantID, id uuid.UUID) (*model.BEPSnapshot, error)
	GetOptimisationPlan(tenantID, id uuid.UUID) (*model.OptimisationPlan, error)
}

// CapTableLookup loads scenario-level cap-table entities by (tenant, id).
type CapTableLookup interface {
	GetShareholder(tenantID, id uuid.UUID) (*model.CapTableShareholder, error)
	GetRound(tenantID, id uuid.UUID) (*model.CapTableRound, error)
	GetPlan(tenantID, id uuid.UUID) (*model.StockOptionPlan, error)
	GetGrant(tenantID, id uuid.UUID) (*model.OptionGrant, error)
	GetValuationScenario(tenantID, id uuid.UUID) (*model.ValuationScenario, error)
	GetBranch(tenantID, id uuid.UUID) (*model.CapTableScenarioBranch, error)
}

// CapTableEntity names the kind of scenario-level cap-table resource a route
// addresses, so RequireCapTableEntityInScenario knows which lookup to run.
type CapTableEntity string

const (
	CapTableShareholder CapTableEntity = "shareholder"
	CapTableRound       CapTableEntity = "round"
	CapTableOptionPlan  CapTableEntity = "option_plan"
	CapTableOptionGrant CapTableEntity = "option_grant"
	CapTableValuation   CapTableEntity = "valuation"
	CapTableBranch      CapTableEntity = "branch"
)

// ResourceScopeMiddleware binds sub-resource IDs in the URL to their parent
// resource. Every data service is keyed on (tenant, id) for these entities,
// so without these guards a user with access to one plan could address any
// product, BEP snapshot or cap-table row of the tenant by pairing an ID they
// can reach with a foreign sub-resource ID.
//
// Mount the guards below RequireScenarioInPlan; a mismatch or unknown entity
// is a 404 so foreign IDs are not confirmed to exist.
type ResourceScopeMiddleware struct {
	products ProductLookup
	bep      BEPLookup
	capTable CapTableLookup
	logger   *logrus.Entry
}

// NewResourceScopeMiddleware creates a ResourceScopeMiddleware.
func NewResourceScopeMiddleware(products ProductLookup, bep BEPLookup, capTable CapTableLookup, logger *logrus.Entry) *ResourceScopeMiddleware {
	return &ResourceScopeMiddleware{products: products, bep: bep, capTable: capTable, logger: logger}
}

// urlUUID parses a UUID route parameter; ok is false (and a 400 has been
// written) when the value is missing or malformed.
func urlUUID(w http.ResponseWriter, r *http.Request, name, label string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid "+label+" ID")
		return uuid.Nil, false
	}
	return id, true
}

func (m *ResourceScopeMiddleware) deny(w http.ResponseWriter, r *http.Request, kind string, id, parentID, actualParent uuid.UUID) {
	m.logger.WithFields(logrus.Fields{
		"user_id":       ctxutil.GetUserID(r.Context()),
		"kind":          kind,
		"id":            id,
		"url_parent":    parentID,
		"actual_parent": actualParent,
	}).Warn(kind + " does not belong to parent in URL — access denied")
	writeJSONError(w, http.StatusNotFound, "not_found", kind+" not found")
}

// RequireProductInScenario verifies {productId} belongs to {scenarioId}.
func (m *ResourceScopeMiddleware) RequireProductInScenario(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scenarioID, ok := urlUUID(w, r, "scenarioId", "scenario")
		if !ok {
			return
		}
		productID, ok := urlUUID(w, r, "productId", "product")
		if !ok {
			return
		}
		p, err := m.products.GetByID(ctxutil.GetTenantID(r.Context()), productID)
		if err != nil || p == nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "product not found")
			return
		}
		if p.ScenarioID != scenarioID {
			m.deny(w, r, "product", productID, scenarioID, p.ScenarioID)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireBEPSnapshotInScenario verifies the BEP {snapshotId} belongs to {scenarioId}.
func (m *ResourceScopeMiddleware) RequireBEPSnapshotInScenario(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scenarioID, ok := urlUUID(w, r, "scenarioId", "scenario")
		if !ok {
			return
		}
		snapshotID, ok := urlUUID(w, r, "snapshotId", "BEP snapshot")
		if !ok {
			return
		}
		s, err := m.bep.GetSnapshot(ctxutil.GetTenantID(r.Context()), snapshotID)
		if err != nil || s == nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "BEP snapshot not found")
			return
		}
		if s.ScenarioID != scenarioID {
			m.deny(w, r, "BEP snapshot", snapshotID, scenarioID, s.ScenarioID)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireOptimisationPlanInSnapshot verifies the BEP optimisation plan
// {optPlanId} belongs to the BEP {snapshotId}.
func (m *ResourceScopeMiddleware) RequireOptimisationPlanInSnapshot(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snapshotID, ok := urlUUID(w, r, "snapshotId", "BEP snapshot")
		if !ok {
			return
		}
		planID, ok := urlUUID(w, r, "optPlanId", "optimisation plan")
		if !ok {
			return
		}
		p, err := m.bep.GetOptimisationPlan(ctxutil.GetTenantID(r.Context()), planID)
		if err != nil || p == nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "optimisation plan not found")
			return
		}
		if p.SnapshotID != snapshotID {
			m.deny(w, r, "optimisation plan", planID, snapshotID, p.SnapshotID)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireCapTableEntityInScenario verifies that the cap-table entity of the
// given kind, addressed by the route parameter param, belongs to {scenarioId}.
// Option grants carry no scenario ID of their own; they are resolved through
// their stock option plan.
func (m *ResourceScopeMiddleware) RequireCapTableEntityInScenario(kind CapTableEntity, param string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scenarioID, ok := urlUUID(w, r, "scenarioId", "scenario")
			if !ok {
				return
			}
			id, ok := urlUUID(w, r, param, string(kind))
			if !ok {
				return
			}
			tenantID := ctxutil.GetTenantID(r.Context())

			owner, found := m.capTableScenario(tenantID, kind, id)
			if !found {
				writeJSONError(w, http.StatusNotFound, "not_found", string(kind)+" not found")
				return
			}
			if owner != scenarioID {
				m.deny(w, r, string(kind), id, scenarioID, owner)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// capTableScenario returns the scenario that owns the entity, or found=false.
func (m *ResourceScopeMiddleware) capTableScenario(tenantID uuid.UUID, kind CapTableEntity, id uuid.UUID) (uuid.UUID, bool) {
	switch kind {
	case CapTableShareholder:
		if e, err := m.capTable.GetShareholder(tenantID, id); err == nil && e != nil {
			return e.ScenarioID, true
		}
	case CapTableRound:
		if e, err := m.capTable.GetRound(tenantID, id); err == nil && e != nil {
			return e.ScenarioID, true
		}
	case CapTableOptionPlan:
		if e, err := m.capTable.GetPlan(tenantID, id); err == nil && e != nil {
			return e.ScenarioID, true
		}
	case CapTableOptionGrant:
		if g, err := m.capTable.GetGrant(tenantID, id); err == nil && g != nil {
			if p, err := m.capTable.GetPlan(tenantID, g.PlanID); err == nil && p != nil {
				return p.ScenarioID, true
			}
		}
	case CapTableValuation:
		if e, err := m.capTable.GetValuationScenario(tenantID, id); err == nil && e != nil {
			return e.ScenarioID, true
		}
	case CapTableBranch:
		if e, err := m.capTable.GetBranch(tenantID, id); err == nil && e != nil {
			return e.ScenarioID, true
		}
	}
	return uuid.Nil, false
}
