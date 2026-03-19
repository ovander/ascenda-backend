# KerPlan — Multi-Country Specification (France & United States)

## 1. Design Philosophy

The original model was built for France (French PCG accounting, cotisations sociales, TVA,
calendar fiscal year). Generalizing to multi-country requires extracting **every country-specific
rule** into a pluggable **Country Tax Profile** system, while keeping the core computation
engine country-agnostic.

### Architecture Principle: Country as a Configuration Layer

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    COUNTRY-AGNOSTIC COMPUTATION ENGINE                       │
│                                                                              │
│  ComputeFullPlan(input FullPlanInput) FullPlanOutput                        │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  The engine uses abstract interfaces (TaxProfile, PayrollProfile,   │    │
│  │  DepreciationProfile, AccountingFormat) and NEVER contains          │    │
│  │  hardcoded rates, thresholds, or formatting rules.                  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│            ▲ implements                        ▲ implements                  │
│            │                                   │                             │
│  ┌─────────┴─────────┐              ┌──────────┴──────────┐                │
│  │  FranceTaxProfile  │              │   USTaxProfile      │                │
│  │  (25% IS, 45%      │              │   (21% Fed + State, │                │
│  │   charges, 20% TVA, │              │    ~8% FICA, Sales  │                │
│  │   PCG format)       │              │    Tax, GAAP format)│                │
│  └────────────────────┘              └─────────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Country Registry Model

### 2.1 Core Types

```go
// internal/model/country.go

package model

// CountryCode is the ISO 3166-1 alpha-2 code.
type CountryCode string

const (
    CountryFR CountryCode = "FR"
    CountryUS CountryCode = "US"
)

// Country holds the metadata for a supported country.
type Country struct {
    Code           CountryCode `gorm:"primaryKey;size:2" json:"code"`
    Name           string      `gorm:"size:100;not null" json:"name"`
    NameLocal      string      `gorm:"size:100" json:"nameLocal"`        // "France", "États-Unis"
    DefaultCurrency string    `gorm:"size:3;not null" json:"defaultCurrency"` // "EUR", "USD"
    FiscalYearRule FiscalYearRule `gorm:"size:20;not null" json:"fiscalYearRule"`
    AccountingStd  AccountingStandard `gorm:"size:20;not null" json:"accountingStd"`
    DefaultLocale  string     `gorm:"size:10;not null" json:"defaultLocale"`  // "fr-FR", "en-US"
    IsActive       bool       `gorm:"default:true" json:"isActive"`

    // Associations
    TaxProfiles      []CountryTaxProfile    `gorm:"foreignKey:CountryCode" json:"-"`
    PayrollProfile   CountryPayrollProfile  `gorm:"foreignKey:CountryCode" json:"-"`
    DeprecProfile    CountryDeprecProfile   `gorm:"foreignKey:CountryCode" json:"-"`
}

type FiscalYearRule string
const (
    FYCalendarOnly FiscalYearRule = "calendar_only"  // France: must be Jan-Dec
    FYFlexible     FiscalYearRule = "flexible"        // US: any 12-month period
)

type AccountingStandard string
const (
    AcctStdPCG  AccountingStandard = "PCG"    // French Plan Comptable Général
    AcctStdGAAP AccountingStandard = "GAAP"   // US Generally Accepted Accounting Principles
    AcctStdIFRS AccountingStandard = "IFRS"   // International (future)
)
```

### 2.2 PlanConfig Extension

The existing `PlanConfig` gains a `CountryCode` field. Every scenario is locked to a country.

```go
// internal/model/settings.go — additions

type PlanConfig struct {
    // ... existing fields ...

    // ── Country & Localization ──
    CountryCode    CountryCode `gorm:"size:2;not null;default:'FR'" json:"countryCode"`
    StateCode      string      `gorm:"size:10" json:"stateCode"`          // US only: "CA", "NY", "TX"
    Currency       string      `gorm:"size:3;not null;default:'EUR'" json:"currency"`
    Locale         string      `gorm:"size:10;not null;default:'fr-FR'" json:"locale"`

    // ── Tax Profile Selection ──
    // Points to the applicable CountryTaxProfile for this plan's fiscal year.
    // Allows historical plans to use prior-year rates.
    TaxProfileYear int `gorm:"not null" json:"taxProfileYear"` // e.g., 2025
}
```

---

## 3. Corporate Tax Profile

### 3.1 France: Impôt sur les Sociétés (IS)

```go
// internal/model/tax_corporate.go

// CountryTaxProfile stores corporate tax rules for a country + effective year.
type CountryTaxProfile struct {
    ID          uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
    CountryCode CountryCode `gorm:"size:2;not null;index" json:"countryCode"`
    EffectiveYear int       `gorm:"not null" json:"effectiveYear"` // 2025
    Label       string      `gorm:"size:100" json:"label"`         // "France IS 2025"

    // ── Corporate Tax Brackets ──
    // France: 2 brackets (SME 15% + standard 25%)
    // US: single federal (21%) + per-state rate
    Brackets []CorporateTaxBracket `gorm:"foreignKey:TaxProfileID;constraint:OnDelete:CASCADE" json:"brackets"`

    // ── Social Contribution on Profits (France-specific) ──
    SocialContribOnProfitsRate decimal.Decimal `gorm:"type:numeric(6,4)" json:"socialContribOnProfitsRate"`
    SocialContribThreshold     decimal.Decimal `gorm:"type:numeric(15,2)" json:"socialContribThreshold"`

    // ── Loss Carryforward Rules ──
    LossCarryForwardYears   int             `gorm:"default:-1" json:"lossCarryForwardYears"` // -1 = unlimited
    LossCarryForwardCap     decimal.Decimal `gorm:"type:numeric(15,2)" json:"lossCarryForwardCap"` // France: 1M€ + 50% above
    LossCarryForwardPctAbove decimal.Decimal `gorm:"type:numeric(6,4)" json:"lossCarryForwardPctAbove"` // 0.50

    // ── Tax Credits ──
    RnDTaxCreditAvailable bool            `gorm:"default:false" json:"rndTaxCreditAvailable"`
    RnDTaxCreditRate      decimal.Decimal `gorm:"type:numeric(6,4)" json:"rndTaxCreditRate"` // France CIR: 30%
    RnDTaxCreditCap       decimal.Decimal `gorm:"type:numeric(15,2)" json:"rndTaxCreditCap"` // France: 100M€

    CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

type CorporateTaxBracket struct {
    ID            uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
    TaxProfileID  uuid.UUID       `gorm:"type:uuid;not null" json:"taxProfileId"`
    SortOrder     int             `gorm:"not null" json:"sortOrder"`
    Label         string          `gorm:"size:50" json:"label"`       // "SME reduced", "Standard", "State"
    Rate          decimal.Decimal `gorm:"type:numeric(8,5);not null" json:"rate"` // 0.15, 0.25, 0.21
    ThresholdFrom decimal.Decimal `gorm:"type:numeric(15,2)" json:"thresholdFrom"` // 0
    ThresholdTo   decimal.Decimal `gorm:"type:numeric(15,2)" json:"thresholdTo"`   // 42500 (France SME)
    IsStateLevel  bool            `gorm:"default:false" json:"isStateLevel"`        // US state taxes
    StateCode     string          `gorm:"size:10" json:"stateCode"`                 // "CA", "NY"
}
```

### 3.2 Seed Data: France 2025

