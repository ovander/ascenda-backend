package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ─────────────────────────────────────────────────────────────────────────────
// §1  Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// ShareClassType identifies the category of shares.
type ShareClassType string

const (
	ShareClassCommon      ShareClassType = "common"      // actions ordinaires
	ShareClassPreferredA  ShareClassType = "preferred_a" // Série A
	ShareClassPreferredB  ShareClassType = "preferred_b" // Série B
	ShareClassPreferredC  ShareClassType = "preferred_c" // Série C
	ShareClassPreferredD  ShareClassType = "preferred_d" // Série D
	ShareClassConvertible ShareClassType = "convertible" // from converted notes
	ShareClassESOPPool    ShareClassType = "esop_pool"   // option pool reserve
)

// LiquidationPreference defines payout priority in a liquidation event.
type LiquidationPreference string

const (
	LiqPrefNone             LiquidationPreference = "none"              // common stock — pro-rata only
	LiqPrefNonParticipating LiquidationPreference = "non_participating" // 1× then convert to common
	LiqPrefParticipating    LiquidationPreference = "participating"     // 1× + pro-rata on remainder
	LiqPrefCapped           LiquidationPreference = "capped"            // participating up to defined cap
)

// AntiDilutionType identifies the anti-dilution mechanism.
type AntiDilutionType string

const (
	AntiDilutionNone           AntiDilutionType = "none"
	AntiDilutionBroadWeighted  AntiDilutionType = "broad_weighted"  // most common for institutional rounds
	AntiDilutionNarrowWeighted AntiDilutionType = "narrow_weighted"
	AntiDilutionFullRatchet    AntiDilutionType = "full_ratchet" // most investor-friendly
)

// ShareholderType classifies the type of equity holder.
type ShareholderType string

const (
	ShareholderFounder     ShareholderType = "founder"     // fondateur / dirigeant
	ShareholderAssociate   ShareholderType = "associate"   // associé initial
	ShareholderInvestor    ShareholderType = "investor"    // new investor group (round-specific)
	ShareholderESOPPool    ShareholderType = "esop"        // stock option pool
	ShareholderInstitution ShareholderType = "institution" // institutional (VC/PE fund)
)

// RoundEventType classifies the type of equity event.
type RoundEventType string

const (
	RoundEventFunding    RoundEventType = "funding_round"  // capital increase with new investor
	RoundEventConversion RoundEventType = "conversion"     // convertible note → equity
	RoundEventSplit      RoundEventType = "stock_split"    // stock split / multiplication
	RoundEventESOPExpand RoundEventType = "esop_expansion" // ESOP pool creation / expansion
	RoundEventSecondary  RoundEventType = "secondary"      // secondary transfer (no new money)
)

// StockOptionInstrument identifies the legal instrument.
type StockOptionInstrument string

const (
	SOIBSAWarrant  StockOptionInstrument = "BSA"   // Bon de Souscription d'Actions
	SOIBCE         StockOptionInstrument = "BCE"   // Bon de Créateur d'Entreprise
	SOIBSPCE       StockOptionInstrument = "BSPCE" // Bon de Souscription de Parts de Créateur d'Entreprise
	SOIStockOption StockOptionInstrument = "SO"    // classic stock option
	SOIRSU         StockOptionInstrument = "RSU"   // Restricted Stock Unit (AGA in France)
)

// OptionLifecycleState tracks where an option batch is in its lifecycle.
type OptionLifecycleState string

const (
	OptionStateVoted      OptionLifecycleState = "voted"      // pool authorised by AGM
	OptionStateAttributed OptionLifecycleState = "attributed" // granted to beneficiary
	OptionStateExercised  OptionLifecycleState = "exercised"  // converted to shares
	OptionStateCancelled  OptionLifecycleState = "cancelled"  // forfeited / lapsed
)

// ValuationScenarioType identifies the FastValo calculation mode.
type ValuationScenarioType string

const (
	// A: (Multiple + Horizon) → IRR
	// IRR = multiple^(1/years) − 1
	ValScenMultipleToIRR ValuationScenarioType = "multiple_to_irr"

	// B: (Investment + Annual yield) → Terminal Value + Multiple
	// Terminal = investment × (1 + yield)^years
	ValScenInvestmentToTerminal ValuationScenarioType = "investment_to_terminal"

	// C: (Investment + Final% + Exit value) → IRR + NPV + Investor share
	// Investor share = exit_value × final%
	// NPV = investor_share / (1 + discount_rate)^years
	ValScenInvestmentPctToIRR ValuationScenarioType = "investment_pct_to_irr"

	// D: (New money + Post-money %) → Pre/Post-money Valuation
	// Post-money = new_money / (pct/100)
	// Pre-money = post_money − new_money
	ValScenNewMoneyToPremoney ValuationScenarioType = "new_money_to_premoney"
)

