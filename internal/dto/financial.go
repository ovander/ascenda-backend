package dto

import (
	"kerplan/internal/model"

	"github.com/shopspring/decimal"
)

// CapexEntryResponse is the API representation of a capex entry.
type CapexEntryResponse struct {
	ID                string          `json:"id"`
	ScenarioID        string          `json:"scenarioId"`
	Category          string          `json:"category"`
	YearIndex         int             `json:"yearIndex"`
	Amount            decimal.Decimal `json:"amount"`
	DepreciationYears int             `json:"depreciationYears"`
	IsManualOverride  bool            `json:"isManualOverride"`
	Timestamps
}

// CapexEntryFromModel converts a model.CapexEntry to a CapexEntryResponse.
func CapexEntryFromModel(e model.CapexEntry) CapexEntryResponse {
	return CapexEntryResponse{
		ID:                e.ID.String(),
		ScenarioID:        e.ScenarioID.String(),
		Category:          string(e.Category),
		YearIndex:         e.YearIndex,
		Amount:            e.Amount,
		DepreciationYears: e.DepreciationYears,
		IsManualOverride:  e.IsManualOverride,
		Timestamps:        Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// CapexEntriesFromModels converts a slice of model.CapexEntry to DTOs.
func CapexEntriesFromModels(entries []model.CapexEntry) []CapexEntryResponse {
	out := make([]CapexEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = CapexEntryFromModel(e)
	}
	return out
}

// OpexEntryResponse is the API representation of an opex entry.
type OpexEntryResponse struct {
	ID         string          `json:"id"`
	ScenarioID string          `json:"scenarioId"`
	LineID     string          `json:"lineId"`
	YearIndex  int             `json:"yearIndex"`
	Amount     decimal.Decimal `json:"amount"`
	Timestamps
}

// OpexEntryFromModel converts a model.OpexManualEntry to an OpexEntryResponse.
func OpexEntryFromModel(e model.OpexManualEntry) OpexEntryResponse {
	return OpexEntryResponse{
		ID:         e.ID.String(),
		ScenarioID: e.ScenarioID.String(),
		LineID:     string(e.LineID),
		YearIndex:  e.YearIndex,
		Amount:     e.Amount,
		Timestamps: Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// OpexEntriesFromModels converts a slice of model.OpexManualEntry to DTOs.
func OpexEntriesFromModels(entries []model.OpexManualEntry) []OpexEntryResponse {
	out := make([]OpexEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = OpexEntryFromModel(e)
	}
	return out
}

// PnlEntryResponse is the API representation of a P&L manual entry.
type PnlEntryResponse struct {
	ID         string          `json:"id"`
	ScenarioID string          `json:"scenarioId"`
	LineID     string          `json:"lineId"`
	YearIndex  int             `json:"yearIndex"`
	Amount     decimal.Decimal `json:"amount"`
	Timestamps
}

// PnlEntryFromModel converts a model.PnlManualEntry to a PnlEntryResponse.
func PnlEntryFromModel(e model.PnlManualEntry) PnlEntryResponse {
	return PnlEntryResponse{
		ID:         e.ID.String(),
		ScenarioID: e.ScenarioID.String(),
		LineID:     string(e.LineID),
		YearIndex:  e.YearIndex,
		Amount:     e.Amount,
		Timestamps: Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// PnlEntriesFromModels converts a slice of model.PnlManualEntry to DTOs.
func PnlEntriesFromModels(entries []model.PnlManualEntry) []PnlEntryResponse {
	out := make([]PnlEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = PnlEntryFromModel(e)
	}
	return out
}

// FiplanEntryResponse is the API representation of a financial plan entry.
type FiplanEntryResponse struct {
	ID         string          `json:"id"`
	ScenarioID string          `json:"scenarioId"`
	LineID     string          `json:"lineId"`
	YearIndex  int             `json:"yearIndex"`
	Amount     decimal.Decimal `json:"amount"`
	Timestamps
}

// FiplanEntryFromModel converts a model.FiplanEntry to a FiplanEntryResponse.
func FiplanEntryFromModel(e model.FiplanEntry) FiplanEntryResponse {
	return FiplanEntryResponse{
		ID:         e.ID.String(),
		ScenarioID: e.ScenarioID.String(),
		LineID:     string(e.LineID),
		YearIndex:  e.YearIndex,
		Amount:     e.Amount,
		Timestamps: Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// FiplanEntriesFromModels converts a slice of model.FiplanEntry to DTOs.
func FiplanEntriesFromModels(entries []model.FiplanEntry) []FiplanEntryResponse {
	out := make([]FiplanEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = FiplanEntryFromModel(e)
	}
	return out
}

// PnlCashEntryResponse is the API representation of a P&L + Cash entry.
type PnlCashEntryResponse struct {
	ID         string          `json:"id"`
	ScenarioID string          `json:"scenarioId"`
	LineID     string          `json:"lineId"`
	YearIndex  int             `json:"yearIndex"`
	Amount     decimal.Decimal `json:"amount"`
	Timestamps
}

// PnlCashEntryFromModel converts a model.PnlCashEntry to a PnlCashEntryResponse.
func PnlCashEntryFromModel(e model.PnlCashEntry) PnlCashEntryResponse {
	return PnlCashEntryResponse{
		ID:         e.ID.String(),
		ScenarioID: e.ScenarioID.String(),
		LineID:     string(e.LineID),
		YearIndex:  e.YearIndex,
		Amount:     e.Amount,
		Timestamps: Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// PnlCashEntriesFromModels converts a slice of model.PnlCashEntry to DTOs.
func PnlCashEntriesFromModels(entries []model.PnlCashEntry) []PnlCashEntryResponse {
	out := make([]PnlCashEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = PnlCashEntryFromModel(e)
	}
	return out
}

// WCREntryResponse is the API representation of a WCR entry.
type WCREntryResponse struct {
	ID         string          `json:"id"`
	ScenarioID string          `json:"scenarioId"`
	LineID     string          `json:"lineId"`
	YearIndex  int             `json:"yearIndex"`
	Amount     decimal.Decimal `json:"amount"`
	Timestamps
}

// WCREntryFromModel converts a model.WCREntry to a WCREntryResponse.
func WCREntryFromModel(e model.WCREntry) WCREntryResponse {
	return WCREntryResponse{
		ID:         e.ID.String(),
		ScenarioID: e.ScenarioID.String(),
		LineID:     string(e.LineID),
		YearIndex:  e.YearIndex,
		Amount:     e.Amount,
		Timestamps: Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// WCREntriesFromModels converts a slice of model.WCREntry to DTOs.
func WCREntriesFromModels(entries []model.WCREntry) []WCREntryResponse {
	out := make([]WCREntryResponse, len(entries))
	for i, e := range entries {
		out[i] = WCREntryFromModel(e)
	}
	return out
}

// CashOverrideResponse is the API representation of a cash monthly override.
type CashOverrideResponse struct {
	ID         string          `json:"id"`
	ScenarioID string          `json:"scenarioId"`
	LineID     string          `json:"lineId"`
	YearIndex  int             `json:"yearIndex"`
	Month      int             `json:"month"`
	SubIndex   int             `json:"subIndex"`
	Amount     decimal.Decimal `json:"amount"`
	Timestamps
}

// CashOverrideFromModel converts a model.CashMonthlyOverride to a CashOverrideResponse.
func CashOverrideFromModel(e model.CashMonthlyOverride) CashOverrideResponse {
	return CashOverrideResponse{
		ID:         e.ID.String(),
		ScenarioID: e.ScenarioID.String(),
		LineID:     string(e.LineID),
		YearIndex:  e.YearIndex,
		Month:      e.Month,
		SubIndex:   e.SubIndex,
		Amount:     e.Amount,
		Timestamps: Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// CashOverridesFromModels converts a slice of model.CashMonthlyOverride to DTOs.
func CashOverridesFromModels(overrides []model.CashMonthlyOverride) []CashOverrideResponse {
	out := make([]CashOverrideResponse, len(overrides))
	for i, e := range overrides {
		out[i] = CashOverrideFromModel(e)
	}
	return out
}

// BudgetOverrideResponse is the API representation of a budget monthly override.
type BudgetOverrideResponse struct {
	ID         string          `json:"id"`
	ScenarioID string          `json:"scenarioId"`
	LineID     string          `json:"lineId"`
	YearIndex  int             `json:"yearIndex"`
	Month      int             `json:"month"`
	Amount     decimal.Decimal `json:"amount"`
	Timestamps
}

// BudgetOverrideFromModel converts a model.BudgetMonthlyOverride to a BudgetOverrideResponse.
func BudgetOverrideFromModel(e model.BudgetMonthlyOverride) BudgetOverrideResponse {
	return BudgetOverrideResponse{
		ID:         e.ID.String(),
		ScenarioID: e.ScenarioID.String(),
		LineID:     string(e.LineID),
		YearIndex:  e.YearIndex,
		Month:      e.Month,
		Amount:     e.Amount,
		Timestamps: Timestamps{CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt},
	}
}

// BudgetOverridesFromModels converts a slice of model.BudgetMonthlyOverride to DTOs.
func BudgetOverridesFromModels(overrides []model.BudgetMonthlyOverride) []BudgetOverrideResponse {
	out := make([]BudgetOverrideResponse, len(overrides))
	for i, e := range overrides {
		out[i] = BudgetOverrideFromModel(e)
	}
	return out
}