```go
var FranceTax2025 = CountryTaxProfile{
    CountryCode:   CountryFR,
    EffectiveYear: 2025,
    Label:         "France IS 2025",
    Brackets: []CorporateTaxBracket{
        {SortOrder: 1, Label: "Taux réduit PME", Rate: d("0.15"),
         ThresholdFrom: d("0"), ThresholdTo: d("42500")},
        {SortOrder: 2, Label: "Taux normal", Rate: d("0.25"),
         ThresholdFrom: d("42500"), ThresholdTo: d("999999999")},
    },
    SocialContribOnProfitsRate: d("0.033"),  // 3.3% contribution sociale
    SocialContribThreshold:     d("763000"), // Turnover > 763k€
    LossCarryForwardYears:      -1,          // Unlimited
    LossCarryForwardCap:        d("1000000"),// 1M€ fully deductible
    LossCarryForwardPctAbove:   d("0.50"),   // 50% above 1M€
    RnDTaxCreditAvailable:      true,
    RnDTaxCreditRate:           d("0.30"),   // CIR 30%
    RnDTaxCreditCap:            d("100000000"), // 100M€
}
```

### 3.3 Seed Data: United States 2025

```go
var USTax2025 = CountryTaxProfile{
    CountryCode:   CountryUS,
    EffectiveYear: 2025,
    Label:         "US Federal + State 2025",
    Brackets: []CorporateTaxBracket{
        // Federal: flat 21%
        {SortOrder: 1, Label: "Federal", Rate: d("0.21"),
         ThresholdFrom: d("0"), ThresholdTo: d("999999999"),
         IsStateLevel: false},

        // State examples (user selects their state):
        {SortOrder: 2, Label: "California", Rate: d("0.084"),
         ThresholdFrom: d("0"), ThresholdTo: d("999999999"),
         IsStateLevel: true, StateCode: "CA"},
        {SortOrder: 3, Label: "New York", Rate: d("0.065"),
         ThresholdFrom: d("0"), ThresholdTo: d("999999999"),
         IsStateLevel: true, StateCode: "NY"},
        {SortOrder: 4, Label: "Texas", Rate: d("0.00"),
         ThresholdFrom: d("0"), ThresholdTo: d("999999999"),
         IsStateLevel: true, StateCode: "TX"},
        {SortOrder: 5, Label: "Delaware", Rate: d("0.087"),
         ThresholdFrom: d("0"), ThresholdTo: d("999999999"),
         IsStateLevel: true, StateCode: "DE"},
        // ... all 50 states + DC seeded ...
    },
    SocialContribOnProfitsRate: d("0"),    // No equivalent
    SocialContribThreshold:     d("0"),
    LossCarryForwardYears:      -1,        // Unlimited (post-TCJA)
    LossCarryForwardCap:        d("0"),    // No fixed cap
    LossCarryForwardPctAbove:   d("0.80"), // 80% of taxable income
    RnDTaxCreditAvailable:      true,
    RnDTaxCreditRate:           d("0.20"), // Federal R&D credit ~20%
    RnDTaxCreditCap:            d("0"),    // No cap
}
```

### 3.4 Tax Computation Engine

```go
// internal/compute/tax.go

// TaxEngine computes corporate tax based on the country profile.
// Replaces the hardcoded France IS computation.
type TaxEngine struct {
    profile CountryTaxProfile
    state   string // US only: which state
}

func NewTaxEngine(profile CountryTaxProfile, stateCode string) *TaxEngine {
    return &TaxEngine{profile: profile, state: stateCode}
}

// ComputeCorporateTax calculates tax for a given year's taxable income.
func (e *TaxEngine) ComputeCorporateTax(
    taxableIncome decimal.Decimal,
    isSmallBusiness bool,       // France: turnover ≤ 10M€
    turnover decimal.Decimal,   // For social contribution threshold
    rndExpenses decimal.Decimal, // For R&D tax credit
    lossCarryForward decimal.Decimal, // Accumulated prior losses
) CorporateTaxResult {
    var result CorporateTaxResult

    // ── Step 1: Apply loss carryforward ──
    deductibleLoss := e.computeDeductibleLoss(taxableIncome, lossCarryForward)
    adjustedIncome := taxableIncome.Sub(deductibleLoss)
    result.LossUsed = deductibleLoss
    result.RemainingLoss = lossCarryForward.Sub(deductibleLoss)

    if adjustedIncome.IsNegative() {
        adjustedIncome = decimal.Zero
    }

    // ── Step 2: Apply bracket tax rates ──
    var totalTax decimal.Decimal
    remaining := adjustedIncome

    for _, bracket := range e.getApplicableBrackets(isSmallBusiness) {
        bracketWidth := bracket.ThresholdTo.Sub(bracket.ThresholdFrom)
        taxableInBracket := decimal.Min(remaining, bracketWidth)
        if taxableInBracket.IsPositive() {
            tax := taxableInBracket.Mul(bracket.Rate)
            totalTax = totalTax.Add(tax)
            result.BracketDetail = append(result.BracketDetail, TaxBracketDetail{
                Label:   bracket.Label,
                Rate:    bracket.Rate,
                Taxable: taxableInBracket,
                Tax:     tax,
            })
            remaining = remaining.Sub(taxableInBracket)
        }
        if remaining.IsZero() {
            break
        }
    }

    // ── Step 3: State tax (US only) ──
    if e.state != "" {
        for _, bracket := range e.profile.Brackets {
            if bracket.IsStateLevel && bracket.StateCode == e.state {
                stateTax := adjustedIncome.Mul(bracket.Rate)
                totalTax = totalTax.Add(stateTax)
                result.StateTax = stateTax
                result.StateCode = e.state
                result.StateRate = bracket.Rate
                break
            }
        }
    }

    // ── Step 4: Social contribution on profits (France) ──
    if e.profile.SocialContribOnProfitsRate.IsPositive() &&
       turnover.GreaterThan(e.profile.SocialContribThreshold) {
        socialContrib := totalTax.Mul(e.profile.SocialContribOnProfitsRate)
        totalTax = totalTax.Add(socialContrib)
        result.SocialContrib = socialContrib
    }

    // ── Step 5: R&D tax credit ──
    if e.profile.RnDTaxCreditAvailable && rndExpenses.IsPositive() {
        credit := rndExpenses.Mul(e.profile.RnDTaxCreditRate)
        if e.profile.RnDTaxCreditCap.IsPositive() {
            credit = decimal.Min(credit, e.profile.RnDTaxCreditCap)
        }
        totalTax = totalTax.Sub(credit)
        if totalTax.IsNegative() {
            result.TaxCreditCarryForward = totalTax.Abs()
            totalTax = decimal.Zero
        }
        result.RnDCredit = credit
    }

    result.GrossTax = totalTax
    result.EffectiveRate = decimal.Zero
    if adjustedIncome.IsPositive() {
        result.EffectiveRate = totalTax.Div(adjustedIncome)
    }

    return result
}

type CorporateTaxResult struct {
    GrossTax              decimal.Decimal     `json:"grossTax"`
    EffectiveRate         decimal.Decimal     `json:"effectiveRate"`
    BracketDetail         []TaxBracketDetail  `json:"bracketDetail"`
    StateTax              decimal.Decimal     `json:"stateTax"`      // US only
    StateCode             string              `json:"stateCode"`
    StateRate             decimal.Decimal     `json:"stateRate"`
    SocialContrib         decimal.Decimal     `json:"socialContrib"` // France only
    RnDCredit             decimal.Decimal     `json:"rndCredit"`
    TaxCreditCarryForward decimal.Decimal     `json:"taxCreditCarryForward"`
    LossUsed              decimal.Decimal     `json:"lossUsed"`
    RemainingLoss         decimal.Decimal     `json:"remainingLoss"`
}

type TaxBracketDetail struct {
    Label   string          `json:"label"`
    Rate    decimal.Decimal `json:"rate"`
    Taxable decimal.Decimal `json:"taxable"`
    Tax     decimal.Decimal `json:"tax"`
}
```

---

## 4. Payroll & Social Charges Profile