// ─────────────────────────────────────────────────────────────────────────────
// §2  Sub-module 1 — Cap Table Simulation (IngéFi) — Stored entities
// ─────────────────────────────────────────────────────────────────────────────

// CapTableCompany stores company-level equity configuration for a scenario.
// One record per scenario (the scenario already anchors tenant + plan).
// Country defaults are applied from PlanConfig.Country via GetCapTableCountryProfile().
type CapTableCompany struct {
	TenantScoped
	ScenarioID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"scenarioId"`

	// Identity
	CompanyName  string     `gorm:"type:varchar(255);not null" json:"companyName"`
	LegalForm    string     `gorm:"type:varchar(50)" json:"legalForm"` // from CountryProfile.LegalForms
	CreationDate *time.Time `json:"creationDate,omitempty"`

	// Equity configuration — auto-filled from country profile, user-overridable
	Currency          string          `gorm:"type:varchar(3);not null;default:'EUR'" json:"currency"`
	CurrencySymbol    string          `gorm:"type:varchar(5);not null;default:'€'" json:"currencySymbol"`
	NominalValueCents int64           `gorm:"not null;default:1" json:"nominalValueCents"` // sub-cent = 0
	InitialShares     int64           `gorm:"not null;default:0" json:"initialShares"`
	InitialCapitalK   decimal.Decimal `gorm:"type:numeric(15,2)" json:"initialCapitalK"`

	// Display preferences — auto-filled from country profile, user-overridable
	DisplayLanguage string `gorm:"type:varchar(5);not null;default:'fr'" json:"displayLanguage"` // BCP-47
	DateFormat      string `gorm:"type:varchar(12);not null;default:'DD/MM/YYYY'" json:"dateFormat"`

	// Localised terminology — auto-filled from country profile
	BookEquityTerm   string `gorm:"type:varchar(100)" json:"bookEquityTerm"`   // "Capitaux propres"
	ShareCapitalTerm string `gorm:"type:varchar(100)" json:"shareCapitalTerm"` // "Capital social"

	// Module limits
	MaxPhases       int             `gorm:"not null;default:7" json:"maxPhases"`
	FounderAlertPct decimal.Decimal `gorm:"type:numeric(5,2);default:20.00" json:"founderAlertPct"`
}

// TableName specifies the table name for GORM.
func (CapTableCompany) TableName() string { return "cap_table_companies" }

// CapTableShareClass defines an equity class with its rights and preferences.
type CapTableShareClass struct {
	TenantScoped
	ScenarioID          uuid.UUID             `gorm:"type:uuid;not null;uniqueIndex:uix_share_class" json:"scenarioId"`
	ClassType           ShareClassType        `gorm:"type:varchar(30);not null;uniqueIndex:uix_share_class" json:"classType"`
	Label               string                `gorm:"type:varchar(100)" json:"label"` // e.g. "Série A Préférées"
	VotingRights        bool                  `gorm:"not null;default:true" json:"votingRights"`
	VotingMultiple      decimal.Decimal       `gorm:"type:numeric(5,2);default:1.00" json:"votingMultiple"` // double-vote etc.
	LiquidationPref     LiquidationPreference `gorm:"type:varchar(30);not null;default:'none'" json:"liquidationPref"`
	LiquidationMultiple decimal.Decimal       `gorm:"type:numeric(5,2);default:1.00" json:"liquidationMultiple"` // 1×, 2×
	ParticipationCap    *decimal.Decimal      `gorm:"type:numeric(5,2)" json:"participationCap,omitempty"`
	AntiDilution        AntiDilutionType      `gorm:"type:varchar(30);not null;default:'none'" json:"antiDilution"`
	ConversionRatio     decimal.Decimal       `gorm:"type:numeric(10,6);default:1.000000" json:"conversionRatio"` // preferred→common
	DividendRatePct     *decimal.Decimal      `gorm:"type:numeric(5,2)" json:"dividendRatePct,omitempty"` // cumulative dividend
	SortOrder           int                   `gorm:"not null;default:0" json:"sortOrder"`
}

// TableName specifies the table name for GORM.
func (CapTableShareClass) TableName() string { return "cap_table_share_classes" }

// CapTableShareholder represents an equity holder.
// Investor groups are created one-per-round; founders persist across all rounds.
type CapTableShareholder struct {
	TenantScoped
	ScenarioID      uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:uix_cap_shareholder" json:"scenarioId"`
	Name            string          `gorm:"type:varchar(255);not null;uniqueIndex:uix_cap_shareholder" json:"name"`
	Type            ShareholderType `gorm:"type:varchar(30);not null" json:"type"`
	ClassType       ShareClassType  `gorm:"type:varchar(30);not null" json:"classType"`
	InitShares      int64           `gorm:"not null;default:0" json:"initShares"`      // shares at founding / entry
	InvestedAmountK decimal.Decimal `gorm:"type:numeric(15,2)" json:"investedAmountK"` // k currency, total across all rounds
	Note            string          `gorm:"type:text" json:"note,omitempty"`
	SortOrder       int             `gorm:"not null;default:0" json:"sortOrder"`
}

