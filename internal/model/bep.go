// Package model — Break-Even Point (BEP) module data models.
// Implements the full entity set for EBE+™-inspired break-even analysis
// with fixed/variable cost breakdown, sensitivity analysis, and
// optimisation planning.
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ─────────────────────────────────────────────────────────────────────────────
// §1  Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// BEPSource indicates how the snapshot inputs were populated.
type BEPSource string

const (
	BEPSourceManual   BEPSource = "manual"   // entered directly by the user
	BEPSourceImported BEPSource = "imported" // pre-populated from P&L / other module
)

// BEPPlanStatus tracks the lifecycle of an optimisation plan.
type BEPPlanStatus string

const (
	BEPPlanDraft     BEPPlanStatus = "draft"     // work in progress
	BEPPlanValidated BEPPlanStatus = "validated" // approved and promoted to new baseline
)

// FixedCostCategory enumerates the seven standard PCG-derived fixed cost buckets.
type FixedCostCategory string

const (
	FixedCostPayroll   FixedCostCategory = "payroll"   // salaires + charges patronales (64x)
	FixedCostRent      FixedCostCategory = "rent"      // loyers, télécoms, consommables (61x/62x)
	FixedCostLeasing   FixedCostCategory = "leasing"   // crédit-bail, maintenance, réparations (61x)
	FixedCostFees      FixedCostCategory = "fees"      // honoraires, personnel extérieur (62x)
	FixedCostTravel    FixedCostCategory = "travel"    // déplacements, missions, réceptions (625)
	FixedCostMarketing FixedCostCategory = "marketing" // publicité, promotion (623)
	FixedCostOther     FixedCostCategory = "other"     // autres charges fixes
)

// VariableCostCategory enumerates the three standard variable cost buckets per unit.
type VariableCostCategory string

const (
	VarCostMaterials   VariableCostCategory = "materials"   // matières premières, sous-traitance (60x/61x)
	VarCostCommissions VariableCostCategory = "commissions" // commissions, redevances, droits (65x)
	VarCostLogistics   VariableCostCategory = "logistics"   // transports, autres coûts directs (624)
)

// BEPSensitivityType identifies which dimension a sensitivity config applies to.
type BEPSensitivityType string

const (
	SensRevenue   BEPSensitivityType = "revenue"    // BEP vs revenue variation
	SensMargin    BEPSensitivityType = "margin"     // BEP vs contribution margin variation
	SensFixedCost BEPSensitivityType = "fixed_cost" // BEP vs fixed cost variation
)

// ─────────────────────────────────────────────────────────────────────────────
// §2  DB Entities
// ─────────────────────────────────────────────────────────────────────────────

// BEPSnapshot is the root aggregate for one break-even analysis.
// Multiple snapshots can coexist per scenario (e.g. budget vs actuals,
// year-over-year comparison).
type BEPSnapshot struct {
	TenantScoped
	ScenarioID uuid.UUID `gorm:"type:uuid;not null;index"             json:"scenarioId"`

	// User-defined metadata
	Label       string     `gorm:"not null"                             json:"label"`
	FiscalYear  int        `gorm:"default:0"                            json:"fiscalYear"` // 0 = not year-anchored
	PeriodStart *time.Time `gorm:"type:date"                           json:"periodStart,omitempty"`
	PeriodEnd   *time.Time `gorm:"type:date"                           json:"periodEnd,omitempty"`
	Source      BEPSource  `gorm:"not null;default:'manual'"            json:"source"`

	// Primary inputs — the two mandatory values for BEP computation.
	// FixedCostsTotal: total annual fixed costs in €.
	// ContributionMarginPct: (Revenue − VariableCosts) / Revenue expressed as a
	// percentage [0.000001 – 100], stored to 6 decimal places.
	FixedCostsTotal       decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0" json:"fixedCostsTotal"`
	ContributionMarginPct decimal.Decimal `gorm:"type:numeric(12,6);not null;default:0" json:"contributionMarginPct"`

	// Optional input. When provided, volume break-even can be computed.
	// nil = user has not entered an average order value.
	AvgOrderValue *decimal.Decimal `gorm:"type:numeric(18,2)"               json:"avgOrderValue,omitempty"`

	// Notes
	Notes string `gorm:"type:text"                                         json:"notes,omitempty"`
}