This is where France and the US differ most dramatically: France ~45% employer charges
vs US ~8–10%.

### 4.1 Model

```go
// internal/model/tax_payroll.go

// CountryPayrollProfile defines employer/employee tax structure for a country.
type CountryPayrollProfile struct {
    ID            uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
    CountryCode   CountryCode `gorm:"size:2;not null;uniqueIndex" json:"countryCode"`
    EffectiveYear int         `gorm:"not null" json:"effectiveYear"`
    Label         string      `gorm:"size:100" json:"label"`

    // ── Employer Charges ──
    EmployerCharges []PayrollChargeLine `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE" json:"employerCharges"`

    // ── Employee Deductions ──
    EmployeeDeductions []PayrollChargeLine `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE" json:"employeeDeductions"`

    // ── Salary Structure ──
    SalaryMonthsDefault int `gorm:"default:12" json:"salaryMonthsDefault"` // FR: 12 or 13, US: 12
    MinimumWageMonthly decimal.Decimal `gorm:"type:numeric(10,2)" json:"minimumWageMonthly"` // SMIC / federal min

    // ── Overtime Rules ──
    StandardWeeklyHours decimal.Decimal `gorm:"type:numeric(5,2);default:35" json:"standardWeeklyHours"` // FR: 35h, US: 40h
    OvertimeMultiplier  decimal.Decimal `gorm:"type:numeric(4,2);default:1.25" json:"overtimeMultiplier"`

    CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// PayrollChargeLine represents one line of employer or employee charges.
type PayrollChargeLine struct {
    ID          uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
    ProfileID   uuid.UUID       `gorm:"type:uuid;not null" json:"profileId"`
    Side        PayrollSide     `gorm:"size:10;not null" json:"side"` // "employer" | "employee"
    Category    string          `gorm:"size:50;not null" json:"category"`
    Label       string          `gorm:"size:100;not null" json:"label"`
    Rate        decimal.Decimal `gorm:"type:numeric(8,5);not null" json:"rate"`
    CeilingType CeilingType     `gorm:"size:20" json:"ceilingType"`
    CeilingAmount decimal.Decimal `gorm:"type:numeric(15,2)" json:"ceilingAmount"` // Annual cap
    IsStateLevel bool           `gorm:"default:false" json:"isStateLevel"`
    StateCode    string          `gorm:"size:10" json:"stateCode"`
    Notes       string          `gorm:"size:255" json:"notes"`
}

type PayrollSide string
const (
    PayrollEmployer PayrollSide = "employer"
    PayrollEmployee PayrollSide = "employee"
)

type CeilingType string
const (
    CeilingNone      CeilingType = "none"       // Rate applies on full salary
    CeilingAbsolute  CeilingType = "absolute"    // Rate capped at fixed amount
    CeilingMultiple  CeilingType = "multiple_ss" // Rate capped at N × social security ceiling
)
```

### 4.2 France Payroll Seed (2025)

```go
var FrancePayroll2025 = CountryPayrollProfile{
    CountryCode:   CountryFR,
    EffectiveYear: 2025,
    Label:         "France Charges Sociales 2025",
    SalaryMonthsDefault: 12,
    MinimumWageMonthly:  d("1766.92"), // SMIC 2025
    StandardWeeklyHours: d("35"),
    OvertimeMultiplier:  d("1.25"),
    EmployerCharges: []PayrollChargeLine{
        // ── Sécurité Sociale ──
        {Side: "employer", Category: "securite_sociale", Label: "Assurance maladie",
         Rate: d("0.1300"), CeilingType: CeilingNone},
        {Side: "employer", Category: "securite_sociale", Label: "Assurance vieillesse (plafonnée)",
         Rate: d("0.0855"), CeilingType: CeilingMultiple, CeilingAmount: d("47100")}, // 1× PASS
        {Side: "employer", Category: "securite_sociale", Label: "Assurance vieillesse (déplafonnée)",
         Rate: d("0.0200"), CeilingType: CeilingNone},
        {Side: "employer", Category: "securite_sociale", Label: "Allocations familiales",
         Rate: d("0.0525"), CeilingType: CeilingNone}, // 5.25% standard, 3.45% if ≤3.5 SMIC
        {Side: "employer", Category: "securite_sociale", Label: "Accidents du travail",
         Rate: d("0.0200"), CeilingType: CeilingNone, Notes: "Rate varies by sector (1-7%)"},

        // ── Chômage ──
        {Side: "employer", Category: "chomage", Label: "Assurance chômage",
         Rate: d("0.0405"), CeilingType: CeilingMultiple, CeilingAmount: d("188400")}, // 4× PASS
        {Side: "employer", Category: "chomage", Label: "AGS",
         Rate: d("0.0015"), CeilingType: CeilingMultiple, CeilingAmount: d("188400")},

        // ── Retraite complémentaire (AGIRC-ARRCO) ──
        {Side: "employer", Category: "retraite_compl", Label: "Tranche 1 (≤PASS)",
         Rate: d("0.0472"), CeilingType: CeilingMultiple, CeilingAmount: d("47100")},
        {Side: "employer", Category: "retraite_compl", Label: "Tranche 2 (>PASS)",
         Rate: d("0.1286"), CeilingType: CeilingMultiple, CeilingAmount: d("376800")}, // 8× PASS

        // ── Autres ──
        {Side: "employer", Category: "autres", Label: "CSA (contribution solidarité autonomie)",
         Rate: d("0.0030"), CeilingType: CeilingNone},
        {Side: "employer", Category: "autres", Label: "FNAL",
         Rate: d("0.0050"), CeilingType: CeilingNone}, // ≥50 employees: 0.50%
        {Side: "employer", Category: "autres", Label: "Formation professionnelle",
         Rate: d("0.0100"), CeilingType: CeilingNone}, // 1% for ≥11 employees
        {Side: "employer", Category: "autres", Label: "Taxe d'apprentissage",
         Rate: d("0.0068"), CeilingType: CeilingNone},
        {Side: "employer", Category: "autres", Label: "Effort construction",
         Rate: d("0.0045"), CeilingType: CeilingNone}, // ≥50 employees
        {Side: "employer", Category: "autres", Label: "Transport (Versement Mobilité)",
         Rate: d("0.0200"), CeilingType: CeilingNone, Notes: "Rate varies by commune (0-3%)"},

        // ── Prévoyance & Mutuelle ──
        {Side: "employer", Category: "prevoyance", Label: "Mutuelle obligatoire (part employeur)",
         Rate: d("0.0100"), CeilingType: CeilingNone, Notes: "Min 50% of base contract"},
        {Side: "employer", Category: "prevoyance", Label: "Prévoyance cadres",
         Rate: d("0.0150"), CeilingType: CeilingMultiple, CeilingAmount: d("47100")},
    },
    // Approximate total employer rate: ~42-47% depending on salary level and sector

    EmployeeDeductions: []PayrollChargeLine{
        {Side: "employee", Category: "securite_sociale", Label: "Assurance vieillesse (plafonnée)",
         Rate: d("0.0690"), CeilingType: CeilingMultiple, CeilingAmount: d("47100")},
        {Side: "employee", Category: "securite_sociale", Label: "Assurance vieillesse (déplafonnée)",
         Rate: d("0.0040"), CeilingType: CeilingNone},
        {Side: "employee", Category: "csg_crds", Label: "CSG déductible",
         Rate: d("0.0680"), CeilingType: CeilingNone, Notes: "Applied on 98.25% of gross"},
        {Side: "employee", Category: "csg_crds", Label: "CSG non-déductible",
         Rate: d("0.0240"), CeilingType: CeilingNone},
        {Side: "employee", Category: "csg_crds", Label: "CRDS",
         Rate: d("0.0050"), CeilingType: CeilingNone},
        {Side: "employee", Category: "retraite_compl", Label: "AGIRC-ARRCO T1",
         Rate: d("0.0315"), CeilingType: CeilingMultiple, CeilingAmount: d("47100")},
        {Side: "employee", Category: "retraite_compl", Label: "AGIRC-ARRCO T2",
         Rate: d("0.0857"), CeilingType: CeilingMultiple, CeilingAmount: d("376800")},
    },
    // Approximate total employee rate: ~22-23%
}
```

### 4.3 US Payroll Seed (2025)

```go
var USPayroll2025 = CountryPayrollProfile{
    CountryCode:   CountryUS,
    EffectiveYear: 2025,
    Label:         "US Payroll Taxes 2025",
    SalaryMonthsDefault: 12,
    MinimumWageMonthly:  d("1256.67"), // Federal $7.25/hr × 40h × 4.33 weeks
    StandardWeeklyHours: d("40"),
    OvertimeMultiplier:  d("1.50"),    // US: 1.5× (vs France 1.25×)
    EmployerCharges: []PayrollChargeLine{
        // ── FICA ──
        {Side: "employer", Category: "fica", Label: "Social Security (OASDI)",
         Rate: d("0.0620"), CeilingType: CeilingAbsolute, CeilingAmount: d("176100")},
         // 2025 wage base. 2026: $184,500
        {Side: "employer", Category: "fica", Label: "Medicare",
         Rate: d("0.0145"), CeilingType: CeilingNone}, // No cap

        // ── Federal Unemployment ──
        {Side: "employer", Category: "futa", Label: "FUTA",
         Rate: d("0.0060"), CeilingType: CeilingAbsolute, CeilingAmount: d("7000"),
         Notes: "6.0% nominal, 5.4% credit = 0.6% effective on first $7,000"},

        // ── State Unemployment (examples — varies by state + experience rating) ──
        {Side: "employer", Category: "suta", Label: "California SUTA",
         Rate: d("0.034"), CeilingType: CeilingAbsolute, CeilingAmount: d("7000"),
         IsStateLevel: true, StateCode: "CA", Notes: "New employer rate 3.4%"},
        {Side: "employer", Category: "suta", Label: "New York SUTA",
         Rate: d("0.041"), CeilingType: CeilingAbsolute, CeilingAmount: d("12800"),
         IsStateLevel: true, StateCode: "NY"},
        {Side: "employer", Category: "suta", Label: "Texas SUTA",
         Rate: d("0.027"), CeilingType: CeilingAbsolute, CeilingAmount: d("9000"),
         IsStateLevel: true, StateCode: "TX"},

        // ── Workers' Compensation (state-mandated, industry-variable) ──
        {Side: "employer", Category: "workers_comp", Label: "Workers' Compensation",
         Rate: d("0.0100"), CeilingType: CeilingNone,
         Notes: "Varies by state and industry class (0.5-5%)"},
    },
    // Approximate total employer rate: ~8-12% depending on state + industry

    EmployeeDeductions: []PayrollChargeLine{
        {Side: "employee", Category: "fica", Label: "Social Security (OASDI)",
         Rate: d("0.0620"), CeilingType: CeilingAbsolute, CeilingAmount: d("176100")},
        {Side: "employee", Category: "fica", Label: "Medicare",
         Rate: d("0.0145"), CeilingType: CeilingNone},
        {Side: "employee", Category: "fica", Label: "Additional Medicare (>$200k)",
         Rate: d("0.0090"), CeilingType: CeilingNone,
         Notes: "Only on wages exceeding $200,000"},
    },
    // Approximate total employee rate: ~7.65% (before income tax withholding)
}
```

### 4.4 Payroll Computation Engine

```go
// internal/compute/payroll_tax.go