// TableName specifies the table name for GORM.
func (CapTableShareholder) TableName() string { return "cap_table_shareholders" }

// CapTableRound represents a single equity event (funding round, conversion, split).
// Linked to the Financing module via FinancingSourceType when applicable.
type CapTableRound struct {
	TenantScoped
	ScenarioID uuid.UUID `gorm:"type:uuid;not null;index" json:"scenarioId"`

	// Identity
	PhaseNumber int            `gorm:"not null" json:"phaseNumber"` // 1–N; 0 = founding
	Label       string         `gorm:"type:varchar(100);not null" json:"label"`
	EventDate   *time.Time     `json:"eventDate,omitempty"`
	EventType   RoundEventType `gorm:"type:varchar(30);not null" json:"eventType"`

	// Link to Financing module (nullable)
	FinancingSourceType *string `gorm:"type:varchar(50)" json:"financingSourceType,omitempty"`

	// Share mechanics
	NominalValueCents int64           `gorm:"not null;default:1" json:"nominalValueCents"` // nominal at this phase
	SplitCoefficient  decimal.Decimal `gorm:"type:numeric(10,6);default:1.000000" json:"splitCoefficient"` // 1 = no split
	NewSharesCreated  int64           `gorm:"not null;default:0" json:"newSharesCreated"`

	// Valuation inputs — user provides ONE of the two; engine derives the other
	PreMoneyValuationK *decimal.Decimal `gorm:"type:numeric(15,2)" json:"preMoneyValuationK,omitempty"`
	AmountRaisedK      decimal.Decimal  `gorm:"type:numeric(15,2)" json:"amountRaisedK"`  // k currency
	PctGranted         decimal.Decimal  `gorm:"type:numeric(8,4)" json:"pctGranted"`       // % granted to new investors

	// Share class for new shares
	ShareClassType ShareClassType `gorm:"type:varchar(30);not null" json:"shareClassType"`

	// Book equity at this phase (for goodwill computation)
	BookEquityK *decimal.Decimal `gorm:"type:numeric(15,2)" json:"bookEquityK,omitempty"` // capitaux propres

	// Emission premium reintegration (optional)
	EmissionPremiumReintegK *decimal.Decimal `gorm:"type:numeric(15,2)" json:"emissionPremiumReintegK,omitempty"`

	SortOrder int `gorm:"not null;default:0" json:"sortOrder"`

	// FiPlan sync — optional link to the capital_increase FiPlan entry for this round.
	// FiscalYearIndex is 0-based (0 = Year 1 … 4 = Year 5) and identifies which
	// annual slot the amount has been pushed to. FiplanSynced is set to true after
	// a successful sync and cleared on unlink.
	FiscalYearIndex    *int             `gorm:"column:fiscal_year_index"          json:"fiscalYearIndex,omitempty"`
	FiplanSynced       bool             `gorm:"not null;default:false"            json:"fiplanSynced"`
	// FiplanSyncedAmountK is the AmountRaisedK value at the time of the last sync.
	// If the round amount is later renegotiated, comparing this field with
	// AmountRaisedK reveals the divergence without querying FiPlan.
	FiplanSyncedAmountK *decimal.Decimal `gorm:"type:numeric(15,2);column:fiplan_synced_amount_k" json:"fiplanSyncedAmountK,omitempty"`
	// IsSyncAmountDivergent is computed at read time (not stored).
	// True when FiplanSynced=true and AmountRaisedK ≠ FiplanSyncedAmountK.
	IsSyncAmountDivergent bool `gorm:"-" json:"isSyncAmountDivergent"`

	// Opening balance sync — for founding-capital rounds (France: capital social
	// deposited before company registration).  When true the round's AmountRaisedK
	// has been written to opening_balance.share_capital and cash_and_securities.
	// Mutually exclusive with FiplanSynced: a round either seeds the opening
	// balance (past) or appears as a forecast resource (future), never both.
	OpeningBalanceSynced       bool             `gorm:"not null;default:false"            json:"openingBalanceSynced"`
	OpeningBalanceSyncedAmountK *decimal.Decimal `gorm:"type:numeric(15,2);column:opening_balance_synced_amount_k" json:"openingBalanceSyncedAmountK,omitempty"`
	// IsOpeningBalanceDivergent is computed at read time (not stored).
	IsOpeningBalanceDivergent bool `gorm:"-" json:"isOpeningBalanceDivergent"`
}