// TableName overrides GORM's default "b_e_p_snapshots" naming.
func (BEPSnapshot) TableName() string { return "bep_snapshots" }

// ─────────────────────────────────────────────────────────────────────────────

// FixedCostLine represents one itemised fixed cost within a BEPSnapshot.
// The seven standard categories map directly to the EBE+ Optimisation sheet.
type FixedCostLine struct {
	TenantScoped
	SnapshotID       uuid.UUID         `gorm:"type:uuid;not null;index"  json:"snapshotId"`
	Category         FixedCostCategory `gorm:"not null"                  json:"category"`
	Label            string            `gorm:"not null"                  json:"label"`
	AmountAnnual     decimal.Decimal   `gorm:"type:numeric(18,2);not null;default:0" json:"amountAnnual"`
	IsCustomCategory bool              `gorm:"default:false"             json:"isCustomCategory"`
	SortOrder        int               `gorm:"default:0"                 json:"sortOrder"`
}

// ─────────────────────────────────────────────────────────────────────────────

// VariableCostLine represents one itemised variable cost per unit within a BEPSnapshot.
// The three standard categories map to the EBE+ Optimisation sheet variable cost table.
type VariableCostLine struct {
	TenantScoped
	SnapshotID       uuid.UUID            `gorm:"type:uuid;not null;index"  json:"snapshotId"`
	Category         VariableCostCategory `gorm:"not null"                  json:"category"`
	Label            string               `gorm:"not null"                  json:"label"`
	AmountPerUnit    decimal.Decimal      `gorm:"type:numeric(18,4);not null;default:0" json:"amountPerUnit"`
	IsCustomCategory bool                 `gorm:"default:false"             json:"isCustomCategory"`
	SortOrder        int                  `gorm:"default:0"                 json:"sortOrder"`
}

// ─────────────────────────────────────────────────────────────────────────────

// SensitivityConfig controls the step size and range used when generating
// sensitivity tables. One record per (snapshot, analysis_type) pair.
// Defaults are created automatically when a snapshot is created.
type SensitivityConfig struct {
	TenantScoped
	SnapshotID   uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:uix_sens_config" json:"snapshotId"`
	AnalysisType BEPSensitivityType `gorm:"not null;uniqueIndex:uix_sens_config"           json:"analysisType"`
	// StepSizePct: increment between columns expressed as a percentage point
	// (e.g. 10 for 10% steps, 5 for 5 pp margin steps).
	StepSizePct decimal.Decimal `gorm:"type:numeric(6,2);not null" json:"stepSizePct"`
	// RangePct: symmetric range around base value
	// (e.g. 50 means ±50%, producing 11 columns at 10% step).
	RangePct  decimal.Decimal `gorm:"type:numeric(6,2);not null" json:"rangePct"`
	IsDefault bool            `gorm:"default:true"               json:"isDefault"`
}

func (SensitivityConfig) TableName() string { return "bep_sensitivity_configs" }