// PayrollTaxEngine computes employer and employee charges per country.
type PayrollTaxEngine struct {
    profile   CountryPayrollProfile
    stateCode string
}

func NewPayrollTaxEngine(profile CountryPayrollProfile, stateCode string) *PayrollTaxEngine {
    return &PayrollTaxEngine{profile: profile, stateCode: stateCode}
}

// ComputeEmployerCharges returns total employer charges for a given annual gross salary.
func (e *PayrollTaxEngine) ComputeEmployerCharges(
    annualGross decimal.Decimal,
) PayrollChargesResult {
    var result PayrollChargesResult
    result.GrossSalary = annualGross

    for _, charge := range e.profile.EmployerCharges {
        // Skip state-level charges that don't match selected state
        if charge.IsStateLevel && charge.StateCode != e.stateCode {
            continue
        }

        taxableBase := e.computeTaxableBase(annualGross, charge)
        amount := taxableBase.Mul(charge.Rate)

        result.Lines = append(result.Lines, PayrollChargeResult{
            Category: charge.Category,
            Label:    charge.Label,
            Rate:     charge.Rate,
            Base:     taxableBase,
            Amount:   amount,
        })
        result.TotalEmployerCharges = result.TotalEmployerCharges.Add(amount)
    }

    result.TotalCost = annualGross.Add(result.TotalEmployerCharges)
    result.EffectiveRate = decimal.Zero
    if annualGross.IsPositive() {
        result.EffectiveRate = result.TotalEmployerCharges.Div(annualGross)
    }

    return result
}

// computeTaxableBase applies ceiling rules.
func (e *PayrollTaxEngine) computeTaxableBase(
    annualGross decimal.Decimal, charge PayrollChargeLine,
) decimal.Decimal {
    switch charge.CeilingType {
    case CeilingAbsolute:
        return decimal.Min(annualGross, charge.CeilingAmount)
    case CeilingMultiple:
        return decimal.Min(annualGross, charge.CeilingAmount)
    default:
        return annualGross
    }
}

type PayrollChargesResult struct {
    GrossSalary          decimal.Decimal       `json:"grossSalary"`
    TotalEmployerCharges decimal.Decimal       `json:"totalEmployerCharges"`
    TotalCost            decimal.Decimal       `json:"totalCost"` // Gross + charges
    EffectiveRate        decimal.Decimal       `json:"effectiveRate"`
    Lines                []PayrollChargeResult  `json:"lines"`
}

type PayrollChargeResult struct {
    Category string          `json:"category"`
    Label    string          `json:"label"`
    Rate     decimal.Decimal `json:"rate"`
    Base     decimal.Decimal `json:"base"`
    Amount   decimal.Decimal `json:"amount"`
}
```

### 4.5 Impact on PlanConfig: Replacing Hardcoded EmployerTaxRate

The existing `PlanConfig.EmployerTaxRate` (a single percentage) is **replaced** by the
`PayrollTaxEngine` for country-aware computation. The single rate is kept as a simplified
**user override** when the user wants to use a flat approximation:

```go
// PlanConfig changes:
type PlanConfig struct {
    // ... existing fields ...

    // BEFORE: EmployerTaxRate decimal.Decimal (single rate, ~42% for France)
    // AFTER:
    UseDetailedPayrollTax bool            `json:"useDetailedPayrollTax"` // true = country engine
    EmployerTaxOverride   decimal.Decimal `json:"employerTaxOverride"`   // Flat rate fallback
    // If UseDetailedPayrollTax=true → PayrollTaxEngine computes per-line charges
    // If UseDetailedPayrollTax=false → use EmployerTaxOverride as simple multiplier
}
```

---

## 5. VAT / Sales Tax Profile

### 5.1 Model

```go
// internal/model/tax_vat.go