// TableName specifies the table name for GORM.
func (CapTableRound) TableName() string { return "cap_table_rounds" }

// CapTablePosition records how each shareholder's position changes after a round.
// Stored for snapshot consistency; re-computable from scratch at any time.
type CapTablePosition struct {
	TenantScoped
	RoundID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uix_cap_position" json:"roundId"`
	ShareholderID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uix_cap_position" json:"shareholderId"`

	// Shares before / after split and issuance
	SharesBeforeSplit int64 `json:"sharesBeforeSplit"`
	SharesAfterSplit  int64 `json:"sharesAfterSplit"`  // adjusted by split coefficient
	NewSharesReceived int64 `json:"newSharesReceived"` // 0 for diluted holders
	SharesAfterRound  int64 `json:"sharesAfterRound"`  // sharesAfterSplit + newSharesReceived

	// Ownership percentages
	PctBasicBefore  decimal.Decimal `gorm:"type:numeric(8,4)" json:"pctBasicBefore"`
	PctBasicAfter   decimal.Decimal `gorm:"type:numeric(8,4)" json:"pctBasicAfter"`
	PctFullyDiluted decimal.Decimal `gorm:"type:numeric(8,4)" json:"pctFullyDiluted"` // includes outstanding options
	DilutionDelta   decimal.Decimal `gorm:"type:numeric(8,4)" json:"dilutionDelta"`   // pctBefore - pctAfter

	// Implied value
	ImpliedValueK decimal.Decimal `gorm:"type:numeric(15,2)" json:"impliedValueK"` // shares × price-per-share

	// Investment tracking
	AmountInvestedK decimal.Decimal `gorm:"type:numeric(15,2)" json:"amountInvestedK"` // 0 for non-investing holders
}

// TableName specifies the table name for GORM.
func (CapTablePosition) TableName() string { return "cap_table_positions" }

// ─────────────────────────────────────────────────────────────────────────────
// §3  Sub-module 2 — Stock Option Plans (FastStockOption) — Stored entities
// ─────────────────────────────────────────────────────────────────────────────

// StockOptionPlan represents one option plan (pool) attached to a funding phase.
// Each phase may have up to 6 concurrent plans (reference model constraint).
type StockOptionPlan struct {
	TenantScoped
	ScenarioID    uuid.UUID             `gorm:"type:uuid;not null;index" json:"scenarioId"`
	RoundID       *uuid.UUID            `gorm:"type:uuid;index" json:"roundId,omitempty"` // phase at which plan was created
	PlanLabel     string                `gorm:"type:varchar(100);not null" json:"planLabel"`
	Instrument    StockOptionInstrument `gorm:"type:varchar(20);not null" json:"instrument"`
	ExercisePrice decimal.Decimal       `gorm:"type:numeric(12,4);not null" json:"exercisePrice"` // prix d'exercice

	// Lifecycle totals (managed via OptionGrants)
	OptionsVoted      int64 `gorm:"not null;default:0" json:"optionsVoted"`      // pool authorised by AGM
	OptionsAttributed int64 `gorm:"not null;default:0" json:"optionsAttributed"` // granted to beneficiaries
	OptionsExercised  int64 `gorm:"not null;default:0" json:"optionsExercised"`  // converted to shares (cumulative)
	OptionsCancelled  int64 `gorm:"not null;default:0" json:"optionsCancelled"`  // forfeited / lapsed

	// BSPCE eligibility checks (French tax law — informational)
	CompanyAgeAtGrantYears *int  `json:"companyAgeAtGrantYears,omitempty"` // must be < 15 for BSPCE
	IsEligibleBSPCE        *bool `json:"isEligibleBspce,omitempty"`

	SortOrder int `gorm:"not null;default:0" json:"sortOrder"`
}