// DefaultSensitivityConfigs returns the three standard sensitivity configs
// for a freshly created snapshot, following EBE+ reference model defaults.
func DefaultSensitivityConfigs(tenantID, snapshotID uuid.UUID) []SensitivityConfig {
	return []SensitivityConfig{
		{
			TenantScoped: TenantScoped{TenantID: tenantID},
			SnapshotID:   snapshotID,
			AnalysisType: SensRevenue,
			StepSizePct:  decimal.NewFromInt(10),
			RangePct:     decimal.NewFromInt(50),
			IsDefault:    true,
		},
		{
			TenantScoped: TenantScoped{TenantID: tenantID},
			SnapshotID:   snapshotID,
			AnalysisType: SensMargin,
			StepSizePct:  decimal.NewFromInt(5),
			RangePct:     decimal.NewFromInt(25),
			IsDefault:    true,
		},
		{
			TenantScoped: TenantScoped{TenantID: tenantID},
			SnapshotID:   snapshotID,
			AnalysisType: SensFixedCost,
			StepSizePct:  decimal.NewFromInt(10),
			RangePct:     decimal.NewFromInt(50),
			IsDefault:    true,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────

// OptimisationPlan is a named set of proposed cost reductions linked to a
// baseline BEPSnapshot. Multiple plans can be created per snapshot
// (e.g. "Outsourcing scenario", "Headcount reduction plan Q3").
type OptimisationPlan struct {
	TenantScoped
	SnapshotID uuid.UUID     `gorm:"type:uuid;not null;index" json:"snapshotId"`
	Name       string        `gorm:"not null"                 json:"name"`
	Status     BEPPlanStatus `gorm:"not null;default:'draft'" json:"status"`
	CreatedBy  uuid.UUID     `gorm:"type:uuid"                json:"createdBy"`
	Notes      string        `gorm:"type:text"                json:"notes,omitempty"`
}

func (OptimisationPlan) TableName() string { return "bep_optimisation_plans" }

// ─────────────────────────────────────────────────────────────────────────────

// FixedCostSaving records one proposed saving on a fixed cost line.
// Each saving is linked to a plan and to the specific FixedCostLine it affects.
// PCGAccountRefs stores a JSON array of PCG account codes for the accountant.
type FixedCostSaving struct {
	TenantScoped
	PlanID          uuid.UUID       `gorm:"type:uuid;not null;index"              json:"planId"`
	FixedCostLineID uuid.UUID       `gorm:"type:uuid;not null"                    json:"fixedCostLineId"`
	SavingAmount    decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0" json:"savingAmount"`
	NewAmount       decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0" json:"newAmount"`
	Comment         string          `gorm:"type:text"                             json:"comment,omitempty"`
	// PCGAccountRefs: PCG account codes this saving relates to.
	// Serialised as a JSON array in the TEXT column.
	PCGAccountRefs StringSlice `gorm:"type:text;serializer:json"             json:"pcgAccountRefs,omitempty"`
}

func (FixedCostSaving) TableName() string { return "bep_fixed_cost_savings" }

// ─────────────────────────────────────────────────────────────────────────────

// VariableCostSaving records one proposed saving on a variable cost line.
type VariableCostSaving struct {
	TenantScoped
	PlanID             uuid.UUID       `gorm:"type:uuid;not null;index"              json:"planId"`
	VariableCostLineID uuid.UUID       `gorm:"type:uuid;not null"                    json:"variableCostLineId"`
	SavingAmount       decimal.Decimal `gorm:"type:numeric(18,4);not null;default:0" json:"savingAmount"`
	NewAmount          decimal.Decimal `gorm:"type:numeric(18,4);not null;default:0" json:"newAmount"`
	Comment            string          `gorm:"type:text"                             json:"comment,omitempty"`
}

func (VariableCostSaving) TableName() string { return "bep_variable_cost_savings" }

// ─────────────────────────────────────────────────────────────────────────────

// PCGReviewItem records a user annotation on one PCG account.
// The PCG account list itself is static reference data (see PCGAccounts below);
// only user-checked items with comments are persisted here.
type PCGReviewItem struct {
	TenantScoped
	PlanID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uix_pcg_review" json:"planId"`
	PCGCode  string    `gorm:"not null;uniqueIndex:uix_pcg_review"           json:"pcgCode"`
	PCGLabel string    `gorm:"not null"                                      json:"pcgLabel"`
	Checked  bool      `gorm:"default:false"                                 json:"checked"`
	Comment  string    `gorm:"type:text"                                     json:"comment,omitempty"`
}

func (PCGReviewItem) TableName() string { return "bep_pcg_review_items" }

// ─────────────────────────────────────────────────────────────────────────────
// §3  StringSlice helper (JSON-serialised []string for PCGAccountRefs)
// ─────────────────────────────────────────────────────────────────────────────

// StringSlice is a []string that GORM serialises as a JSON array in a TEXT column.
type StringSlice []string

// ─────────────────────────────────────────────────────────────────────────────
// §4  Static PCG reference data
// ─────────────────────────────────────────────────────────────────────────────

// PCGAccount represents one line of the French Plan Comptable Général.
type PCGAccount struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Group string `json:"group"` // e.g. "60", "61", …, "65"
}

// PCGAccounts is the canonical list of operating expense accounts (60–65)
// relevant to BEP analysis, derived from the EBE+ reference model.
// Future-proofed: this list can be moved to a DB seed table to allow
// admin updates without code changes.
var PCGAccounts = []PCGAccount{
	// ── 60  Achats ────────────────────────────────────────────────────────────
	{"601", "Achats de matières premières et fournitures", "60"},
	{"602", "Achats d'autres approvisionnements", "60"},
	{"604", "Achats d'études et prestations de services", "60"},
	{"605", "Achats de matériel, équipements et travaux", "60"},
	{"606", "Achats non stockés de matières et fournitures", "60"},
	{"607", "Achats de marchandises", "60"},
	{"608", "Frais accessoires d'achats", "60"},

	// ── 61  Services extérieurs ───────────────────────────────────────────────
	{"611", "Sous-traitance générale", "61"},
	{"612", "Redevances de crédit-bail — mobilier", "61"},
	{"6122", "Redevances de crédit-bail — immobilier", "61"},
	{"613", "Locations et charges locatives", "61"},
	{"614", "Charges locatives et de copropriété", "61"},
	{"615", "Entretien et réparations", "61"},
	{"616", "Primes d'assurances", "61"},
	{"617", "Études et recherches", "61"},
	{"618", "Divers (services extérieurs)", "61"},

	// ── 62  Autres services extérieurs ────────────────────────────────────────
	{"621", "Personnel extérieur à l'entreprise", "62"},
	{"6211", "Personnel intérimaire", "62"},
	{"6214", "Personnel détaché ou prêté à l'entreprise", "62"},
	{"622", "Rémunérations d'intermédiaires et honoraires", "62"},
	{"6221", "Commissions et courtages sur achats", "62"},
	{"6222", "Commissions et courtages sur ventes", "62"},
	{"6226", "Honoraires", "62"},
	{"6227", "Frais d'actes et de contentieux", "62"},
	{"623", "Publicité, publications, relations publiques", "62"},
	{"6231", "Annonces et insertions", "62"},
	{"6232", "Échantillons", "62"},
	{"6233", "Foires et expositions", "62"},
	{"6234", "Cadeaux à la clientèle", "62"},
	{"6235", "Primes", "62"},
	{"6236", "Catalogues et imprimés", "62"},
	{"624", "Transports de biens et transports collectifs du personnel", "62"},
	{"625", "Déplacements, missions et réceptions", "62"},
	{"6251", "Voyages et déplacements", "62"},
	{"6256", "Missions", "62"},
	{"6257", "Réceptions", "62"},
	{"626", "Frais postaux et de télécommunications", "62"},
	{"627", "Services bancaires et assimilés", "62"},
	{"628", "Divers (autres services extérieurs)", "62"},

	// ── 63  Impôts, taxes et versements assimilés ─────────────────────────────
	{"631", "Impôts, taxes et versements assimilés sur rémunérations", "63"},
	{"6311", "Taxe sur les salaires", "63"},
	{"6312", "Taxe d'apprentissage", "63"},
	{"6313", "Participation des employeurs à la formation professionnelle continue", "63"},
	{"6314", "Cotisation pour défaut d'investissement obligatoire dans la construction", "63"},
	{"633", "Impôts, taxes et versements assimilés — autres", "63"},
	{"635", "Autres impôts, taxes et versements assimilés", "63"},
	{"6351", "Impôts directs", "63"},
	{"6353", "Taxes sur le chiffre d'affaires non récupérables", "63"},
	{"6358", "Taxes diverses", "63"},

	// ── 64  Charges de personnel ──────────────────────────────────────────────
	{"641", "Rémunérations du personnel", "64"},
	{"6411", "Salaires et appointements", "64"},
	{"6413", "Primes et gratifications", "64"},
	{"6414", "Indemnités et avantages divers", "64"},
	{"642", "Rémunérations du dirigeant social (gérant, etc.)", "64"},
	{"644", "Rémunérations du travail de l'exploitant", "64"},
	{"645", "Charges de sécurité sociale et de prévoyance", "64"},
	{"6451", "Cotisations à l'URSSAF", "64"},
	{"6452", "Cotisations aux mutuelles", "64"},
	{"6453", "Cotisations aux caisses de retraite", "64"},
	{"6454", "Cotisations aux ASSEDIC", "64"},
	{"647", "Autres charges sociales", "64"},
	{"6471", "Prestations directes", "64"},
	{"6472", "Versements au comité d'entreprise", "64"},
	{"648", "Autres charges de personnel", "64"},

	// ── 65  Autres charges de gestion courante ────────────────────────────────
	{"651", "Redevances pour concessions, brevets, licences, marques", "65"},
	{"653", "Jetons de présence", "65"},
	{"654", "Pertes sur créances irrécouvrables", "65"},
	{"655", "Quotes-parts de résultat sur opérations faites en commun", "65"},
	{"658", "Charges diverses de gestion courante", "65"},
}

// ─────────────────────────────────────────────────────────────────────────────
// §5  Compute output types (returned by API, never persisted)
// ─────────────────────────────────────────────────────────────────────────────

// BEPCoreResult holds the four primary break-even outputs derived from
// the two mandatory inputs (FixedCostsTotal, ContributionMarginPct).
type BEPCoreResult struct {
	// BEPRevenue: annual revenue at which the business covers all its costs (€).
	// Undefined (nil) when ContributionMarginPct ≤ 0.
	BEPRevenue *decimal.Decimal `json:"bepRevenue"`
	// BEPVolume: number of average-priced orders needed to reach BEP.
	// Undefined (nil) when AvgOrderValue is nil or BEPRevenue is undefined.
	BEPVolume *decimal.Decimal `json:"bepVolume"`
	// MonthlyBEP: BEPRevenue / 12, for cash-flow sensitivity.
	MonthlyBEP *decimal.Decimal `json:"monthlyBep"`
	// VariableCostPct: 1 − ContributionMarginPct/100 (derived, for display).
	VariableCostPct decimal.Decimal `json:"variableCostPct"`
	// VariableCostPerOrder: AvgOrderValue × VariableCostPct.
	// Undefined (nil) when AvgOrderValue is nil.
	VariableCostPerOrder *decimal.Decimal `json:"variableCostPerOrder"`

	// Validation warnings surfaced to the user.
	Warnings []BEPWarning `json:"warnings,omitempty"`
}

// BEPWarning carries a non-fatal advisory from the compute engine.
type BEPWarning struct {
	Code    string `json:"code"`    // machine-readable, e.g. "margin_below_10_pct"
	Message string `json:"message"` // human-readable
}

// EBERow represents one column in the EBE estimation table.
// Revenue is expressed relative to BEPRevenue using VariationPct.
type EBERow struct {
	VariationPct decimal.Decimal `json:"variationPct"` // e.g. −50, −40, … 0 (BEP), … +50
	Revenue      decimal.Decimal `json:"revenue"`      // k€
	TotalCosts   decimal.Decimal `json:"totalCosts"`   // k€ = FixedCosts + VariableCosts
	EBE          decimal.Decimal `json:"ebe"`          // k€ = Revenue × Margin% − FixedCosts
	IsNegative   bool            `json:"isNegative"`   // true when EBE < 0 (loss zone)
	IsBEP        bool            `json:"isBep"`        // true for the centre column
}

// MarginSensRow represents one column in the BEP-vs-margin sensitivity table.
type MarginSensRow struct {
	MarginVariationPp decimal.Decimal  `json:"marginVariationPp"` // ±Δ percentage points
	MarginPct         decimal.Decimal  `json:"marginPct"`         // base + variation
	BEPRevenue        *decimal.Decimal `json:"bepRevenue"`        // nil if margin ≤ 0
	IsBase            bool             `json:"isBase"`            // true for the zero-variation column
	IsUndefined       bool             `json:"isUndefined"`       // true when margin ≤ 0
}

// FixedCostSensRow represents one column in the BEP-vs-fixed-cost sensitivity table.
type FixedCostSensRow struct {
	CostVariationPct decimal.Decimal `json:"costVariationPct"` // e.g. −50, −40, … 0, … +50
	NewFixedCosts    decimal.Decimal `json:"newFixedCosts"`    // k€
	MarginPct        decimal.Decimal `json:"marginPct"`        // held constant
	BEPRevenue       decimal.Decimal `json:"bepRevenue"`       // k€
	IsBase           bool            `json:"isBase"`           // true for the zero-variation column
}

// BEPReport is the full computed report returned by GET /bep/snapshots/{id}/report.
type BEPReport struct {
	SnapshotID uuid.UUID          `json:"snapshotId"`
	Core       BEPCoreResult      `json:"core"`
	EBETable   []EBERow           `json:"ebeTable"`   // 11 columns, ±range% in step% increments
	MarginSens []MarginSensRow    `json:"marginSens"` // 11 columns, ±range pp in step pp increments
	CostSens   []FixedCostSensRow `json:"costSens"`   // 11 columns, ±range% in step% increments
}

// OptimisedCostState shows baseline vs optimised for one cost dimension.
type OptimisedCostState struct {
	Current   decimal.Decimal `json:"current"`   // baseline value
	Optimised decimal.Decimal `json:"optimised"` // after applying savings
	DeltaAbs  decimal.Decimal `json:"deltaAbs"`  // current − optimised (positive = saving)
	DeltaPct  decimal.Decimal `json:"deltaPct"`  // Δ% from baseline
}

// ─────────────────────────────────────────────────────────────────────────────
// Multi-year break-even (derived from plan — not persisted)
// ─────────────────────────────────────────────────────────────────────────────

// MultiYearBEPRow holds break-even data for one plan year.
// All monetary fields are in the same unit as the plan (€).
type MultiYearBEPRow struct {
	Year int `json:"year"` // 1-based plan year (1–5)

	// Cost / revenue inputs for this year
	FixedCosts            decimal.Decimal `json:"fixedCosts"`            // payroll + opex (€)
	Revenue               decimal.Decimal `json:"revenue"`               // planned net sales (€)
	ContributionMarginPct decimal.Decimal `json:"contributionMarginPct"` // 0–100

	// Break-even output
	BEPRevenue          *decimal.Decimal `json:"bepRevenue"` // nil when margin ≤ 0
	BEPRevenueUndefined bool             `json:"bepRevenueUndefined"`
	RevenueAboveBEP     bool             `json:"revenueAboveBep"` // planned revenue ≥ BEP revenue

	// Profitability
	AnnualEBE     decimal.Decimal `json:"annualEbe"`     // Revenue × Margin% − FixedCosts (€)
	CumulativeEBE decimal.Decimal `json:"cumulativeEbe"` // running sum from year 1

	// Flags
	IsFirstAnnualBEP         bool `json:"isFirstAnnualBep"`         // first year with annual EBE > 0
	IsCumulativeBEPCrossover bool `json:"isCumulativeBepCrossover"` // year cumulative EBE first ≥ 0
}

// MultiYearBEPReport is returned by GET /bep/multi-year-report.
// It requires no snapshot — it is derived entirely from the plan.
type MultiYearBEPReport struct {
	Years [5]MultiYearBEPRow `json:"years"`

	// First year in which annual EBE is positive (nil = never within 5 years)
	FirstProfitableYear *int `json:"firstProfitableYear,omitempty"`

	// Year in which cumulative EBE first turns non-negative (payback year)
	CumulativeBEPYear *int `json:"cumulativeBepYear,omitempty"`
	// Estimated month within that year at which cumulative crossover occurs (1–12)
	CumulativeBEPMonth *int `json:"cumulativeBepMonth,omitempty"`

	// Total cumulative EBE at end of year 5 (positive = profitable overall)
	TotalCumulativeEBE decimal.Decimal `json:"totalCumulativeEbe"`
}

// OptimisedBEPReport is returned by GET /bep/snapshots/{id}/plans/{planId}/report.
type OptimisedBEPReport struct {
	SnapshotID uuid.UUID `json:"snapshotId"`
	PlanID     uuid.UUID `json:"planId"`

	// Baseline (unchanged)
	BaselineFixedCosts decimal.Decimal  `json:"baselineFixedCosts"`
	BaselineMarginPct  decimal.Decimal  `json:"baselineMarginPct"`
	BaselineBEPRevenue *decimal.Decimal `json:"baselineBepRevenue"`
	BaselineBEPVolume  *decimal.Decimal `json:"baselineBepVolume"`

	// Fixed cost optimisation
	FixedCosts OptimisedCostState `json:"fixedCosts"`

	// Variable cost optimisation (margin improvement)
	VariableCostPerUnit OptimisedCostState `json:"variableCostPerUnit"`
	MarginPct           OptimisedCostState `json:"marginPct"`

	// Combined optimised break-even
	OptimisedBEPRevenue *decimal.Decimal `json:"optimisedBepRevenue"`
	OptimisedBEPVolume  *decimal.Decimal `json:"optimisedBepVolume"`

	// Improvement deltas
	BEPRevenueImprovementPct *decimal.Decimal `json:"bepRevenueImprovementPct"` // nil if baseline undefined
	BEPVolumeImprovement     *decimal.Decimal `json:"bepVolumeImprovement"`     // baseline volume − optimised volume

	Warnings []BEPWarning `json:"warnings,omitempty"`
}
