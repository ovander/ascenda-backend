package service

import (
	"context"

	"ascenda/internal/model"
	"github.com/google/uuid"
)

// BSheetServicer is the interface used by BSheetHandler.
// Defined here so the handler can accept either the real *BSheetService or a test mock.
type BSheetServicer interface {
	GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.BSheetReport, error)
	GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error)
}

// CapexServicer is the interface used by CapexHandler.
// Defined here so the handler can accept either the real *CapexService or a test mock.
type CapexServicer interface {
	ListEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapexEntry, error)
	UpdateEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.CapexEntry) error
	GetSummary(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapexSummary, error)
}

// Compile-time checks that the real service types satisfy these interfaces.
var _ BSheetServicer = (*BSheetService)(nil)
var _ CapexServicer = (*CapexService)(nil)