// TableName specifies the table name for GORM.
func (StockOptionPlan) TableName() string { return "stock_option_plans" }

// OptionsReserve returns unallocated headroom.
func (p *StockOptionPlan) OptionsReserve() int64 {
	return p.OptionsVoted - p.OptionsAttributed
}

// OptionGrant records options granted to one beneficiary under one plan at one phase.
type OptionGrant struct {
	TenantScoped
	PlanID        uuid.UUID `gorm:"type:uuid;not null;index" json:"planId"`
	ShareholderID uuid.UUID `gorm:"type:uuid;not null;index" json:"shareholderId"`
	RoundID       uuid.UUID `gorm:"type:uuid;not null;index" json:"roundId"` // phase of grant

	OptionsGranted   int64 `gorm:"not null;default:0" json:"optionsGranted"`
	OptionsExercised int64 `gorm:"not null;default:0" json:"optionsExercised"` // cumulative from origin
	OptionsCancelled int64 `gorm:"not null;default:0" json:"optionsCancelled"`

	// Vesting schedule (metadata; enforcement is future scope)
	VestingStart  *time.Time `json:"vestingStart,omitempty"`
	VestingMonths *int       `json:"vestingMonths,omitempty"`
	CliffMonths   *int       `json:"cliffMonths,omitempty"`

	Note string `gorm:"type:text" json:"note,omitempty"`
}

// TableName specifies the table name for GORM.
func (OptionGrant) TableName() string { return "option_grants" }

// OptionsOutstanding returns options granted but not yet exercised or cancelled.
func (g *OptionGrant) OptionsOutstanding() int64 {
	return g.OptionsGranted - g.OptionsExercised - g.OptionsCancelled
}

// ─────────────────────────────────────────────────────────────────────────────
// §4  Sub-module 3 — Company Valuation (FastValo) — Stored entities
// ─────────────────────────────────────────────────────────────────────────────

// ValuationScenario stores one FastValo calculation, including both inputs and results.
// Results are re-computed on load; storage provides history and audit trail.
type ValuationScenario struct {
	TenantScoped
	ScenarioID uuid.UUID             `gorm:"type:uuid;not null;index" json:"scenarioId"`
	CalcType   ValuationScenarioType `gorm:"type:varchar(40);not null" json:"calcType"`
	Label      string                `gorm:"type:varchar(100)" json:"label"` // user-given name

	// ── Inputs ────────────────────────────────────────────────────────────────
	InvestmentK       *decimal.Decimal `gorm:"type:numeric(15,2)" json:"investmentK,omitempty"`       // k currency
	HorizonYears      *decimal.Decimal `gorm:"type:numeric(5,2)" json:"horizonYears,omitempty"`       // exit horizon (may be fractional)
	AnnualYieldPct    *decimal.Decimal `gorm:"type:numeric(8,4)" json:"annualYieldPct,omitempty"`     // % per year
	FinalInvestorPct  *decimal.Decimal `gorm:"type:numeric(8,4)" json:"finalInvestorPct,omitempty"`   // % at exit
	ExitCompanyValueK *decimal.Decimal `gorm:"type:numeric(15,2)" json:"exitCompanyValueK,omitempty"` // exit valuation
	MoneyMultiple     *decimal.Decimal `gorm:"type:numeric(10,4)" json:"moneyMultiple,omitempty"`
	DiscountRatePct   *decimal.Decimal `gorm:"type:numeric(8,4)" json:"discountRatePct,omitempty"`  // for NPV
	PreMoneyInputK    *decimal.Decimal `gorm:"type:numeric(15,2)" json:"preMoneyInputK,omitempty"`  // scenario D alt input
	NewMoneyK         *decimal.Decimal `gorm:"type:numeric(15,2)" json:"newMoneyK,omitempty"`       // scenario D

	// ── Computed Outputs (re-derived on every read) ───────────────────────────
	OutIRRPct         *decimal.Decimal `gorm:"type:numeric(8,4)" json:"outIrrPct,omitempty"`
	OutTerminalValueK *decimal.Decimal `gorm:"type:numeric(15,2)" json:"outTerminalValueK,omitempty"`
	OutMultiple       *decimal.Decimal `gorm:"type:numeric(10,4)" json:"outMultiple,omitempty"`
	OutInvestorShareK *decimal.Decimal `gorm:"type:numeric(15,2)" json:"outInvestorShareK,omitempty"`
	OutNPVK           *decimal.Decimal `gorm:"type:numeric(15,2)" json:"outNpvK,omitempty"`
	OutPreMoneyK      *decimal.Decimal `gorm:"type:numeric(15,2)" json:"outPreMoneyK,omitempty"`
	OutPostMoneyK     *decimal.Decimal `gorm:"type:numeric(15,2)" json:"outPostMoneyK,omitempty"`

	Currency        string `gorm:"type:varchar(3);default:'EUR'" json:"currency"`
	DisplayLanguage string `gorm:"type:varchar(5);default:'fr'" json:"displayLanguage"` // fr|en
}

