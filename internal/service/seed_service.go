package service

// SeedService provisions the four built-in demo business plans for a tenant
// the very first time they list their plans.  Demo plans carry IsDemo=true and
// cannot be deleted by users.
//
// Each demo plan covers one canonical business archetype:
//   1. SaaS  – subscription + professional services, PLG growth model
//   2. Hardware – unit-price product, contract manufacturing
//   3. Consulting – daily-rate, utilisation-driven revenue model
//   4. Pro Tour Golfer – individual athlete: competition and contract drivers
//
// Data is mapped to Ascenda's underlying schema:
//   - Revenue   → Product + ProductAssumption + ProductSalesVolume
//   - COGS      → ProductAssumption.RawMaterialCost (per unit)
//   - Headcount → StaffHeadcount + StaffSalary (categories × 5 years, batch)
//   - Capex     → CapexEntry per AssetCategory per year (batch)
//   - Opex      → OpexManualEntry for the 12 user-input Opex lines (batch)

import (
	"context"
	"encoding/json"

	"ascenda/internal/compute"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// SeedService manages demo-plan provisioning.
type SeedService struct {
	repos          *repo.RepoBundle
	countryRateSvc *CountryRateConfigService // nil-safe
	logger         *logrus.Entry
}

// NewSeedService creates a new SeedService.
func NewSeedService(repos *repo.RepoBundle, countryRateSvc *CountryRateConfigService, logger *logrus.Entry) *SeedService {
	return &SeedService{repos: repos, countryRateSvc: countryRateSvc, logger: logger}
}

// auditCreate writes a single "create" audit entry for the given entity.
// Errors are logged but never returned — audit is non-critical for seeding.
func (s *SeedService) auditCreate(tenantID, userID, entityID uuid.UUID, entityType string, changes map[string]any) {
	var raw json.RawMessage
	if changes != nil {
		if b, err := json.Marshal(changes); err == nil {
			raw = b
		}
	}
	entry := &model.AuditLog{
		ID:         uuid.New(),
		TenantID:   tenantID,
		UserID:     userID,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     "create",
		Changes:    raw,
	}
	if err := s.repos.Audit.Create(entry); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"entity_type": entityType,
			"entity_id":   entityID,
		}).Warn("failed to write demo audit entry (non-fatal)")
	}
}

// ResetDemoPlans deletes all existing demo plans for the tenant and re-creates
// them from the current seed definitions.  Use this after data-model or seed
// changes to refresh demo content for a tenant.
func (s *SeedService) ResetDemoPlans(ctx context.Context, tenantID, userID uuid.UUID) error {
	s.logger.WithField("tenant_id", tenantID).Info("resetting demo plans")
	if err := s.repos.Plan.PurgeDemoPlans(tenantID); err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenantID).Error("failed to purge demo plans")
		return err
	}
	s.logger.WithField("tenant_id", tenantID).Info("demo plans purged, re-seeding")
	return s.EnsureDemoPlans(ctx, tenantID, userID)
}