// CountryVATProfile defines indirect tax structure (VAT or Sales Tax).
type CountryVATProfile struct {
    ID            uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
    CountryCode   CountryCode `gorm:"size:2;not null" json:"countryCode"`
    EffectiveYear int         `gorm:"not null" json:"effectiveYear"`
    TaxSystem     IndirectTaxSystem `gorm:"size:20;not null" json:"taxSystem"`

    // ── VAT Rates (France / EU) ──
    StandardRate   decimal.Decimal `gorm:"type:numeric(6,4)" json:"standardRate"`   // FR: 0.20
    ReducedRates   []VATReducedRate `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE" json:"reducedRates"`

    // ── Sales Tax (US) ──
    StateSalesTaxRates []StateSalesTax `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE" json:"stateSalesTaxRates"`
    SaaSIsTaxable      bool           `gorm:"default:false" json:"saasIsTaxable"` // US: varies by state

    // ── Common ──
    IncludedInPrice    bool `gorm:"default:true" json:"includedInPrice"`  // FR: true (TTC), US: false
    VATOnExports       bool `gorm:"default:false" json:"vatOnExports"`    // Usually 0% on exports

    CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

type IndirectTaxSystem string
const (
    TaxSystemVAT      IndirectTaxSystem = "vat"       // France, EU
    TaxSystemSalesTax IndirectTaxSystem = "sales_tax"  // US
    TaxSystemGST      IndirectTaxSystem = "gst"        // Future: Australia, India
)

type VATReducedRate struct {
    ID        uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
    ProfileID uuid.UUID       `gorm:"type:uuid;not null" json:"profileId"`
    Rate      decimal.Decimal `gorm:"type:numeric(6,4);not null" json:"rate"`
    Label     string          `gorm:"size:100" json:"label"` // "Taux réduit", "Taux intermédiaire"
    AppliesTo string          `gorm:"size:255" json:"appliesTo"` // Description of applicable goods
}

type StateSalesTax struct {
    ID          uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
    ProfileID   uuid.UUID       `gorm:"type:uuid;not null" json:"profileId"`
    StateCode   string          `gorm:"size:10;not null" json:"stateCode"`
    StateName   string          `gorm:"size:50" json:"stateName"`
    StateRate   decimal.Decimal `gorm:"type:numeric(6,4)" json:"stateRate"`
    AvgLocalRate decimal.Decimal `gorm:"type:numeric(6,4)" json:"avgLocalRate"` // Average local tax
    CombinedRate decimal.Decimal `gorm:"type:numeric(6,4)" json:"combinedRate"`
    SaaSExempt  bool            `gorm:"default:false" json:"saasExempt"` // True if SaaS not taxed
}
```

### 5.2 Seed Data

```go
var FranceVAT2025 = CountryVATProfile{
    CountryCode:  CountryFR,
    EffectiveYear: 2025,
    TaxSystem:    TaxSystemVAT,
    StandardRate: d("0.20"),
    ReducedRates: []VATReducedRate{
        {Rate: d("0.10"), Label: "Taux intermédiaire", AppliesTo: "Transport, restauration, hôtellerie"},
        {Rate: d("0.055"), Label: "Taux réduit", AppliesTo: "Alimentation, énergie, livres"},
        {Rate: d("0.021"), Label: "Taux super-réduit", AppliesTo: "Presse, médicaments remboursables"},
    },
    IncludedInPrice: true,
    VATOnExports: false,
}

var USVAT2025 = CountryVATProfile{
    CountryCode:  CountryUS,
    EffectiveYear: 2025,
    TaxSystem:    TaxSystemSalesTax,
    StandardRate: d("0"), // No federal sales tax
    StateSalesTaxRates: []StateSalesTax{
        {StateCode: "CA", StateName: "California", StateRate: d("0.0725"), AvgLocalRate: d("0.0158"), CombinedRate: d("0.0883")},
        {StateCode: "NY", StateName: "New York", StateRate: d("0.0400"), AvgLocalRate: d("0.0453"), CombinedRate: d("0.0853")},
        {StateCode: "TX", StateName: "Texas", StateRate: d("0.0625"), AvgLocalRate: d("0.0200"), CombinedRate: d("0.0825")},
        {StateCode: "FL", StateName: "Florida", StateRate: d("0.0600"), AvgLocalRate: d("0.0102"), CombinedRate: d("0.0702")},
        {StateCode: "WA", StateName: "Washington", StateRate: d("0.0650"), AvgLocalRate: d("0.0283"), CombinedRate: d("0.0933")},
        {StateCode: "DE", StateName: "Delaware", StateRate: d("0"), AvgLocalRate: d("0"), CombinedRate: d("0")},
        {StateCode: "OR", StateName: "Oregon", StateRate: d("0"), AvgLocalRate: d("0"), CombinedRate: d("0")},
        // ... all 50 states + DC ...
    },
    IncludedInPrice: false, // US: price + tax at checkout
    SaaSIsTaxable: true,    // Varies: ~30 states tax SaaS, ~20 exempt
}
```

### 5.3 Impact on Computation

```go
// internal/compute/vat.go

// VATEngine replaces hardcoded VAT rate throughout the model.
type VATEngine struct {
    profile CountryVATProfile
    state   string
}

// GetEffectiveRate returns the applicable indirect tax rate.
func (e *VATEngine) GetEffectiveRate() decimal.Decimal {
    switch e.profile.TaxSystem {
    case TaxSystemVAT:
        return e.profile.StandardRate // 20% for France
    case TaxSystemSalesTax:
        if e.state != "" {
            for _, st := range e.profile.StateSalesTaxRates {
                if st.StateCode == e.state {
                    return st.CombinedRate
                }
            }
        }
        return decimal.Zero // No state selected or no sales tax
    default:
        return decimal.Zero
    }
}

// IsPriceInclusive returns whether displayed prices include tax.
func (e *VATEngine) IsPriceInclusive() bool {
    return e.profile.IncludedInPrice // France: true, US: false
}
```

---

## 6. Depreciation Profile

### 6.1 Model

```go
// internal/model/tax_depreciation.go

type CountryDeprecProfile struct {
    ID            uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
    CountryCode   CountryCode `gorm:"size:2;not null;uniqueIndex" json:"countryCode"`
    EffectiveYear int         `gorm:"not null" json:"effectiveYear"`
    Label         string      `gorm:"size:100" json:"label"`

    DefaultMethod     DeprecMethod `gorm:"size:20;not null" json:"defaultMethod"`
    AllowedMethods    []DeprecMethod `gorm:"-" json:"allowedMethods"` // Serialized as JSON

    // US-specific: Section 179 / Bonus Depreciation
    Section179Available bool            `gorm:"default:false" json:"section179Available"`
    Section179Limit     decimal.Decimal `gorm:"type:numeric(15,2)" json:"section179Limit"`     // $2.5M (2025)
    Section179PhaseOut  decimal.Decimal `gorm:"type:numeric(15,2)" json:"section179PhaseOut"`   // $4.0M
    BonusDeprecRate     decimal.Decimal `gorm:"type:numeric(6,4)" json:"bonusDeprecRate"`       // 0.40 (2025)

    // Standard useful lives per asset category
    UsefulLives []DeprecUsefulLife `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE" json:"usefulLives"`

    CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

type DeprecMethod string
const (
    DeprecStraightLine DeprecMethod = "straight_line" // Linear / linéaire
    DeprecDeclining    DeprecMethod = "declining"      // Dégressif / DB
    DeprecMACRS        DeprecMethod = "macrs"           // US Modified Accelerated Cost Recovery
    DeprecSection179   DeprecMethod = "section_179"     // US immediate expensing
)

type DeprecUsefulLife struct {
    ID            uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
    ProfileID     uuid.UUID       `gorm:"type:uuid;not null" json:"profileId"`
    AssetCategory string          `gorm:"size:50;not null" json:"assetCategory"` // Maps to CapexCategory
    DefaultYears  int             `gorm:"not null" json:"defaultYears"`
    MACRSClass    int             `gorm:"" json:"macrsClass"` // US: 3, 5, 7, 15, 27.5, 39 years
    Notes         string          `gorm:"size:255" json:"notes"`
}
```

### 6.2 Comparison Table

```
┌─────────────────────┬───────────────────────────────┬───────────────────────────────┐
│ Asset Category      │ France (SLN)                  │ US (MACRS + Bonus)            │
├─────────────────────┼───────────────────────────────┼───────────────────────────────┤
│ Land                │ Not depreciable               │ Not depreciable               │
│ Buildings           │ 20–50 years SLN               │ 39 years (non-residential)    │
│ Set-up / Leasehold  │ 5–10 years SLN                │ 15 years MACRS                │
│ Patents / IP        │ Legal life or 5 years SLN     │ 15 years MACRS (or S179)      │
│ R&D (capitalized)   │ 3–5 years SLN                 │ 5 years MACRS (or S179)       │
│ Software            │ 1–5 years SLN                 │ 3 years MACRS (or S179)       │
│ Prototypes          │ 3–5 years SLN                 │ 5 years MACRS (or S179)       │
│ Equipment           │ 5–10 years SLN                │ 5–7 years MACRS               │
│ Furniture           │ 5–10 years SLN                │ 7 years MACRS                 │
│ Computers/IT        │ 3–5 years SLN                 │ 5 years MACRS (or S179)       │
│ Vehicles            │ 4–5 years SLN                 │ 5 years MACRS (limits apply)  │
│ Financial assets    │ Not depreciable (impairment)  │ Not depreciable               │
└─────────────────────┴───────────────────────────────┴───────────────────────────────┘

Key US accelerators:
  Section 179: Immediate 100% expensing up to $2.5M (2025)
  Bonus Depreciation: 40% first-year deduction (2025), phasing down 20%/year
  MACRS: Accelerated depreciation schedule (front-loaded)

Key France rules:
  Straight-line (linéaire) is the standard
  Declining-balance (dégressif) no longer available for new assets post-2020
  No equivalent of Section 179 immediate expensing
  Component approach: each major component depreciated separately
```

---

## 7. P&L Format: French PCG vs US GAAP

### 7.1 Format Selector

The P&L computation already produces both French (7 aggregates) and Anglo-Saxon (P&L+Cash)
formats. Multi-country extends this to make the **primary format** country-dependent:

```go
// internal/model/pnl_format.go

type PnLFormat string
const (
    PnLFormatFrenchPCG PnLFormat = "french_pcg"  // Charges par nature
    PnLFormatUSGAAP    PnLFormat = "us_gaap"      // Functional expense
    PnLFormatIFRS      PnLFormat = "ifrs"          // Future
)

// Country → default P&L format:
var DefaultPnLFormat = map[CountryCode]PnLFormat{
    CountryFR: PnLFormatFrenchPCG,
    CountryUS: PnLFormatUSGAAP,
}
```

### 7.2 Side-by-Side: French PCG vs US GAAP

```
┌─ FRENCH PCG (par nature) ──────────┐  ┌─ US GAAP (functional) ─────────────┐
│                                     │  │                                     │
│  Ventes de marchandises             │  │  Revenue                            │
│  Production vendue                  │  │  - Cost of Goods Sold (COGS)        │
│  ─────────────────                  │  │  ─────────────────                  │
│  A. Chiffre d'affaires              │  │  Gross Profit                       │
│  - Achats consommés                 │  │                                     │
│  - Variation stocks                 │  │  Operating Expenses:                │
│  ─────────────────                  │  │  - Research & Development           │
│  B. Consommation de l'exercice      │  │  - Sales & Marketing               │
│  ─────────────────                  │  │  - General & Administrative         │
│  C. Valeur Ajoutée (A-B)            │  │  - Depreciation & Amortization     │
│  - Impôts et taxes                  │  │  ─────────────────                  │
│  - Charges de personnel             │  │  Total Operating Expenses           │
│  ─────────────────                  │  │  ─────────────────                  │
│  D. EBE / EBITDA (C-taxes-payroll)  │  │  Operating Income (EBIT)            │
│  - Dotations amortissements         │  │                                     │
│  + Reprises, transferts             │  │  Other Income / (Expense):          │
│  ─────────────────                  │  │  - Interest Income                  │
│  E. Résultat d'exploitation (EBIT)  │  │  - Interest Expense                │
│  + Produits financiers              │  │  - Other, net                       │
│  - Charges financières              │  │  ─────────────────                  │
│  ─────────────────                  │  │  Income Before Tax                  │
│  F. Résultat courant avant impôt    │  │  - Income Tax Provision             │
│  +/- Résultat exceptionnel          │  │  ─────────────────                  │
│  - Impôt sur les sociétés           │  │  Net Income                         │
│  ─────────────────                  │  │                                     │
│  G. Résultat net                    │  │                                     │
└─────────────────────────────────────┘  └─────────────────────────────────────┘
```

### 7.3 Computation Mapping

The existing `ComputePnl()` (French) and `ComputePnlCash()` (Anglo-Saxon) remain as-is.
For US plans, the **primary** P&L is the functional format (P&L+Cash), and the French PCG
format becomes secondary / optional.

```go
// In ComputeFullPlan:
switch input.Config.CountryCode {
case CountryFR:
    // Primary: French PCG → ComputePnl()
    // Secondary: Functional → ComputePnlCash()
    out.PrimaryPnL = "french_pcg"
case CountryUS:
    // Primary: Functional → ComputePnlCash()
    // Secondary: French PCG → ComputePnl() (optional, for comparison)
    out.PrimaryPnL = "us_gaap"
}
// Both are always computed; the frontend shows the primary with a toggle for secondary.
```

---

## 8. Localization: Number & Currency Formatting

### 8.1 Server-Side: Raw Decimal Values

The backend **always returns raw decimal values**. Formatting is a frontend concern.
However, the backend provides the locale metadata needed for correct display:

```go
// Included in every API response via PlanConfigComputed:
type LocaleInfo struct {
    CountryCode     CountryCode `json:"countryCode"`
    Locale          string      `json:"locale"`          // "fr-FR", "en-US"
    CurrencyCode    string      `json:"currencyCode"`    // "EUR", "USD"
    CurrencySymbol  string      `json:"currencySymbol"`  // "€", "$"
    CurrencyPosition string    `json:"currencyPosition"` // "suffix" (€), "prefix" ($)
    DecimalSeparator string    `json:"decimalSeparator"` // "," (FR), "." (US)
    ThousandsSep     string    `json:"thousandsSep"`     // " " (FR), "," (US)
    UnitLabel       string     `json:"unitLabel"`        // "k€", "k$"
    DateFormat      string     `json:"dateFormat"`       // "DD/MM/YYYY", "MM/DD/YYYY"
}

var LocaleDefaults = map[CountryCode]LocaleInfo{
    CountryFR: {
        CountryCode: CountryFR, Locale: "fr-FR",
        CurrencyCode: "EUR", CurrencySymbol: "€", CurrencyPosition: "suffix",
        DecimalSeparator: ",", ThousandsSep: "\u00A0", // non-breaking space
        UnitLabel: "k€", DateFormat: "DD/MM/YYYY",
    },
    CountryUS: {
        CountryCode: CountryUS, Locale: "en-US",
        CurrencyCode: "USD", CurrencySymbol: "$", CurrencyPosition: "prefix",
        DecimalSeparator: ".", ThousandsSep: ",",
        UnitLabel: "k$", DateFormat: "MM/DD/YYYY",
    },
}
```

### 8.2 Frontend Formatting (Vue.js Reference)

```typescript
// composables/useLocale.ts

function formatCurrency(value: number, locale: LocaleInfo): string {
    return new Intl.NumberFormat(locale.locale, {
        style: 'currency',
        currency: locale.currencyCode,
        minimumFractionDigits: 1,
        maximumFractionDigits: 1,
    }).format(value)
}

// France: 1 234,5 €
// US: $1,234.5
```

---

## 9. Fiscal Year Rules

```go
// internal/compute/fiscal_year.go

// ValidateFiscalYear checks whether the chosen fiscal year start is valid
// for the selected country.
func ValidateFiscalYear(config model.PlanConfig, country model.Country) error {
    switch country.FiscalYearRule {
    case model.FYCalendarOnly:
        // France: fiscal year MUST start January 1
        if config.ForecastStart.Month() != time.January || config.ForecastStart.Day() != 1 {
            return apierror.ValidationError(
                "French law requires fiscal year to start January 1st",
                map[string]string{
                    "forecastStart": config.ForecastStart.Format("2006-01-02"),
                    "required":     "January 1",
                },
            )
        }
        // First fiscal year can be < 12 months (for new companies), but must end Dec 31
        return nil

    case model.FYFlexible:
        // US: any start date, but must be 12-month period
        // First fiscal year can be shorter
        return nil

    default:
        return nil
    }
}
```

---

## 10. Working Capital: Payment Term Defaults

```go
// internal/model/country_defaults.go

type CountryWCDefaults struct {
    CountryCode       CountryCode
    DefaultCustomerDays int    // Normal payment terms
    MaxCustomerDays     int    // Legal maximum (FR) or typical max (US)
    DefaultSupplierDays int
    MaxSupplierDays     int
    DefaultInventoryDays int
    LegalTermEnforced   bool   // France: true (legal penalties), US: false
}

var WCDefaults = map[CountryCode]CountryWCDefaults{
    CountryFR: {
        CountryCode:       CountryFR,
        DefaultCustomerDays: 45,   // 45 days end-of-month (standard)
        MaxCustomerDays:     60,   // Legal max: 60 days from invoice
        DefaultSupplierDays: 45,
        MaxSupplierDays:     60,
        DefaultInventoryDays: 30,
        LegalTermEnforced:   true, // French law LME (2008) + penalties
    },
    CountryUS: {
        CountryCode:       CountryUS,
        DefaultCustomerDays: 30,   // Net 30 standard
        MaxCustomerDays:     90,   // No legal max (market-driven)
        DefaultSupplierDays: 30,
        MaxSupplierDays:     90,
        DefaultInventoryDays: 45,
        LegalTermEnforced:   false,
    },
}
```

---

## 11. Impact on Existing Computation Functions

### 11.1 Changes Required Per Compute Function

| Function | Change Required | Details |
|---|---|---|
| `ComputeProductRevenue` | Minor | Currency-aware rounding, VAT engine for inclusive/exclusive price |
| `ComputeConsolidatedRevenue` | None | Already currency-agnostic |
| `ComputeStaffPayroll` | **Major** | Replace flat `EmployerTaxRate` with `PayrollTaxEngine` |
| `ComputeCapexSummary` | **Major** | Replace SLN-only with `DepreciationEngine` (SLN/MACRS/S179) |
| `ComputeSLNDepreciation` | **Replaced** | Becomes `ComputeDepreciation()` dispatching to method |
| `ComputeOpexSummary` | Minor | Tax engine for per-capita opex cost adjustments |
| `ComputePnl` | **Major** | Use `TaxEngine` for corporate tax, format-aware aggregation |
| `ComputeFiplan` | Minor | Currency labels, loan term norms |
| `ComputePnlCash` | Minor | Becomes primary for US plans |
| `ComputeBSheet` | Minor | Locale-aware presentation, no formula changes |
| `ComputeRatios` | Minor | Some ratios France-specific (Added Value) → conditional |
| `ComputeWCR` | Minor | Default payment terms from country profile |
| `ComputeCash` | Minor | VAT engine for collection/payment timing |
| `ComputeBudget1/2` | Minor | Upstream changes propagate automatically |
| `CalibrateFinancials` | Minor | Interest rates from country-specific profiles |

### 11.2 Refactored ComputeFullPlan Signature

```go
// internal/compute/engine.go

// FullPlanInput gains country profiles:
type FullPlanInput struct {
    // ... existing fields ...

    // ── Country Profiles (NEW) ──
    TaxProfile     model.CountryTaxProfile    // Corporate tax brackets + rules
    PayrollProfile model.CountryPayrollProfile // Employer/employee charges
    VATProfile     model.CountryVATProfile     // VAT or sales tax
    DeprecProfile  model.CountryDeprecProfile  // Depreciation methods + lives
}

// FullPlanOutput gains country-aware tax breakdown:
type FullPlanOutput struct {
    // ... existing fields ...

    // ── Country-Specific Additions (NEW) ──
    TaxBreakdown   [5]CorporateTaxResult  // Detailed tax per year
    PayrollDetail  PayrollBreakdownByYear  // Detailed payroll charges per year
    DeprecSchedule DepreciationSchedule    // Method-specific schedule (SLN/MACRS/S179)
}
```

---

## 12. Database Schema Changes

### 12.1 New Tables

```sql
-- Country registry
CREATE TABLE countries (
    code        VARCHAR(2) PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    name_local  VARCHAR(100),
    default_currency VARCHAR(3) NOT NULL,
    fiscal_year_rule VARCHAR(20) NOT NULL,
    accounting_std VARCHAR(20) NOT NULL,
    default_locale VARCHAR(10) NOT NULL,
    is_active   BOOLEAN DEFAULT true
);

-- Corporate tax profiles (multi-year)
CREATE TABLE country_tax_profiles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code    VARCHAR(2) NOT NULL REFERENCES countries(code),
    effective_year  INT NOT NULL,
    label           VARCHAR(100),
    social_contrib_on_profits_rate NUMERIC(6,4) DEFAULT 0,
    social_contrib_threshold NUMERIC(15,2) DEFAULT 0,
    loss_carry_forward_years INT DEFAULT -1,
    loss_carry_forward_cap NUMERIC(15,2) DEFAULT 0,
    loss_carry_forward_pct_above NUMERIC(6,4) DEFAULT 0,
    rnd_tax_credit_available BOOLEAN DEFAULT false,
    rnd_tax_credit_rate NUMERIC(6,4) DEFAULT 0,
    rnd_tax_credit_cap NUMERIC(15,2) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(country_code, effective_year)
);

CREATE TABLE corporate_tax_brackets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tax_profile_id  UUID NOT NULL REFERENCES country_tax_profiles(id) ON DELETE CASCADE,
    sort_order      INT NOT NULL,
    label           VARCHAR(50),
    rate            NUMERIC(8,5) NOT NULL,
    threshold_from  NUMERIC(15,2) DEFAULT 0,
    threshold_to    NUMERIC(15,2) DEFAULT 999999999,
    is_state_level  BOOLEAN DEFAULT false,
    state_code      VARCHAR(10)
);

