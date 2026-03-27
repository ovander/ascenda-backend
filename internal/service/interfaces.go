package service

import (
	"context"

	"github.com/google/uuid"
	"ascenda/internal/model"
	"ascenda/internal/pkg/pagination"
)

// PlanServicer is the interface that PlanHandler depends on. Depending on this
// interface rather than the concrete *PlanService makes the handler independently
// testable (swap in a mock) and keeps the coupling explicit.
//
// Methods here cover only the operations called by PlanHandler. ScenarioHandler
// already uses its own ScenarioServicer interface — the patterns are consistent.
type PlanServicer interface {
	ListPlans(ctx context.Context, tenantID uuid.UUID, params pagination.Params) ([]model.BusinessPlan, int64, error)
	CreatePlan(ctx context.Context, tenantID, createdBy uuid.UUID, name, description, country string) (*model.BusinessPlan, error)
	GetPlan(ctx context.Context, tenantID, planID uuid.UUID) (*model.BusinessPlan, error)
	UpdatePlan(ctx context.Context, tenantID, planID uuid.UUID, name, description, status string) error
	DeletePlan(ctx context.Context, tenantID, planID uuid.UUID) error
	// Lifecycle state machine
	TransitionPlanStatus(ctx context.Context, tenantID, planID uuid.UUID, newStatus, callerRole string) error
	GetPlanImpact(ctx context.Context, tenantID, planID uuid.UUID) (*PlanImpact, error)
	// Scenario helpers used by plan handler for impact query
	GetScenarioImpact(ctx context.Context, tenantID, planID, scenarioID uuid.UUID) (*ScenarioImpact, error)
	// Used by ScenarioHandler (already satisfies ScenarioServicer)
	ListScenarios(ctx context.Context, tenantID, planID uuid.UUID) ([]model.Scenario, error)
}

// Compile-time check: *PlanService must satisfy PlanServicer.
var _ PlanServicer = (*PlanService)(nil)