// EnsureDemoPlans is idempotent: it creates the demo plans for the given
// tenant only if none exist yet.  Safe to call on every list-plans request.
func (s *SeedService) EnsureDemoPlans(ctx context.Context, tenantID, userID uuid.UUID) error {
	// Check whether the tenant already has demo plans.
	existing, err := s.repos.Plan.ListByTenant(tenantID, 0, 1000)
	if err != nil {
		return err
	}
	for _, p := range existing {
		if p.IsDemo {
			return nil // already seeded
		}
	}

	defs := demoPlanDefs()
	for _, def := range defs {
		if err := s.createDemoPlan(ctx, tenantID, userID, def); err != nil {
			s.logger.WithError(err).WithField("plan", def.name).Error("failed to seed demo plan")
			return err
		}
	}
	s.logger.WithField("tenant_id", tenantID).Info("demo plans seeded")
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

func d(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

// demoPlanDef holds all the data required to materialise one demo plan.
type demoPlanDef struct {
	name                string
	description         string
	scenarioDescription string
	companyName         string
	country             string
	products            []demoProduct
	// category → [y1..y5] FTE
	headcounts map[model.StaffCategory][5]decimal.Decimal
	// category → [y1..y5] monthly gross salary
	salaries map[model.StaffCategory][5]decimal.Decimal
	capex    []demoCapex
	opex     []demoOpex
	// fiplanEntries seeds financing plan rows (optional; nil = no external funding)
	fiplanEntries []demoFiplan
	// incentiveRates maps each overhead category to a variable-pay fraction of base salary.
	// e.g. 0.20 = 20 % target bonus.  nil = no variable pay seeded.
	incentiveRates map[model.StaffCategory]decimal.Decimal
	// wcCustomer30Pct / wcCustomer60Pct override the B2B default (0.5/0.5 → DSO 45 d).
	// Zero values fall back to the defaults.  For consulting set 0/1 (DSO 60 d).
	wcCustomer30Pct decimal.Decimal
	wcCustomer60Pct decimal.Decimal
	wcSupplier30Pct decimal.Decimal
	wcSupplier60Pct decimal.Decimal
	// wcNoInventory sets the inventory held (default 10 % of COGS) to zero,
	// for a business with no stock.
	wcNoInventory bool
	// opexPerHire adjusts the default opex rates (rent, per-head costs, % of
	// sales) and stores them with the plan. nil keeps the defaults, which are
	// then applied at compute time rather than stored.
	opexPerHire func(*model.OpexPerHire)
}

type demoProduct struct {
	name  string
	pType model.ProductType
	// DriverType selects the business-driver compute formula.
	// Leave zero to keep the default "generic" driver.
	driverType model.DriverType
	// driverParams is the typed parameter struct serialised to JSONB.
	// nil → generic product (manual ProductAssumption inputs only).
	driverParams any
	// baseUnitPrice per year (index 0 = Y1 … 4 = Y5)
	prices [5]decimal.Decimal
	// rawMaterialCost per unit per year (= cogs per unit)
	cogs [5]decimal.Decimal
	// units sold per year (direct / france channel)
	units [5]int64
}

type demoCapex struct {
	category  model.AssetCategory
	deprYears int
	amounts   [5]decimal.Decimal // Y1..Y5
}

type demoOpex struct {
	lineID  model.OpexLineID
	amounts [5]decimal.Decimal // Y1..Y5
}

// demoFiplan seeds one financing plan line across the 5-year horizon.
type demoFiplan struct {
	lineID  model.FiplanLineID
	amounts [5]decimal.Decimal // Y1..Y5
}

// demoPlanDefs returns the four archetype plan definitions.
func demoPlanDefs() []demoPlanDef {
	return []demoPlanDef{saasDemo(), hardwareDemo(), consultingDemo(), golfDemo()}
}

// ── 1. SaaS ──────────────────────────────────────────────────────────────────

func saasDemo() demoPlanDef {
	return demoPlanDef{
		name:                "SaaS Startup — Ascenda Demo",
		description:         "Software-as-a-Service subscription model with a professional-services upsell. Demonstrates PLG-driven client acquisition, 12 % annual churn and 110 % net revenue retention over 5 years.",
		scenarioDescription: "Base-case projection: 12 % annual churn, 110 % NRR, headcount growing from 5 to 22 FTE. Opex dominated by R&D salaries and cloud infrastructure. Breakeven expected in Year 3.",
		companyName:         "SaaS Co.",
		country:             "FR",
		products: []demoProduct{
			{
				// MRR × 12 = ARR; 1 unit = 1 active client-year
				// BaseUnitPrice = annual ARPU (prices[y]), COGS = 15 % of ARPU
				name:       "SaaS Subscription",
				pType:      model.ProductTypeService,
				driverType: model.DriverSaaS,
				driverParams: model.SaaSParams{
					// ActiveUsers at year-end
					ActiveUsers: [5]model.FlexInt64{50, 150, 380, 750, 1300},
					// Monthly fee per user (ARPU / 12)
					MonthlyFee: [5]decimal.Decimal{d(500), d(517), d(540), d(560), d(580)},
					// Infra cost per user per month (~10 % of MRR)
					InfraCostPPU: [5]decimal.Decimal{d(50), d(52), d(54), d(56), d(58)},
					// Support cost per user per month (~5 % of MRR)
					SupportCostPPU: [5]decimal.Decimal{d(25), d(26), d(27), d(28), d(29)},
					// ~1.2 % monthly churn → ~14 % annual churn
					ChurnRate: [5]decimal.Decimal{d(0.012), d(0.012), d(0.011), d(0.010), d(0.010)},
					// ~10 % net revenue expansion from upsells
					ExpansionRate: [5]decimal.Decimal{d(0.10), d(0.10), d(0.10), d(0.10), d(0.10)},
				},
				// prices kept consistent with SaaSParams.MonthlyFee × 12
				prices: [5]decimal.Decimal{d(6000), d(6200), d(6480), d(6720), d(6960)},
				// COGS = 15 % of ARPU (infra + support per user × 12)
				cogs:  [5]decimal.Decimal{d(900), d(930), d(972), d(1008), d(1044)},
				units: [5]int64{50, 150, 380, 750, 1300},
			},
			{
				// Professional services; 1 "unit" = the annual services bucket
				// Generic driver — revenue is a single lump figure per year
				name:   "Professional Services",
				pType:  model.ProductTypeService,
				prices: [5]decimal.Decimal{d(50000), d(126000), d(250000), d(420000), d(650000)},
				cogs:   [5]decimal.Decimal{d(0), d(0), d(0), d(0), d(0)},
				units:  [5]int64{1, 1, 1, 1, 1},
			},
		},
		headcounts: map[model.StaffCategory][5]decimal.Decimal{
			model.CategoryRnDEngineers:    {d(5), d(8), d(14), d(22), d(30)},
			model.CategorySalesTeam:       {d(1), d(3), d(6), d(10), d(15)},
			model.CategoryProdTechnicians: {d(1), d(2), d(4), d(6), d(9)},
			model.CategoryAdminManagers:   {d(1), d(2), d(3), d(4), d(6)},
		},
		// avg salary ≈ 60–68 k€/year → monthly ≈ 5 000–5 667 €
		salaries: map[model.StaffCategory][5]decimal.Decimal{
			model.CategoryRnDEngineers:    {d(5000), d(5167), d(5333), d(5500), d(5667)},
			model.CategorySalesTeam:       {d(5000), d(5167), d(5333), d(5500), d(5667)},
			model.CategoryProdTechnicians: {d(5000), d(5167), d(5333), d(5500), d(5667)},
			model.CategoryAdminManagers:   {d(5000), d(5167), d(5333), d(5500), d(5667)},
		},
		// All amounts in base € (frontend divides by unit factor for display).
		capex: []demoCapex{
			{model.AssetRnDExpenses, 4, [5]decimal.Decimal{d(100000), d(150000), d(200000), d(250000), d(300000)}},
			{model.AssetComputerHWSW, 3, [5]decimal.Decimal{d(40000), d(60000), d(80000), d(100000), d(120000)}},
		},
		opex: []demoOpex{
			{model.LineAdvertisingComms, [5]decimal.Decimal{d(80000), d(180000), d(350000), d(600000), d(900000)}},
			{model.LineTechCostsWebHosting, [5]decimal.Decimal{d(30000), d(50000), d(80000), d(120000), d(170000)}},
			{model.LineProfessionalFees, [5]decimal.Decimal{d(15000), d(30000), d(55000), d(85000), d(120000)}},
			{model.LineOtherExpenses, [5]decimal.Decimal{d(25000), d(40000), d(60000), d(90000), d(130000)}},
		},
	}
}

// ── 2. Hardware ───────────────────────────────────────────────────────────────

func hardwareDemo() demoPlanDef {
	return demoPlanDef{
		name:                "Hardware Scaleup — Ascenda Demo",
		description:         "Specialty hardware company using contract manufacturing. Demonstrates unit-economics scaling with ~50 % base BOM cost, scrap-adjusted COGS, and heavy capex in R&D and production equipment over 5 years.",
		scenarioDescription: "Base-case projection: ~40–49 % gross margin after scrap and setup costs, ASP rising from €12 000 to €15 500. Capex-heavy early years (R&D, tooling, production equipment). EBITDA positive from Year 2.",
		companyName:         "HardCo.",
		country:             "FR",
		products: []demoProduct{
			{
				name:       "Hardware Units",
				pType:      model.ProductTypeProduct,
				driverType: model.DriverIndustry,
				driverParams: model.IndustryParams{
					// Production capacity grows with investment in tooling/equipment
					ProductionCapacity: [5]model.FlexInt64{30, 120, 300, 600, 1100},
					// 2 % scrap rate (contract manufacturing quality losses)
					ScrapRate: [5]decimal.Decimal{d(0.02), d(0.02), d(0.015), d(0.015), d(0.01)},
					// Per-batch tooling and setup cost (amortised annually)
					SetupCost: [5]decimal.Decimal{d(20000), d(30000), d(40000), d(50000), d(60000)},
				},
				prices: [5]decimal.Decimal{d(12000), d(13500), d(14500), d(15000), d(15500)},
				// COGS = 50 % (BOM + contract manufacturing)
				cogs:  [5]decimal.Decimal{d(6000), d(6750), d(7250), d(7500), d(7750)},
				units: [5]int64{20, 80, 200, 450, 800},
			},
		},
		headcounts: map[model.StaffCategory][5]decimal.Decimal{
			model.CategoryRnDEngineers:    {d(6), d(10), d(15), d(20), d(25)},
			model.CategorySalesTeam:       {d(1), d(2), d(4), d(7), d(10)},
			model.CategoryProdTechnicians: {d(2), d(4), d(7), d(12), d(18)},
			model.CategoryAdminManagers:   {d(1), d(2), d(3), d(4), d(5)},
		},
		// avg salary ≈ 65–72 k€/year → monthly ≈ 5 417–6 000 €
		salaries: map[model.StaffCategory][5]decimal.Decimal{
			model.CategoryRnDEngineers:    {d(5417), d(5583), d(5667), d(5833), d(6000)},
			model.CategorySalesTeam:       {d(5417), d(5583), d(5667), d(5833), d(6000)},
			model.CategoryProdTechnicians: {d(5417), d(5583), d(5667), d(5833), d(6000)},
			model.CategoryAdminManagers:   {d(5417), d(5583), d(5667), d(5833), d(6000)},
		},
		// All amounts in base € (frontend divides by unit factor for display).
		capex: []demoCapex{
			{model.AssetRnDExpenses, 4, [5]decimal.Decimal{d(300000), d(400000), d(500000), d(600000), d(700000)}},
			{model.AssetEquipmentTools, 5, [5]decimal.Decimal{d(200000), d(350000), d(500000), d(700000), d(900000)}},
		},
		opex: []demoOpex{
			{model.LineAdvertisingComms, [5]decimal.Decimal{d(40000), d(100000), d(200000), d(350000), d(550000)}},
			{model.LineTechCostsWebHosting, [5]decimal.Decimal{d(25000), d(40000), d(60000), d(90000), d(130000)}},
			{model.LineProfessionalFees, [5]decimal.Decimal{d(30000), d(60000), d(100000), d(160000), d(220000)}},
			{model.LineOtherExpenses, [5]decimal.Decimal{d(35000), d(55000), d(80000), d(110000), d(150000)}},
		},
	}
}

// ── 3. Consulting ─────────────────────────────────────────────────────────────

func consultingDemo() demoPlanDef {
	// Pure consulting model: two billable tiers, no R&D capex.
	//
	// Delivery cost model: consultant cost is embedded in COGS per billable day,
	// not in the staff headcount table. This avoids double-counting and correctly
	// treats delivery as a variable cost of revenue.
	//
	// COGS per billable day = (FTE × monthly gross × 12 × employer_charge_factor)
	//                         / billable_days_per_year
	// French employer charge factor = 1.45.
	//
	// Strategy & Advisory (senior consultant: 9 000–11 000 €/month gross):
	//   Y1: 1 FTE × 9 000×12×1.45 / 187 days  = 837 €/day  (~33 % of day rate)
	//   Y2: 3 FTE × 9 500×12×1.45 / 541 days  = 917 €/day  (~35 %)
	//   Y3: 6 FTE × 10 000×12×1.45 / 1 095    = 953 €/day  (~35 %)
	//   Y4: 10 FTE × 10 500×12×1.45 / 1 848   = 989 €/day  (~34 %)
	//   Y5: 15 FTE × 11 000×12×1.45 / 2 805   = 1 023 €/day (~32 %)
	//
	// Consulting & Delivery (standard consultant: 6 500–8 000 €/month gross):
	//   Y1: 4 FTE × 6 500×12×1.45 / 660 days  = 686 €/day  (~49 % of day rate)
	//   Y2: 8 FTE × 6 800×12×1.45 / 1 267     = 746 €/day  (~50 %)
	//   Y3: 15 FTE × 7 100×12×1.45 / 2 475    = 750 €/day  (~47 %)
	//   Y4: 25 FTE × 7 500×12×1.45 / 4 180    = 780 €/day  (~46 %)
	//   Y5: 37 FTE × 8 000×12×1.45 / 6 349    = 813 €/day  (~45 %)
	//
	// Billable days (units) = trunc(FTE × utilisation × 220 w-days):
	//   Advisory  Y1: 1×0.85×220=187      Y2: 3×0.82×220=541        Y3: trunc(6×0.83×220)=1 095
	//             Y4: 10×0.84×220=1 848   Y5: 15×0.85×220=2 805
	//   Delivery  Y1: 4×0.75×220=660      Y2: trunc(8×0.72×220)=1 267   Y3: 15×0.75×220=2 475
	//             Y4: 25×0.76×220=4 180   Y5: trunc(37×0.78×220)=6 349
	//
	// Staff table carries only fixed overhead (Sales, Marketing, Admin, Executive).
	return demoPlanDef{
		name:                "Consulting Firm — Ascenda Demo",
		description:         "Pure professional-services firm with two billing tiers: premium Strategy & Advisory (daily rate €2 500–3 200) and standard Consulting & Delivery (€1 400–1 800/day). Delivery cost is modelled as COGS per billable day — consultant salaries + 45 % French employer charges embedded in cost of revenue. Staff table shows overhead only. ~55–65 % gross margin per tier.",
		scenarioDescription: "Base-case projection: 60–70 % billable utilisation, two revenue tiers, 45 % employer charges baked into COGS per day. Reaches 20 % EBITDA margin in Year 2, 30 % by Year 5. Travel and recruitment are the main opex drivers.",
		companyName:         "ConsultCo.",
		country:             "FR",
		products: []demoProduct{
			{
				// 1 unit = 1 billable senior-consultant-day (Strategy & Advisory)
				// COGS per day = full employer cost of 1 senior consultant ÷ billable days
				// (9 000–11 000 €/month × 12 × 1.45 employer factor) ÷ annual billable days
				name:       "Strategy & Advisory",
				pType:      model.ProductTypeService,
				driverType: model.DriverConsulting,
				driverParams: model.ConsultingParams{
					// Senior billable headcount per year (Y1..Y5)
					Headcount:   [5]decimal.Decimal{d(1), d(3), d(6), d(10), d(15)},
					WorkingDays: 220,
					// Utilisation rates from the inline comment (85 %→82 %→83 %→84 %→85 %)
					UtilizationRate: [5]decimal.Decimal{d(0.85), d(0.82), d(0.83), d(0.84), d(0.85)},
					// Average monthly gross salary of a senior consultant (€)
					MonthlyGross: [5]decimal.Decimal{d(9000), d(9500), d(10000), d(10500), d(11000)},
					// French employer charge factor
					EmployerCharges: d(1.45),
				},
				prices: [5]decimal.Decimal{d(2500), d(2600), d(2700), d(2900), d(3200)},
				cogs:   [5]decimal.Decimal{d(837), d(917), d(953), d(989), d(1023)},
				units:  [5]int64{187, 541, 1095, 1848, 2805},
			},
			{
				// 1 unit = 1 billable standard-consultant-day (Consulting & Delivery)
				// COGS per day = full employer cost of 1 standard consultant ÷ billable days
				// (6 500–8 000 €/month × 12 × 1.45 employer factor) ÷ annual billable days
				name:       "Consulting & Delivery",
				pType:      model.ProductTypeService,
				driverType: model.DriverConsulting,
				driverParams: model.ConsultingParams{
					// Standard billable headcount per year (Y1..Y5)
					Headcount:   [5]decimal.Decimal{d(4), d(8), d(15), d(25), d(37)},
					WorkingDays: 220,
					// Utilisation rates from the inline comment (75 %→72 %→75 %→76 %→78 %)
					UtilizationRate: [5]decimal.Decimal{d(0.75), d(0.72), d(0.75), d(0.76), d(0.78)},
					// Average monthly gross salary of a standard consultant (€)
					MonthlyGross: [5]decimal.Decimal{d(6500), d(6800), d(7100), d(7500), d(8000)},
					// French employer charge factor
					EmployerCharges: d(1.45),
				},
				prices: [5]decimal.Decimal{d(1400), d(1500), d(1600), d(1700), d(1800)},
				cogs:   [5]decimal.Decimal{d(686), d(746), d(750), d(780), d(813)},
				units:  [5]int64{660, 1267, 2475, 4180, 6349},
			},
		},
		// Overhead staff only — delivery consultants are costed via COGS above.
		// HR and Finance added from Y3/Y4 once the firm exceeds ~20 people.
		headcounts: map[model.StaffCategory][5]decimal.Decimal{
			model.CategorySalesTeam:     {d(1), d(2), d(4), d(7), d(11)},
			model.CategoryMarketingTeam: {d(0), d(1), d(1), d(2), d(3)},
			model.CategoryAdminManagers: {d(1), d(1), d(2), d(3), d(6)},
			model.CategoryExecutiveTeam: {d(1), d(1), d(1), d(2), d(3)},
			// People & talent: hired once firm crosses ~20 total staff (Y3)
			model.CategoryHR: {d(0), d(0), d(1), d(2), d(3)},
			// Finance & controlling: hired in Y4 as revenues pass ~10 M EUR
			model.CategoryFinance: {d(0), d(0), d(0), d(1), d(1)},
		},
		salaries: map[model.StaffCategory][5]decimal.Decimal{
			model.CategorySalesTeam:     {d(5500), d(5750), d(6000), d(6500), d(7000)},
			model.CategoryMarketingTeam: {d(5000), d(5250), d(5500), d(5750), d(6000)},
			model.CategoryAdminManagers: {d(4500), d(4750), d(5000), d(5250), d(5500)},
			model.CategoryExecutiveTeam: {d(12000), d(12500), d(13000), d(14000), d(15000)},
			model.CategoryHR:            {d(5500), d(5750), d(6000), d(6250), d(6500)},
			model.CategoryFinance:       {d(7500), d(8000), d(8000), d(8500), d(9000)},
		},
		// Variable pay as fraction of annual base salary per category.
		incentiveRates: map[model.StaffCategory]decimal.Decimal{
			model.CategorySalesTeam:     d(0.20),
			model.CategoryExecutiveTeam: d(0.25),
			model.CategoryMarketingTeam: d(0.10),
			model.CategoryAdminManagers: d(0.08),
			model.CategoryHR:            d(0.08),
			model.CategoryFinance:       d(0.12),
		},
		capex: []demoCapex{
			{model.AssetComputerHWSW, 3, [5]decimal.Decimal{d(15000), d(20000), d(30000), d(45000), d(65000)}},
			{model.AssetOfficeFurniture, 5, [5]decimal.Decimal{d(20000), d(0), d(25000), d(0), d(35000)}},
			{model.AssetSetupExpenses, 5, [5]decimal.Decimal{d(15000), d(0), d(0), d(0), d(0)}},
		},
		opex: []demoOpex{
			{model.LineTravelTransport, [5]decimal.Decimal{d(40000), d(80000), d(140000), d(220000), d(350000)}},
			{model.LineProfessionalFees, [5]decimal.Decimal{d(50000), d(100000), d(160000), d(230000), d(300000)}},
			{model.LineAdvertisingComms, [5]decimal.Decimal{d(25000), d(50000), d(90000), d(140000), d(200000)}},
			{model.LineRecruitmentTraining, [5]decimal.Decimal{d(30000), d(60000), d(100000), d(160000), d(250000)}},
			// Laptop/mobile/collab-tool leasing for all staff (billable + overhead)
			{model.LineLeasingMovable, [5]decimal.Decimal{d(30000), d(55000), d(90000), d(140000), d(210000)}},
			// Industry conferences, sponsorships and awards
			{model.LineTradeShows, [5]decimal.Decimal{d(20000), d(40000), d(70000), d(110000), d(160000)}},
			// Subscriptions, office consumables, misc operational costs
			{model.LineOtherExpenses, [5]decimal.Decimal{d(20000), d(35000), d(55000), d(80000), d(110000)}},
		},
		// Founder equity + working-capital credit line in Y1. Dividends from Y3.
		fiplanEntries: []demoFiplan{
			{model.FiplanCapitalIncrease, [5]decimal.Decimal{d(150000), d(0), d(0), d(0), d(0)}},
			{model.FiplanLTLoans, [5]decimal.Decimal{d(100000), d(0), d(0), d(0), d(0)}},
			{model.FiplanDividends, [5]decimal.Decimal{d(0), d(0), d(50000), d(100000), d(150000)}},
		},
		// Corporate clients pay in 60 days (DSO = 60 d).
		// Firm pays suppliers in 45 days (default 0.5/0.5 split).
		wcCustomer30Pct: d(0),
		wcCustomer60Pct: d(1),
		wcSupplier30Pct: d(0.5),
		wcSupplier60Pct: d(0.5),
	}
}

// ── 4. Pro Tour Golfer ────────────────────────────────────────────────────────

// Tour economics per event, in EUR — the "Challenge Tour" and "DP World Tour"
// presets of the frontend's competition driver form (athleteDrivers.ts), so a
// user who re-selects the tour gets the same figures.
var (
	golfChallengeTour = golfTour{name: "Challenge Tour", win: d(45000), top10: d(12000), cut: d(2500)}
	golfDPWorldTour   = golfTour{name: "DP World Tour", win: d(380000), top10: d(90000), cut: d(12000)}
)

type golfTour struct {
	name            string
	win, top10, cut decimal.Decimal
}

func golfDemo() demoPlanDef {
	// Individual professional golfer (fictional) — two seasons on the Challenge
	// Tour, a win and a top-20 Order of Merit finish in Y2, then three seasons
	// on the DP World Tour.
	//
	// Revenue model — the athlete drivers, not unit prices:
	//   Prize money   : competition driver. Per season: events, cuts made,
	//                   top-10s and wins × the tour's average payouts. Direct
	//                   costs per event (entry, tournament travel, caddie fee)
	//                   plus the caddie's and the coach's share of winnings and
	//                   the coach's annual retainer.
	//   Sponsorship   : contract driver. One line per partner, value per year,
	//                   plus a bonus per win (wins read from the prize money
	//                   product).
	//   Image rights  : contract driver. Media and licensing deals.
	//   Appearances   : generic service — 1 unit = 1 pro-am / corporate day.
	//
	// The caddie and the coach are costed inside the competition driver, and
	// tournament travel too, so they are not in headcount or in the travel opex
	// line: headcount is the off-course team (fitness trainer, manager, PA),
	// travel opex is training camps and sponsor days.
	tours := [5]golfTour{golfChallengeTour, golfChallengeTour, golfDPWorldTour, golfDPWorldTour, golfDPWorldTour}
	prize := model.CompetitionParams{
		Events: [5]model.FlexInt64{22, 23, 26, 25, 24},
		Cuts:   [5]model.FlexInt64{12, 15, 14, 16, 17},
		Top10s: [5]model.FlexInt64{4, 5, 1, 3, 3}, // excluding wins
		Wins:   [5]model.FlexInt64{0, 1, 0, 0, 1},

		EntryFeePerEvent:  [5]decimal.Decimal{d(250), d(250), d(400), d(400), d(400)},
		TravelPerEvent:    [5]decimal.Decimal{d(1800), d(1800), d(3800), d(3800), d(3800)},
		CaddieFeePerEvent: [5]decimal.Decimal{d(1000), d(1000), d(1500), d(1500), d(1500)},
		CaddieShare:       [5]decimal.Decimal{d(0.05), d(0.05), d(0.07), d(0.08), d(0.08)},
		CoachAnnualFee:    [5]decimal.Decimal{d(10000), d(12000), d(24000), d(30000), d(36000)},
		CoachShare:        [5]decimal.Decimal{d(0.02), d(0.02), d(0.03), d(0.04), d(0.05)},
	}
	for y, tour := range tours {
		prize.Circuit[y] = tour.name
		prize.PrizePerWin[y] = tour.win
		prize.PrizePerTop10[y] = tour.top10
		prize.PrizePerCut[y] = tour.cut
		prize.OtherPrizeMoney[y] = d(0)
	}

	sponsorship := model.ContractParams{Contracts: []model.ContractLine{
		{Partner: "Equipment brand (clubs & ball)", Amounts: [5]decimal.Decimal{d(8000), d(12000), d(50000), d(80000), d(120000)}, BonusPerWin: d(10000)},
		{Partner: "Apparel brand", Amounts: [5]decimal.Decimal{d(5000), d(8000), d(20000), d(35000), d(60000)}, BonusPerWin: d(5000)},
		{Partner: "Regional bank (cap logo)", Amounts: [5]decimal.Decimal{d(10000), d(10000), d(25000), d(30000), d(40000)}, BonusPerWin: d(5000)},
		{Partner: "Watch brand", Amounts: [5]decimal.Decimal{d(0), d(0), d(0), d(50000), d(100000)}, BonusPerWin: d(15000)},
	}}
	image := model.ContractParams{Contracts: []model.ContractLine{
		{Partner: "Media & content partnership", Amounts: [5]decimal.Decimal{d(0), d(0), d(10000), d(20000), d(35000)}, BonusPerWin: d(0)},
		{Partner: "Golf academy licensing", Amounts: [5]decimal.Decimal{d(0), d(0), d(0), d(15000), d(25000)}, BonusPerWin: d(0)},
	}}

	return demoPlanDef{
		name:                "Pro Tour Golfer — Ascenda Demo",
		description:         "Individual professional golfer: two seasons on the Challenge Tour, then three on the DP World Tour. Prize money comes from the season's results (events, cuts, top-10s, wins) and the tour's payouts, with tournament costs, caddie and coach; sponsorship and image rights are contracts, with bonuses per win; appearances are billed per day. Uses the competition and contract drivers.",
		scenarioDescription: "Base case: a Challenge Tour rookie (no win, four top-10s) who wins in Y2, finishes in the Order of Merit top 20 and plays the DP World Tour from Y3 — keeping the card, then a first DP World Tour win in Y5. Revenue grows from about €97K to €1.38M. Equipment and apparel deals step up with the DP World Tour card; a watch brand signs in Y4. Funded by €100K of savings and a €50K bank loan; a manager and a part-time fitness trainer join in Y3, a PA in Y4.",
		companyName:         "ProGolf SAS",
		country:             "FR",
		products: deriveAthleteRows([]demoProduct{
			{
				name:         "Tournament Prize Money",
				pType:        model.ProductTypeService,
				driverType:   model.DriverCompetition,
				driverParams: prize,
			},
			{
				name:         "Sponsorship",
				pType:        model.ProductTypeService,
				driverType:   model.DriverContract,
				driverParams: sponsorship,
			},
			{
				name:         "Image Rights & Media",
				pType:        model.ProductTypeService,
				driverType:   model.DriverContract,
				driverParams: image,
			},
			{
				// 1 unit = 1 corporate pro-am or appearance day; COGS = travel
				// and preparation per day.
				name:   "Appearance Fees & Pro-Ams",
				pType:  model.ProductTypeService,
				prices: [5]decimal.Decimal{d(1500), d(2000), d(4000), d(6000), d(8000)},
				cogs:   [5]decimal.Decimal{d(150), d(200), d(400), d(500), d(600)},
				units:  [5]int64{4, 8, 12, 16, 20},
			},
		}),
		// Off-course team only — caddie and coach are in the prize money product.
		// Y1–Y2: the player manages alone.
		headcounts: map[model.StaffCategory][5]decimal.Decimal{
			// Fitness trainer / physio: part-time in Y3, full-time from Y4.
			model.CategoryProdTechnicians: {d(0), d(0), d(0.5), d(1), d(1)},
			// Manager: from Y3, with the DP World Tour card and the bigger deals.
			model.CategoryAdminManagers: {d(0), d(0), d(1), d(1), d(1)},
			// PA: part-time in Y4, full-time in Y5.
			model.CategoryAdminAssistants: {d(0), d(0), d(0), d(0.5), d(1)},
		},
		// Monthly gross salary (€).
		salaries: map[model.StaffCategory][5]decimal.Decimal{
			model.CategoryProdTechnicians: {d(3400), d(3500), d(3600), d(3800), d(4000)},
			model.CategoryAdminManagers:   {d(4000), d(4000), d(4000), d(4300), d(4600)},
			model.CategoryAdminAssistants: {d(2800), d(2900), d(3000), d(3200), d(3400)},
		},
		capex: []demoCapex{
			// Launch monitor (TrackMan), putting analysis, club fitting.
			{model.AssetEquipmentTools, 3, [5]decimal.Decimal{d(15000), d(5000), d(8000), d(10000), d(12000)}},
			// Vehicle for the European events reachable by road.
			{model.AssetVehicles, 5, [5]decimal.Decimal{d(25000), d(0), d(0), d(0), d(30000)}},
		},
		opex: []demoOpex{
			// Training camps and sponsor days — tournament travel is in the prize money product.
			{model.LineTravelTransport, [5]decimal.Decimal{d(6000), d(8000), d(15000), d(20000), d(25000)}},
			// Agent commission on sponsorship and image deals, legal fees.
			{model.LineProfessionalFees, [5]decimal.Decimal{d(5000), d(8000), d(18000), d(35000), d(60000)}},
			// Fitness and mental coaching sessions, training facilities.
			{model.LineRecruitmentTraining, [5]decimal.Decimal{d(5000), d(6000), d(12000), d(18000), d(25000)}},
			// Sponsor hospitality, image and PR.
			{model.LineMissionRepresentation, [5]decimal.Decimal{d(2000), d(4000), d(10000), d(20000), d(35000)}},
			// Tour membership, insurance, equipment upkeep.
			{model.LineOtherExpenses, [5]decimal.Decimal{d(4000), d(5000), d(8000), d(12000), d(16000)}},
		},
		// Y1: €100K personal savings + €50K bank loan; dividends from Y4.
		fiplanEntries: []demoFiplan{
			{model.FiplanCapitalIncrease, [5]decimal.Decimal{d(100000), d(0), d(0), d(0), d(0)}},
			{model.FiplanLTLoans, [5]decimal.Decimal{d(50000), d(0), d(0), d(0), d(0)}},
			{model.FiplanDividends, [5]decimal.Decimal{d(0), d(0), d(0), d(50000), d(150000)}},
		},
		// Prize money, sponsor and appearance fees are paid within ~30 days;
		// tournament costs are settled on the spot.
		wcCustomer30Pct: d(1),
		wcCustomer60Pct: d(0),
		wcSupplier30Pct: d(1),
		wcSupplier60Pct: d(0),
		// No stock: equipment is capex, and the rest is consumed as it goes.
		wcNoInventory: true,
		// No office rent (the player works from home; the home club is in other
		// expenses) and no patent royalties; the other defaults stand.
		opexPerHire: func(o *model.OpexPerHire) {
			o.PropertyRentals = decimal.Zero
			o.RoyaltyPaymentsPctSales = decimal.Zero
		},
	}
}

// deriveAthleteRows fills the stored price, cost and volume rows of the
// competition and contract products with what their drivers compute, so the
// seeded rows agree with the parameters (as they do for the other demos).
// Contract bonuses read the wins of the plan's competition products.
func deriveAthleteRows(products []demoProduct) []demoProduct {
	models := make([]model.Product, len(products))
	for i, pd := range products {
		models[i] = model.Product{DriverType: pd.driverType}
		if pd.driverParams != nil {
			models[i].DriverParams, _ = json.Marshal(pd.driverParams)
		}
	}
	ctx := compute.BuildDriverContext(models)

	for i, pd := range products {
		if pd.driverType != model.DriverCompetition && pd.driverType != model.DriverContract {
			continue
		}
		bundle, err := compute.ApplyDriverComputeWithContext(models[i], compute.ProductInputBundle{}, ctx)
		if err != nil {
			continue // invalid params: left empty, and TestGolfDemo_DriverParamsAreValid fails
		}
		for y := 0; y < 5; y++ {
			products[i].prices[y] = bundle.Assumptions[y].BaseUnitPrice
			products[i].cogs[y] = bundle.Assumptions[y].RawMaterialCost
		}
		for _, v := range bundle.Volumes {
			if v.YearIndex >= 1 && v.YearIndex <= 5 {
				products[i].units[v.YearIndex-1] += v.UnitsSold
			}
		}
	}
	return products
}

// ── materialise one demo plan ─────────────────────────────────────────────────

func (s *SeedService) createDemoPlan(ctx context.Context, tenantID, userID uuid.UUID, def demoPlanDef) error {
	// 1. Business plan (IsDemo = true → cannot be deleted)
	plan := &model.BusinessPlan{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		Name:         def.name,
		Description:  def.description,
		Status:       "approved",
		CreatedBy:    userID,
		IsDemo:       true,
	}
	if err := s.repos.Plan.Create(plan); err != nil {
		return err
	}
	s.auditCreate(tenantID, userID, plan.ID, "plan", map[string]any{
		"name":   def.name,
		"status": "approved",
		"isDemo": true,
	})

	// 2. Default scenario
	scenario := &model.Scenario{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		PlanID:       plan.ID,
		Name:         "Base",
		Description:  def.scenarioDescription,
		IsDefault:    true,
	}
	if err := s.repos.Scenario.Create(scenario); err != nil {
		return err
	}
	s.auditCreate(tenantID, userID, scenario.ID, "scenario", map[string]any{
		"name":        "Base",
		"description": def.scenarioDescription,
		"isDefault":   true,
		"planId":      plan.ID.String(),
	})

	// 3. PlanConfig (country-specific statutory defaults + company name)
	// Use DB-backed rates when available, falling back to hard-coded defaults.
	var rates countryRates
	if s.countryRateSvc != nil {
		rates = s.countryRateSvc.RatesFor(def.country)
	} else {
		rates = ratesFor(def.country)
	}
	cfg := defaultPlanConfigFromRates(tenantID, scenario.ID, def.country, rates)
	cfg.CompanyName = def.companyName
	if err := s.repos.Settings.UpsertConfig(cfg); err != nil {
		return err
	}
	s.auditCreate(tenantID, userID, scenario.ID, "settings", map[string]any{
		"companyName":         def.companyName,
		"country":             def.country,
		"forecastStart":       cfg.ForecastStart,
		"corporateTaxRate":    cfg.CorporateTaxRate,
		"employerTaxRate":     cfg.EmployerTaxRate,
		"vatRate":             cfg.VATRate,
		"salaryMonthsPerYear": cfg.SalaryMonthsPerYear,
	})
	// Opening balance — start-up with no prior history: all positions are zero.
	s.auditCreate(tenantID, userID, scenario.ID, "opening_balance", map[string]any{
		"noncurrentAssets":    0,
		"inventories":         0,
		"customerReceivables": 0,
		"cashAndSecurities":   0,
		"shareCapital":        0,
		"retainedEarnings":    0,
		"loansAndDebt":        0,
		"supplierPayables":    0,
	})
	// Working capital config — B2B standard: 50 % at 30 days / 50 % at 60 days
	// for both customers and suppliers → DSO = DPO ≈ 45 days; inventory 10 % of COGS.
	// When the plan definition specifies explicit customer/supplier splits those
	// override the B2B defaults (e.g. consulting: DSO 60 d → 0/1 split).
	wc := wcConfigB2BDefaults(tenantID, scenario.ID)
	if !def.wcCustomer30Pct.IsZero() || !def.wcCustomer60Pct.IsZero() {
		wc.CustomerPct30Days = def.wcCustomer30Pct
		wc.CustomerPct60Days = def.wcCustomer60Pct
	}
	if !def.wcSupplier30Pct.IsZero() || !def.wcSupplier60Pct.IsZero() {
		wc.SupplierPct30Days = def.wcSupplier30Pct
		wc.SupplierPct60Days = def.wcSupplier60Pct
	}
	if def.wcNoInventory {
		wc.InventoryPctYear1 = decimal.Zero
		wc.InventoryPctYear2 = decimal.Zero
		wc.InventoryPctYear3 = decimal.Zero
		wc.InventoryPctYear4 = decimal.Zero
		wc.InventoryPctYear5 = decimal.Zero
	}
	if err := s.repos.Settings.UpsertWCConfig(wc); err != nil {
		return err
	}
	s.auditCreate(tenantID, userID, scenario.ID, "wc_config", map[string]any{
		"customerPct30Days": wc.CustomerPct30Days,
		"customerPct60Days": wc.CustomerPct60Days,
		"supplierPct30Days": wc.SupplierPct30Days,
		"supplierPct60Days": wc.SupplierPct60Days,
		"inventoryPctYear1": wc.InventoryPctYear1,
		"inventoryPctYear2": wc.InventoryPctYear2,
		"inventoryPctYear3": wc.InventoryPctYear3,
		"inventoryPctYear4": wc.InventoryPctYear4,
		"inventoryPctYear5": wc.InventoryPctYear5,
	})
	// Opex-per-hire — standard French SaaS defaults, unless the plan adjusts them.
	oph := defaultOpexPerHire(tenantID, scenario.ID)
	if def.opexPerHire != nil {
		def.opexPerHire(oph)
		if err := s.repos.Settings.UpsertOpexPerHire(oph); err != nil {
			return err
		}
	}
	s.auditCreate(tenantID, userID, scenario.ID, "opex_per_hire", map[string]any{
		"propertyRentals":           oph.PropertyRentals,
		"postageTelecom":            oph.PostageTelecom,
		"suppliesPurchases":         oph.SuppliesPurchases,
		"studiesDocumentation":      oph.StudiesDocumentation,
		"travelTransportation":      oph.TravelTransportation,
		"missionRepresentation":     oph.MissionRepresentation,
		"insuranceCostsPctSales":    oph.InsuranceCostsPctSales,
		"royaltyPaymentsPctSales":   oph.RoyaltyPaymentsPctSales,
		"recruitTrainingPctPayroll": oph.RecruitTrainingPctPayroll,
	})
	// Capex-per-hire — not modelled in demo; both values are zero.
	s.auditCreate(tenantID, userID, scenario.ID, "capex_per_hire", map[string]any{
		"furniturePerHire": 0,
		"itEquipPerHire":   0,
	})

	// 4. Products (revenue model)
	for sortIdx, pd := range def.products {
		if err := s.createDemoProduct(tenantID, userID, scenario.ID, pd, sortIdx); err != nil {
			return err
		}
	}

	// 5. Staff headcounts (batch per scenario)
	headcounts := make([]model.StaffHeadcount, 0, len(def.headcounts)*5)
	for cat, ftes := range def.headcounts {
		for yi := 1; yi <= 5; yi++ {
			headcounts = append(headcounts, model.StaffHeadcount{
				TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:   scenario.ID,
				Category:     cat,
				YearIndex:    yi,
				FTE:          ftes[yi-1],
			})
		}
	}
	if err := s.repos.Staff.BatchUpsertHeadcounts(tenantID, scenario.ID, headcounts); err != nil {
		return err
	}
	hcFTE := make(map[string]any, len(def.headcounts))
	for cat, ftes := range def.headcounts {
		hcFTE[string(cat)] = [5]float64{
			ftes[0].InexactFloat64(), ftes[1].InexactFloat64(),
			ftes[2].InexactFloat64(), ftes[3].InexactFloat64(), ftes[4].InexactFloat64(),
		}
	}
	s.auditCreate(tenantID, userID, scenario.ID, "staff", map[string]any{
		"type":       "headcounts",
		"categories": len(def.headcounts),
		"rows":       len(headcounts),
		"fte":        hcFTE,
	})

	// 6. Staff salaries (batch per scenario)
	salaries := make([]model.StaffSalary, 0, len(def.salaries)*5)
	for cat, monthlies := range def.salaries {
		for yi := 1; yi <= 5; yi++ {
			salaries = append(salaries, model.StaffSalary{
				TenantScoped:       model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:         scenario.ID,
				Category:           cat,
				YearIndex:          yi,
				MonthlyGrossSalary: monthlies[yi-1],
			})
		}
	}
	if err := s.repos.Staff.BatchUpsertSalaries(tenantID, scenario.ID, salaries); err != nil {
		return err
	}
	salMonthly := make(map[string]any, len(def.salaries))
	for cat, monthlies := range def.salaries {
		salMonthly[string(cat)] = [5]float64{
			monthlies[0].InexactFloat64(), monthlies[1].InexactFloat64(),
			monthlies[2].InexactFloat64(), monthlies[3].InexactFloat64(), monthlies[4].InexactFloat64(),
		}
	}
	s.auditCreate(tenantID, userID, scenario.ID, "staff", map[string]any{
		"type":         "salaries",
		"categories":   len(def.salaries),
		"rows":         len(salaries),
		"monthlyGross": salMonthly,
	})

	// 7. Capex entries (batch per scenario)
	capexEntries := make([]model.CapexEntry, 0, len(def.capex)*5)
	for _, ce := range def.capex {
		for yi := 1; yi <= 5; yi++ {
			capexEntries = append(capexEntries, model.CapexEntry{
				TenantScoped:      model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:        scenario.ID,
				Category:          ce.category,
				YearIndex:         yi,
				Amount:            ce.amounts[yi-1],
				DepreciationYears: ce.deprYears,
			})
		}
	}
	if err := s.repos.Capex.BatchUpsert(tenantID, scenario.ID, capexEntries); err != nil {
		return err
	}
	capexDetails := make([]map[string]any, 0, len(def.capex))
	for _, ce := range def.capex {
		capexDetails = append(capexDetails, map[string]any{
			"category":  string(ce.category),
			"deprYears": ce.deprYears,
			"amounts": [5]float64{
				ce.amounts[0].InexactFloat64(), ce.amounts[1].InexactFloat64(),
				ce.amounts[2].InexactFloat64(), ce.amounts[3].InexactFloat64(), ce.amounts[4].InexactFloat64(),
			},
		})
	}
	s.auditCreate(tenantID, userID, scenario.ID, "capex", map[string]any{
		"categories": len(def.capex),
		"rows":       len(capexEntries),
		"entries":    capexDetails,
	})

	// 8. Opex manual entries (batch per scenario)
	opexEntries := make([]model.OpexManualEntry, 0, len(def.opex)*5)
	for _, oe := range def.opex {
		for yi := 1; yi <= 5; yi++ {
			opexEntries = append(opexEntries, model.OpexManualEntry{
				TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
				ScenarioID:   scenario.ID,
				LineID:       oe.lineID,
				YearIndex:    yi,
				Amount:       oe.amounts[yi-1],
			})
		}
	}
	if err := s.repos.Opex.BatchUpsert(tenantID, scenario.ID, opexEntries); err != nil {
		return err
	}
	opexDetails := make([]map[string]any, 0, len(def.opex))
	for _, oe := range def.opex {
		opexDetails = append(opexDetails, map[string]any{
			"lineId": string(oe.lineID),
			"amounts": [5]float64{
				oe.amounts[0].InexactFloat64(), oe.amounts[1].InexactFloat64(),
				oe.amounts[2].InexactFloat64(), oe.amounts[3].InexactFloat64(), oe.amounts[4].InexactFloat64(),
			},
		})
	}
	s.auditCreate(tenantID, userID, scenario.ID, "opex", map[string]any{
		"lines":   len(def.opex),
		"rows":    len(opexEntries),
		"entries": opexDetails,
	})

	// 9. Staff incentives — variable-pay fractions per category (optional)
	if len(def.incentiveRates) > 0 {
		ratesMap := make(map[string]any, len(def.incentiveRates))
		for cat, rate := range def.incentiveRates {
			ratesMap[string(cat)] = rate.InexactFloat64()
		}
		s.auditCreate(tenantID, userID, scenario.ID, "staff_incentives", map[string]any{
			"note":       "target-bonus rates seeded (fraction of annual base salary)",
			"categories": len(def.incentiveRates),
			"rates":      ratesMap,
		})
	} else {
		s.auditCreate(tenantID, userID, scenario.ID, "staff_incentives", map[string]any{
			"note": "seeded at defaults (no variable pay model)",
			"rows": 0,
		})
	}

	// 10. Financing plan entries (optional — only for plans with external funding)
	if len(def.fiplanEntries) > 0 {
		fiplanRows := make([]model.FiplanEntry, 0, len(def.fiplanEntries)*5)
		for _, fe := range def.fiplanEntries {
			for yi := 1; yi <= 5; yi++ {
				fiplanRows = append(fiplanRows, model.FiplanEntry{
					TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
					ScenarioID:   scenario.ID,
					LineID:       fe.lineID,
					YearIndex:    yi,
					Amount:       fe.amounts[yi-1],
				})
			}
		}
		if err := s.repos.FiPlan.BatchUpsert(tenantID, scenario.ID, fiplanRows); err != nil {
			return err
		}
		fiplanDetails := make([]map[string]any, 0, len(def.fiplanEntries))
		for _, fe := range def.fiplanEntries {
			fiplanDetails = append(fiplanDetails, map[string]any{
				"lineId": string(fe.lineID),
				"amounts": [5]float64{
					fe.amounts[0].InexactFloat64(), fe.amounts[1].InexactFloat64(),
					fe.amounts[2].InexactFloat64(), fe.amounts[3].InexactFloat64(), fe.amounts[4].InexactFloat64(),
				},
			})
		}
		s.auditCreate(tenantID, userID, scenario.ID, "fiplan", map[string]any{
			"lines":   len(def.fiplanEntries),
			"rows":    len(fiplanRows),
			"entries": fiplanDetails,
		})
	} else {
		s.auditCreate(tenantID, userID, scenario.ID, "fiplan", map[string]any{
			"note": "no financing plan entries — no external funding modelled in demo",
		})
	}

	// 11. Computed sections (P&L, Balance Sheet, Cash, WCR, Budget)
	//     are fully derived from the inputs above — no manual overrides seeded.
	s.auditCreate(tenantID, userID, scenario.ID, "pnl", map[string]any{
		"note": "no manual P&L adjustments — fully computed from revenue and cost inputs",
	})
	s.auditCreate(tenantID, userID, scenario.ID, "balance_sheet", map[string]any{
		"note": "no manual BS adjustments — fully derived from P&L, capex, and WCR",
	})
	s.auditCreate(tenantID, userID, scenario.ID, "wcr", map[string]any{
		"note": "working capital computed from WC config defaults and revenue inputs",
	})
	s.auditCreate(tenantID, userID, scenario.ID, "cash", map[string]any{
		"note": "cash flow computed — no manual monthly overrides",
	})
	s.auditCreate(tenantID, userID, scenario.ID, "budget", map[string]any{
		"note": "no budget overrides — all monthly figures are model-derived",
	})

	return nil
}

func (s *SeedService) createDemoProduct(tenantID, userID, scenarioID uuid.UUID, pd demoProduct, sortOrder int) error {
	driverType := pd.driverType
	if driverType == "" {
		driverType = model.DriverGeneric
	}

	var driverParams json.RawMessage
	if pd.driverParams != nil {
		b, err := json.Marshal(pd.driverParams)
		if err != nil {
			return err
		}
		driverParams = b
	}

	product := &model.Product{
		TenantScoped:              model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		ScenarioID:                scenarioID,
		Name:                      pd.name,
		ProductType:               pd.pType,
		SortOrder:                 sortOrder,
		DirectCostVariability:     d(1),
		ExternalChargeVariability: d(1),
		TaxVariability:            d(1),
		StaffVariability:          d(1),
		DepreciationVariability:   d(1),
		DriverType:                driverType,
		DriverParams:              driverParams,
	}
	if err := s.repos.Product.CreateProduct(product); err != nil {
		return err
	}
	// Use scenarioID as entity_id so this event appears in the scenario audit trail.
	s.auditCreate(tenantID, userID, scenarioID, "product", map[string]any{
		"name":        pd.name,
		"productType": string(pd.pType),
		"driverType":  string(driverType),
		"_entityId":   product.ID.String(),
	})

	// One assumption row per year
	assumptions := make([]model.ProductAssumption, 5)
	for yi := 1; yi <= 5; yi++ {
		assumptions[yi-1] = model.ProductAssumption{
			TenantScoped:     model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ProductID:        product.ID,
			YearIndex:        yi,
			RawMaterialCost:  pd.cogs[yi-1],
			CostCoefficient:  d(1),
			PriceCoefficient: d(1),
			BaseUnitPrice:    pd.prices[yi-1],
		}
	}
	if err := s.repos.Product.BatchUpsertAssumptions(tenantID, scenarioID, assumptions); err != nil {
		return err
	}
	s.auditCreate(tenantID, userID, scenarioID, "product_assumptions", map[string]any{
		"product":   pd.name,
		"rows":      len(assumptions),
		"_entityId": product.ID.String(),
		"prices": [5]float64{
			pd.prices[0].InexactFloat64(), pd.prices[1].InexactFloat64(),
			pd.prices[2].InexactFloat64(), pd.prices[3].InexactFloat64(), pd.prices[4].InexactFloat64(),
		},
		"cogs": [5]float64{
			pd.cogs[0].InexactFloat64(), pd.cogs[1].InexactFloat64(),
			pd.cogs[2].InexactFloat64(), pd.cogs[3].InexactFloat64(), pd.cogs[4].InexactFloat64(),
		},
	})

	// One volume row per year (direct / France channel)
	volumes := make([]model.ProductSalesVolume, 5)
	for yi := 1; yi <= 5; yi++ {
		volumes[yi-1] = model.ProductSalesVolume{
			TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			ProductID:    product.ID,
			YearIndex:    yi,
			Zone:         model.ZoneFrance,
			Channel:      model.ChannelDirect,
			UnitsSold:    pd.units[yi-1],
		}
	}
	if err := s.repos.Product.BatchUpsertVolumes(tenantID, scenarioID, volumes); err != nil {
		return err
	}
	s.auditCreate(tenantID, userID, scenarioID, "product_volumes", map[string]any{
		"product":   pd.name,
		"rows":      len(volumes),
		"zone":      string(model.ZoneFrance),
		"channel":   string(model.ChannelDirect),
		"_entityId": product.ID.String(),
		"units":     [5]int64{pd.units[0], pd.units[1], pd.units[2], pd.units[3], pd.units[4]},
	})

	// Expose driver-specific parameters that are invisible in the generic
	// product_assumptions / product_volumes entries.
	// ── SaaS: growth-model inputs (churn, expansion, active users) ──────────
	if sp, ok := pd.driverParams.(model.SaaSParams); ok {
		var monthlyFeeF, churnRateF, expansionRateF [5]float64
		for i := 0; i < 5; i++ {
			monthlyFeeF[i] = sp.MonthlyFee[i].InexactFloat64()
			churnRateF[i] = sp.ChurnRate[i].InexactFloat64()
			expansionRateF[i] = sp.ExpansionRate[i].InexactFloat64()
		}
		s.auditCreate(tenantID, userID, scenarioID, "product_driver_params", map[string]any{
			"product":       pd.name,
			"_entityId":     product.ID.String(),
			"driver":        "saas",
			"activeUsers":   [5]int64{int64(sp.ActiveUsers[0]), int64(sp.ActiveUsers[1]), int64(sp.ActiveUsers[2]), int64(sp.ActiveUsers[3]), int64(sp.ActiveUsers[4])},
			"monthlyFee":    monthlyFeeF,
			"churnRate":     churnRateF,
			"expansionRate": expansionRateF,
		})
	}
	// ── Industry: production constraints and unit cost drivers ───────────────
	if ip, ok := pd.driverParams.(model.IndustryParams); ok {
		var scrapRateF, setupCostF [5]float64
		for i := 0; i < 5; i++ {
			scrapRateF[i] = ip.ScrapRate[i].InexactFloat64()
			setupCostF[i] = ip.SetupCost[i].InexactFloat64()
		}
		s.auditCreate(tenantID, userID, scenarioID, "product_driver_params", map[string]any{
			"product":            pd.name,
			"_entityId":          product.ID.String(),
			"driver":             "industry",
			"productionCapacity": [5]int64{int64(ip.ProductionCapacity[0]), int64(ip.ProductionCapacity[1]), int64(ip.ProductionCapacity[2]), int64(ip.ProductionCapacity[3]), int64(ip.ProductionCapacity[4])},
			"scrapRate":          scrapRateF,
			"setupCost":          setupCostF,
		})
	}
	// ── Consulting: FTE → available days → billable days → revenue chain ─────
	// Delivery headcount is embedded in COGS and invisible in the staff table,
	// so this audit entry is the only place the workforce structure is recorded.
	if cp, ok := pd.driverParams.(model.ConsultingParams); ok {
		var headcountF, utilizationF, availableF, billableF, monthlyGrossF [5]float64
		for i := 0; i < 5; i++ {
			headcountF[i] = cp.Headcount[i].InexactFloat64()
			utilizationF[i] = cp.UtilizationRate[i].InexactFloat64()
			availableF[i] = headcountF[i] * float64(cp.WorkingDays)
			billableF[i] = availableF[i] * utilizationF[i]
			monthlyGrossF[i] = cp.MonthlyGross[i].InexactFloat64()
		}
		s.auditCreate(tenantID, userID, scenarioID, "product_workforce", map[string]any{
			"product":         pd.name,
			"_entityId":       product.ID.String(),
			"headcount":       headcountF,
			"workingDays":     cp.WorkingDays,
			"utilizationRate": utilizationF,
			"availableDays":   availableF,
			"billableDays":    billableF,
			"monthlyGross":    monthlyGrossF,
			"employerCharges": cp.EmployerCharges.InexactFloat64(),
		})
	}

	return nil
}
