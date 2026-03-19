package dto

import (
	"kerplan/internal/model"

	"github.com/shopspring/decimal"
)

// HeadcountResponse is the API representation of a staff headcount record.
type HeadcountResponse struct {
	ID           string          `json:"id"`
	ScenarioID   string          `json:"scenarioId"`
	Category     string          `json:"category"`
	YearIndex    int             `json:"yearIndex"`
	FTE          decimal.Decimal `json:"fte"`
	IsOverridden bool            `json:"isOverridden"`
	Timestamps
}

// HeadcountFromModel converts a model.StaffHeadcount to a HeadcountResponse.
func HeadcountFromModel(h model.StaffHeadcount) HeadcountResponse {
	return HeadcountResponse{
		ID:           h.ID.String(),
		ScenarioID:   h.ScenarioID.String(),
		Category:     string(h.Category),
		YearIndex:    h.YearIndex,
		FTE:          h.FTE,
		IsOverridden: h.IsOverridden,
		Timestamps:   Timestamps{CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt},
	}
}

// HeadcountsFromModels converts a slice of model.StaffHeadcount to DTOs.
func HeadcountsFromModels(hcs []model.StaffHeadcount) []HeadcountResponse {
	out := make([]HeadcountResponse, len(hcs))
	for i, h := range hcs {
		out[i] = HeadcountFromModel(h)
	}
	return out
}

// SalaryResponse is the API representation of a staff salary record.
type SalaryResponse struct {
	ID                 string          `json:"id"`
	ScenarioID         string          `json:"scenarioId"`
	Category           string          `json:"category"`
	YearIndex          int             `json:"yearIndex"`
	MonthlyGrossSalary decimal.Decimal `json:"monthlyGrossSalary"`
	AnnualIncreasePct  decimal.Decimal `json:"annualIncreasePct"`
	IsOverridden       bool            `json:"isOverridden"`
	Timestamps
}

// SalaryFromModel converts a model.StaffSalary to a SalaryResponse.
func SalaryFromModel(s model.StaffSalary) SalaryResponse {
	return SalaryResponse{
		ID:                 s.ID.String(),
		ScenarioID:         s.ScenarioID.String(),
		Category:           string(s.Category),
		YearIndex:          s.YearIndex,
		MonthlyGrossSalary: s.MonthlyGrossSalary,
		AnnualIncreasePct:  s.AnnualIncreasePct,
		IsOverridden:       s.IsOverridden,
		Timestamps:         Timestamps{CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt},
	}
}

// SalariesFromModels converts a slice of model.StaffSalary to DTOs.
func SalariesFromModels(sals []model.StaffSalary) []SalaryResponse {
	out := make([]SalaryResponse, len(sals))
	for i, s := range sals {
		out[i] = SalaryFromModel(s)
	}
	return out
}

// IncentiveResponse is the API representation of a staff incentive record.
type IncentiveResponse struct {
	ID                 string          `json:"id"`
	ScenarioID         string          `json:"scenarioId"`
	YearIndex          int             `json:"yearIndex"`
	IncentivePct       decimal.Decimal `json:"incentivePct"`
	SpecificIncentives decimal.Decimal `json:"specificIncentives"`
	IsOverridden       bool            `json:"isOverridden"`
	Timestamps
}

// IncentiveFromModel converts a model.StaffIncentive to an IncentiveResponse.
func IncentiveFromModel(inc model.StaffIncentive) IncentiveResponse {
	return IncentiveResponse{
		ID:                 inc.ID.String(),
		ScenarioID:         inc.ScenarioID.String(),
		YearIndex:          inc.YearIndex,
		IncentivePct:       inc.IncentivePct,
		SpecificIncentives: inc.SpecificIncentives,
		IsOverridden:       inc.IsOverridden,
		Timestamps:         Timestamps{CreatedAt: inc.CreatedAt, UpdatedAt: inc.UpdatedAt},
	}
}

// IncentivesFromModels converts a slice of model.StaffIncentive to DTOs.
func IncentivesFromModels(incs []model.StaffIncentive) []IncentiveResponse {
	out := make([]IncentiveResponse, len(incs))
	for i, inc := range incs {
		out[i] = IncentiveFromModel(inc)
	}
	return out
}

// TenantResponse is the API representation of a tenant.
type TenantResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	IsActive bool   `json:"isActive"`
	Timestamps
}

// TenantFromModel converts a model.Tenant to a TenantResponse.
func TenantFromModel(t model.Tenant) TenantResponse {
	return TenantResponse{
		ID:         t.ID.String(),
		Name:       t.Name,
		Slug:       t.Slug,
		IsActive:   t.IsActive,
		Timestamps: Timestamps{CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt},
	}
}