-- Payroll profiles
CREATE TABLE country_payroll_profiles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code    VARCHAR(2) NOT NULL REFERENCES countries(code),
    effective_year  INT NOT NULL,
    label           VARCHAR(100),
    salary_months_default INT DEFAULT 12,
    minimum_wage_monthly NUMERIC(10,2),
    standard_weekly_hours NUMERIC(5,2) DEFAULT 35,
    overtime_multiplier NUMERIC(4,2) DEFAULT 1.25,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(country_code, effective_year)
);

CREATE TABLE payroll_charge_lines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id      UUID NOT NULL REFERENCES country_payroll_profiles(id) ON DELETE CASCADE,
    side            VARCHAR(10) NOT NULL, -- 'employer' | 'employee'
    category        VARCHAR(50) NOT NULL,
    label           VARCHAR(100) NOT NULL,
    rate            NUMERIC(8,5) NOT NULL,
    ceiling_type    VARCHAR(20) DEFAULT 'none',
    ceiling_amount  NUMERIC(15,2) DEFAULT 0,
    is_state_level  BOOLEAN DEFAULT false,
    state_code      VARCHAR(10),
    notes           VARCHAR(255)
);

-- VAT / Sales Tax profiles
CREATE TABLE country_vat_profiles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code    VARCHAR(2) NOT NULL REFERENCES countries(code),
    effective_year  INT NOT NULL,
    tax_system      VARCHAR(20) NOT NULL, -- 'vat' | 'sales_tax'
    standard_rate   NUMERIC(6,4) DEFAULT 0,
    included_in_price BOOLEAN DEFAULT true,
    vat_on_exports  BOOLEAN DEFAULT false,
    saas_is_taxable BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(country_code, effective_year)
);

