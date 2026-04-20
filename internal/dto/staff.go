package dto

import (
	"ascenda/internal/model"

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

// ─────────────────────────────────────────────────────────────────────────
// Payroll Summary DTO
// ─────────────────────────────────────────────────────────────────────────

// StaffPayrollYearDTO flattens payroll + functional breakdown for one year into
// the shape expected by the frontend summary table.
type StaffPayrollYearDTO struct {
	YearIndex        int                        `json:"yearIndex"`
	TotalPayroll     decimal.Decimal            `json:"totalPayroll"`     // gross salary (excl. employer charges)
	EmployerCharges  decimal.Decimal            `json:"employerCharges"`  // employer social/tax charges
	TotalWithCharges decimal.Decimal            `json:"totalWithCharges"` // gross + employer charges
	Incentives       decimal.Decimal            `json:"incentives"`       // total incentives (amount + specific)
	TotalStaffCost   decimal.Decimal            `json:"totalStaffCost"`   // totalWithCharges + incentives
	ByFunction       map[string]decimal.Decimal `json:"byFunction"`       // rnd, production, sales_marketing, ga
}

// StaffPayrollSummaryResponse is the flattened API response for GET /staff/summary.
type StaffPayrollSummaryResponse struct {
	ScenarioID string                `json:"scenarioId"`
	Years      []StaffPayrollYearDTO `json:"years"`
}

// StaffPayrollSummaryFromModel converts a model.StaffPayrollSummary to the API response DTO.
func StaffPayrollSummaryFromModel(s *model.StaffPayrollSummary, scenarioID string) StaffPayrollSummaryResponse {
	resp := StaffPayrollSummaryResponse{
		ScenarioID: scenarioID,
		Years:      make([]StaffPayrollYearDTO, 5),
	}

	// EmployerTaxRate is stored as a fraction (e.g. 0.45 = 45%).
	// SubtotalPayroll = grossBase × (1 + EmployerTaxRate)
	// → grossBase     = SubtotalPayroll / (1 + EmployerTaxRate)
	// → employerCharges = SubtotalPayroll − grossBase
	onePlusRate := decimal.NewFromInt(1).Add(s.EmployerTaxRate)

	for i := range s.Payroll {
		year := s.Payroll[i]
		fb := s.FunctionalBreakdown[i]

		var grossBase, employerCharges decimal.Decimal
		if !onePlusRate.IsZero() {
			grossBase = year.SubtotalPayroll.Div(onePlusRate)
			employerCharges = year.SubtotalPayroll.Sub(grossBase)
		}

		resp.Years[i] = StaffPayrollYearDTO{
			YearIndex:        year.YearIndex,
			TotalPayroll:     grossBase,
			EmployerCharges:  employerCharges,
			TotalWithCharges: year.SubtotalPayroll,
			Incentives:       year.SubtotalIncentives,
			TotalStaffCost:   year.TotalPayroll,
			ByFunction: map[string]decimal.Decimal{
				"rnd":            fb.RnD,
				"production":     fb.Production,
				"sales_marketing": fb.Sales,
				"ga":             fb.GnA,
			},
		}
	}

	return resp
}

// TenantResponse is the API representation of a tenant.
type TenantResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	AICredits int    `json:"aiCredits"`
	IsActive  bool   `json:"isActive"`
	Timestamps
}

// TenantFromModel converts a model.Tenant to a TenantResponse.
func TenantFromModel(t model.Tenant) TenantResponse {
	return TenantResponse{
		ID:         t.ID.String(),
		Name:       t.Name,
		Slug:       t.Slug,
		AICredits:  t.AICredits,
		IsActive:   t.IsActive,
		Timestamps: Timestamps{CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt},
	}
}