// TableName specifies the table name for GORM.
func (ValuationScenario) TableName() string { return "valuation_scenarios" }

// ─────────────────────────────────────────────────────────────────────────────
// §5  Scenario Branching
// ─────────────────────────────────────────────────────────────────────────────

// CapTableScenarioBranch allows users to fork the cap table from any phase and
// explore alternative raise amounts, valuations, or option pool sizes.
type CapTableScenarioBranch struct {
	TenantScoped
	ScenarioID          uuid.UUID  `gorm:"type:uuid;not null;index" json:"scenarioId"` // parent Ascenda scenario
	Name                string     `gorm:"type:varchar(100);not null" json:"name"`
	Description         string     `gorm:"type:text" json:"description,omitempty"`
	BranchedFromRoundID *uuid.UUID `gorm:"type:uuid" json:"branchedFromRoundId,omitempty"` // nil = branch from founding
	CreatedByUserID     uuid.UUID  `gorm:"type:uuid;not null" json:"createdByUserId"`
	Status              string     `gorm:"type:varchar(20);not null;default:'draft'" json:"status"` // draft|approved|archived
}

// TableName specifies the table name for GORM.
func (CapTableScenarioBranch) TableName() string { return "cap_table_scenario_branches" }

// ─────────────────────────────────────────────────────────────────────────────
// §6  Computed Report Structures (never stored)
// ─────────────────────────────────────────────────────────────────────────────

// CapTableReport is the complete computed output of the Cap Table module.
type CapTableReport struct {
	IngeFi      IngeFiReport      `json:"ingeFi"`
	StockOption StockOptionReport `json:"stockOption"`
	FastValo    FastValoReport    `json:"fastValo"`
	Warnings    []ValidationWarning `json:"warnings"`
}

// ── 6.1 IngéFi report ─────────────────────────────────────────────────────────

// IngeFiReport is the cap table evolution (multi-round dilution and valuation).
type IngeFiReport struct {
	Phases    []IngeFiPhase     `json:"phases"`    // one entry per round including founding
	Matrix    CapTableMatrix    `json:"matrix"`    // shareholder × phase ownership grid
	Waterfall DilutionWaterfall `json:"waterfall"` // founder dilution funnel
	Charts    IngeFiCharts      `json:"charts"`
}

// IngeFiPhase — computed state at one funding round.
type IngeFiPhase struct {
	PhaseNumber         int             `json:"phaseNumber"`
	Label               string          `json:"label"`
	TotalSharesBefore   int64           `json:"totalSharesBefore"`
	TotalSharesAfter    int64           `json:"totalSharesAfter"`
	PreMoneyValuationK  decimal.Decimal `json:"preMoneyValuationK"`
	PostMoneyValuationK decimal.Decimal `json:"postMoneyValuationK"`
	AmountRaisedK       decimal.Decimal `json:"amountRaisedK"`
	SharePriceEur       decimal.Decimal `json:"sharePriceEur"`       // post-money / total shares after
	NominalValueCents   int64           `json:"nominalValueCents"`
	EmissionPremium     EmissionPremiumDetail `json:"emissionPremium"`
	Goodwill            decimal.Decimal `json:"goodwill"`            // post-money - book equity
	SplitCoefficient    decimal.Decimal `json:"splitCoefficient"`
	ValuationGrowthPct  decimal.Decimal `json:"valuationGrowthPct"`  // (postN - postN-1) / postN-1
	ValuationMultiple   decimal.Decimal `json:"valuationMultiple"`   // postN / postN-1

	// Per-shareholder impacts
	Positions []IngeFiShareholderPosition `json:"positions"`
}