-- Depreciation profiles
CREATE TABLE country_deprec_profiles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code    VARCHAR(2) NOT NULL REFERENCES countries(code),
    effective_year  INT NOT NULL,
    label           VARCHAR(100),
    default_method  VARCHAR(20) NOT NULL, -- 'straight_line' | 'macrs'
    section_179_available BOOLEAN DEFAULT false,
    section_179_limit NUMERIC(15,2) DEFAULT 0,
    section_179_phase_out NUMERIC(15,2) DEFAULT 0,
    bonus_deprec_rate NUMERIC(6,4) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(country_code, effective_year)
);

CREATE TABLE deprec_useful_lives (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id      UUID NOT NULL REFERENCES country_deprec_profiles(id) ON DELETE CASCADE,
    asset_category  VARCHAR(50) NOT NULL,
    default_years   INT NOT NULL,
    macrs_class     INT,
    notes           VARCHAR(255)
);
```

### 12.2 Existing Table Changes

```sql
-- plan_configs: add country fields
ALTER TABLE plan_configs
    ADD COLUMN country_code VARCHAR(2) NOT NULL DEFAULT 'FR' REFERENCES countries(code),
    ADD COLUMN state_code VARCHAR(10),
    ADD COLUMN currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    ADD COLUMN locale VARCHAR(10) NOT NULL DEFAULT 'fr-FR',
    ADD COLUMN tax_profile_year INT NOT NULL DEFAULT 2025,
    ADD COLUMN use_detailed_payroll_tax BOOLEAN DEFAULT false,
    ADD COLUMN employer_tax_override NUMERIC(6,4);