// EmissionPremiumDetail details the emission premium computation.
type EmissionPremiumDetail struct {
	PerShareK            decimal.Decimal `json:"perShareK"`            // share_price - nominal
	TotalK               decimal.Decimal `json:"totalK"`               // per_share × new_shares_created
	ReintegratedK        decimal.Decimal `json:"reintegratedK"`        // amount folded back into capital
	AdjustedNominalCents int64           `json:"adjustedNominalCents"` // (capital + reinteg) / total_shares
}

// IngeFiShareholderPosition — one holder at one phase.
type IngeFiShareholderPosition struct {
	ShareholderID uuid.UUID       `json:"shareholderId"`
	Name          string          `json:"name"`
	Type          ShareholderType `json:"type"`
	ClassType     ShareClassType  `json:"classType"`

	SharesBeforeSplit int64 `json:"sharesBeforeSplit"`
	SharesAfterSplit  int64 `json:"sharesAfterSplit"`
	NewSharesReceived int64 `json:"newSharesReceived"`
	SharesAfterRound  int64 `json:"sharesAfterRound"`

	PctBasicBefore  decimal.Decimal `json:"pctBasicBefore"`
	PctBasicAfter   decimal.Decimal `json:"pctBasicAfter"`
	PctFullyDiluted decimal.Decimal `json:"pctFullyDiluted"` // incl. outstanding options
	DilutionDelta   decimal.Decimal `json:"dilutionDelta"`   // pctBefore - pctAfter

	ImpliedValueK   decimal.Decimal `json:"impliedValueK"`   // shares × price_per_share
	AmountInvestedK decimal.Decimal `json:"amountInvestedK"`
}

// CapTableMatrix — shareholder × phase ownership grid (for the consolidated table view).
type CapTableMatrix struct {
	ColumnLabels []string                  `json:"columnLabels"` // ["Création", "Phase 1", …]
	Rows         []CapTableMatrixRow       `json:"rows"`
	Totals       []CapTableMatrixTotalsRow `json:"totals"` // sum row per phase
}

// CapTableMatrixRow — one shareholder row across all phases.
type CapTableMatrixRow struct {
	ShareholderName string               `json:"shareholderName"`
	Type            ShareholderType      `json:"type"`
	Cells           []CapTableMatrixCell `json:"cells"` // one per phase
}

// CapTableMatrixCell — ownership snapshot at one phase for one shareholder.
type CapTableMatrixCell struct {
	Shares          int64           `json:"shares"`
	PctBasic        decimal.Decimal `json:"pctBasic"`
	PctFullyDiluted decimal.Decimal `json:"pctFullyDiluted"`
	ImpliedValueK   decimal.Decimal `json:"impliedValueK"`
}

// CapTableMatrixTotalsRow — aggregate totals for one phase column.
type CapTableMatrixTotalsRow struct {
	TotalShares          int64           `json:"totalShares"`
	TotalSharesPlusPools int64           `json:"totalSharesPlusPools"`
	TotalValueK          decimal.Decimal `json:"totalValueK"`
}

// DilutionWaterfall — tracks each holder's ownership % across all phases.
type DilutionWaterfall struct {
	Holders []DilutionWaterfallHolder `json:"holders"`
}

// DilutionWaterfallHolder — one holder's dilution trajectory.
type DilutionWaterfallHolder struct {
	Name          string            `json:"name"`
	Type          ShareholderType   `json:"type"`
	InitialPct    decimal.Decimal   `json:"initialPct"`
	PctByPhase    []decimal.Decimal `json:"pctByPhase"` // indexed by phase order
	FinalPct      decimal.Decimal   `json:"finalPct"`
	TotalDilution decimal.Decimal   `json:"totalDilution"` // initialPct - finalPct
}

// IngeFiCharts — chart data bundles for the IngéFi sub-module.
type IngeFiCharts struct {
	OwnershipStackedBar []ChartStackedBar `json:"ownershipStackedBar"` // stacked ownership per phase
	ValuationTimeline   []ChartDataPoint  `json:"valuationTimeline"`   // pre + post money per phase
	DilutionFunnel      []ChartDataPoint  `json:"dilutionFunnel"`      // founder % across phases
}

// ── 6.2 FastStockOption report ─────────────────────────────────────────────────

// StockOptionReport is the computed output of the stock option sub-module.
type StockOptionReport struct {
	Plans   []StockOptionPlanSummary `json:"plans"`
	ByPhase []StockOptionPhaseReport `json:"byPhase"`
	Charts  StockOptionCharts        `json:"charts"`
}

// StockOptionPlanSummary — lifecycle status for one plan.
type StockOptionPlanSummary struct {
	PlanID          uuid.UUID             `json:"planId"`
	PlanLabel       string                `json:"planLabel"`
	Instrument      StockOptionInstrument `json:"instrument"`
	ExercisePrice   decimal.Decimal       `json:"exercisePrice"`
	Voted           int64                 `json:"voted"`
	Attributed      int64                 `json:"attributed"`
	Exercised       int64                 `json:"exercised"`
	Cancelled       int64                 `json:"cancelled"`
	Reserve         int64                 `json:"reserve"`     // voted - attributed
	Outstanding     int64                 `json:"outstanding"` // attributed - exercised - cancelled
	IsEligibleBSPCE *bool                 `json:"isEligibleBspce,omitempty"`
}

// StockOptionPhaseReport — per-phase snapshot of all beneficiaries.
type StockOptionPhaseReport struct {
	PhaseNumber int                         `json:"phaseNumber"`
	Label       string                      `json:"label"`
	Founders    []StockOptionBeneficiaryRow `json:"founders"`
	Investors   []StockOptionBeneficiaryRow `json:"investors"`
	Totals      StockOptionBeneficiaryRow   `json:"totals"`
}

// StockOptionBeneficiaryRow — one row in the per-phase beneficiary breakdown.
type StockOptionBeneficiaryRow struct {
	Name             string          `json:"name"`
	SharesBefore     int64           `json:"sharesBefore"`
	OptionsGranted   int64           `json:"optionsGranted"`
	OptionsExercised int64           `json:"optionsExercised"`
	SharesAfter      int64           `json:"sharesAfter"`
	PctBasic         decimal.Decimal `json:"pctBasic"`        // %#1
	PctFullyDiluted  decimal.Decimal `json:"pctFullyDiluted"` // %#2
	ImpliedValueK    decimal.Decimal `json:"impliedValueK"`
	SharesPostSplit  int64           `json:"sharesPostSplit"` // adjusted by any split coefficient
}

// StockOptionCharts — chart data bundles for the FastStockOption sub-module.
type StockOptionCharts struct {
	PoolUtilisationGauge   []ChartGaugeEntry `json:"poolUtilisationGauge"`   // one per plan
	FullyDilutedComparison []ChartDataPoint  `json:"fullyDilutedComparison"` // basic vs FD per phase
}

// ── 6.3 FastValo report ────────────────────────────────────────────────────────

// FastValoReport is the computed output for all valuation scenarios.
type FastValoReport struct {
	Scenarios []ValuationScenarioResult `json:"scenarios"`
}

// ValuationScenarioResult — computed outputs for one FastValo scenario.
type ValuationScenarioResult struct {
	ScenarioID uuid.UUID             `json:"scenarioId"`
	CalcType   ValuationScenarioType `json:"calcType"`
	Label      string                `json:"label"`
	Language   string                `json:"language"`

	// Re-computed outputs
	IRRPct         *decimal.Decimal `json:"irrPct,omitempty"`
	TerminalValueK *decimal.Decimal `json:"terminalValueK,omitempty"`
	MoneyMultiple  *decimal.Decimal `json:"moneyMultiple,omitempty"`
	InvestorShareK *decimal.Decimal `json:"investorShareK,omitempty"`
	NPVK           *decimal.Decimal `json:"npvK,omitempty"`
	PreMoneyK      *decimal.Decimal `json:"preMoneyK,omitempty"`
	PostMoneyK     *decimal.Decimal `json:"postMoneyK,omitempty"`

	// Bilingual labels (fr|en)
	Labels map[string]string `json:"labels"`
}

// ── 6.4 Shared chart helpers ───────────────────────────────────────────────────

// ChartDataPoint is a single labelled numeric value for chart rendering.
type ChartDataPoint struct {
	Label string          `json:"label"`
	Value decimal.Decimal `json:"value"`
}

// ChartStackedBar is one stacked bar with multiple coloured segments.
type ChartStackedBar struct {
	Label    string       `json:"label"`
	Segments []ChartSlice `json:"segments"`
}

// ChartSlice is one segment of a stacked bar.
type ChartSlice struct {
	Label string          `json:"label"`
	Value decimal.Decimal `json:"value"`
	Color string          `json:"color"`
}

// ChartGaugeEntry is one gauge entry (e.g. option pool utilisation).
type ChartGaugeEntry struct {
	PlanLabel string          `json:"planLabel"`
	Total     int64           `json:"total"`
	Used      int64           `json:"used"`
	PctUsed   decimal.Decimal `json:"pctUsed"`
}