-- Rename employer_tax_rate to employer_tax_override for clarity
ALTER TABLE plan_configs RENAME COLUMN employer_tax_rate TO employer_tax_override;
```

---

## 13. API Changes

### 13.1 New Endpoints

```go
// Country registry (public, no auth needed)
r.Route("/countries", func(r chi.Router) {
    r.Get("/", handlers.Country.List)                    // List supported countries
    r.Get("/{code}", handlers.Country.Get)               // Country details
    r.Get("/{code}/tax/{year}", handlers.Country.GetTaxProfile)
    r.Get("/{code}/payroll/{year}", handlers.Country.GetPayrollProfile)
    r.Get("/{code}/vat/{year}", handlers.Country.GetVATProfile)
    r.Get("/{code}/depreciation/{year}", handlers.Country.GetDeprecProfile)
    r.Get("/{code}/states", handlers.Country.ListStates)  // US states list
})

// Tax simulation (authenticated)
r.Route("/plans/{planID}/scenarios/{scenID}/tax", func(r chi.Router) {
    r.Get("/corporate", handlers.Tax.GetCorporateTaxBreakdown)   // Detailed tax per year
    r.Get("/payroll", handlers.Tax.GetPayrollBreakdown)          // Detailed charges per role
    r.Post("/simulate", handlers.Tax.SimulateTaxChange)          // What-if on rates
})
```

### 13.2 Plan Creation: Country Selection

```go
type CreatePlanRequest struct {
    Name        string          `json:"name" validate:"required"`
    Description string          `json:"description"`
    CountryCode model.CountryCode `json:"countryCode" validate:"required,oneof=FR US"`
    StateCode   string          `json:"stateCode"`   // Required if countryCode=US
    Currency    string          `json:"currency"`     // Default from country
    ForecastStart time.Time     `json:"forecastStart" validate:"required"`
}
```

---

## 14. France vs US Summary: Impact Matrix

```
┌────────────────────────┬──────────────────────────────┬──────────────────────────────┐
│ Domain                 │ France (FR)                  │ United States (US)           │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Corporate Tax          │ 15% (SME ≤42.5k€) + 25%     │ 21% federal + 0-10% state    │
│                        │ + 3.3% social contribution   │ No social contribution       │
│                        │ Loss c/f: 1M€+50% above     │ Loss c/f: 80% of income      │
│                        │ CIR 30% R&D credit           │ ~20% R&D credit              │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Employer Payroll       │ ~42-47% of gross             │ ~8-12% of gross              │
│ (charges patronales    │ 15+ separate charges         │ FICA + FUTA + SUTA + WC      │
│  vs FICA/FUTA)         │ Complex ceiling system       │ Simple caps (SS wage base)   │
│                        │ 35h/week standard            │ 40h/week standard            │
│                        │ 12 or 13 salary months       │ 12 months only               │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Employee Deductions    │ ~22-23% (CSG/CRDS/retraite)  │ ~7.65% (FICA only)           │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ VAT / Sales Tax        │ 20% TVA (included in price)  │ 0-10% state+local            │
│                        │ Reduced: 10%, 5.5%, 2.1%     │ (added at checkout)          │
│                        │ Uniform national rate         │ Varies by state + product    │
│                        │ 0% on exports                │ SaaS: ~30 states taxable     │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Depreciation           │ Straight-line only (post-2020)│ MACRS (accelerated)          │
│                        │ No immediate expensing        │ Section 179: $2.5M immediate │
│                        │ Component approach            │ Bonus deprec: 40% Y1 (2025) │
│                        │ Longer useful lives           │ Shorter recovery periods     │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Fiscal Year            │ Calendar year only (Jan-Dec)  │ Any 12-month period          │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ P&L Format             │ Par nature (PCG) — primary    │ Functional (GAAP) — primary  │
│                        │ 7 aggregates (A→G)            │ Gross Profit → OpIncome      │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Payment Terms          │ Legal max: 60 days            │ Market-driven: 30-90 days    │
│                        │ Penalties enforced by law      │ Contractual only             │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Currency & Format      │ 1 234,50 €                   │ $1,234.50                    │
│                        │ DD/MM/YYYY                    │ MM/DD/YYYY                   │
│                        │ Price includes VAT (TTC)      │ Price excludes sales tax     │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Entity Types           │ SARL, SAS, SA                 │ C-Corp, S-Corp, LLC          │
├────────────────────────┼──────────────────────────────┼──────────────────────────────┤
│ Key Regulatory          │ LME (payment terms)          │ TCJA (Tax Cuts & Jobs Act)   │
│                        │ Code du travail (labor)       │ FLSA (labor)                 │
│                        │ PCG (accounting)              │ FASB/GAAP (accounting)       │
└────────────────────────┴──────────────────────────────┴──────────────────────────────┘
```

---

## 15. Migration Strategy: Adding a New Country

When adding a third country (e.g., Germany, UK), the process is:

1. **Add seed data**: `CountryTaxProfile`, `CountryPayrollProfile`, `CountryVATProfile`,
   `CountryDeprecProfile` for the new country
2. **Add `CountryCode` constant** and default locale
3. **No compute engine changes** — the engine uses profiles, not country-specific code
4. **Add fiscal year validation** if the country has specific rules
5. **Add UI translations** (label files in Vue i18n)
6. **Test** with country-specific edge cases

The architecture is designed so that adding a new country requires **zero changes to the
computation engine** — only new profile data and UI translations.
