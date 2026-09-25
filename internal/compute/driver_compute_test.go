package compute

import (
	"encoding/json"
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// planCfg returns a minimal PlanConfig for driver compute tests.
func planCfg() model.PlanConfig {
	return model.PlanConfig{ForecastStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// emptyBundle returns a blank ProductInputBundle with identity coefficients.
func emptyBundle() ProductInputBundle {
	b := ProductInputBundle{}
	for i := 0; i < MaxYears; i++ {
		b.Assumptions[i].CostCoefficient = decimal.NewFromInt(1)
		b.Assumptions[i].PriceCoefficient = decimal.NewFromInt(1)
	}
	return b
}

// driverProduct builds a Product with encoded DriverParams.
func driverProduct(dt model.DriverType, params any) model.Product {
	raw, err := json.Marshal(params)
	if err != nil {
		panic(err)
	}
	return model.Product{
		TenantScoped: model.TenantScoped{ID: uuid.New()},
		DriverType:   dt,
		DriverParams: raw,
	}
}

// ─── Generic pass-through ──────────────────────────────────────────────────

func TestApplyDriverCompute_Generic_Unchanged(t *testing.T) {
	product := model.Product{DriverType: model.DriverGeneric}
	original := emptyBundle()
	original.Assumptions[0].BaseUnitPrice = d("500")
	original.Volumes = []model.ProductSalesVolume{
		{YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 100},
	}

	got, err := ApplyDriverCompute(product, original)
	require.NoError(t, err)
	assert.Equal(t, int64(100), got.Volumes[0].UnitsSold)
	assert.True(t, d("500").Equal(got.Assumptions[0].BaseUnitPrice))
}

func TestApplyDriverCompute_EmptyDriverType_Unchanged(t *testing.T) {
	product := model.Product{} // DriverType == ""
	original := emptyBundle()
	original.Assumptions[0].BaseUnitPrice = d("999")

	got, err := ApplyDriverCompute(product, original)
	require.NoError(t, err)
	assert.True(t, d("999").Equal(got.Assumptions[0].BaseUnitPrice))
}

func TestApplyDriverCompute_NilParams_FallsThrough(t *testing.T) {
	product := model.Product{DriverType: model.DriverSaaS} // params nil
	got, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)
	assert.Nil(t, got.Volumes) // unchanged
}

// ─── Consulting driver ────────────────────────────────────────────────────

func TestApplyDriverCompute_Consulting_VolumesAndCosts(t *testing.T) {
	// 1 FTE, 220 working days, 85% utilisation → 187 billable days
	// Monthly gross 9 000, employer 1.45
	// CostPerDay = (1 × 9000 × 12 × 1.45) / 187 = 156540 / 187 ≈ 837.43
	params := model.ConsultingParams{
		Headcount:       [5]decimal.Decimal{d("1"), d("1"), d("1"), d("1"), d("1")},
		WorkingDays:     220,
		UtilizationRate: [5]decimal.Decimal{d("0.85"), d("0.85"), d("0.85"), d("0.85"), d("0.85")},
		MonthlyGross:    [5]decimal.Decimal{d("9000"), d("9000"), d("9000"), d("9000"), d("9000")},
		EmployerCharges: d("1.45"),
	}
	product := driverProduct(model.DriverConsulting, params)
	// Set a billing rate on Y1 assumption (user-set, must survive the override).
	bundle := emptyBundle()
	bundle.Assumptions[0].BaseUnitPrice = d("2000") // €/day billing rate

	got, err := ApplyDriverCompute(product, bundle)
	require.NoError(t, err)

	// Billable days: 1 × 220 × 0.85 = 187
	assert.Equal(t, int64(187), got.Volumes[0].UnitsSold)
	assert.Equal(t, MaxYears, len(got.Volumes))
	assert.Equal(t, model.ZoneFrance, got.Volumes[0].Zone)
	assert.Equal(t, model.ChannelDirect, got.Volumes[0].Channel)

	// CostPerDay ≈ 837.43 — allow 0.01 tolerance for decimal division
	expectedCost := d("837.43")
	diff := got.Assumptions[0].RawMaterialCost.Sub(expectedCost).Abs()
	assert.True(t, diff.LessThan(d("0.01")), "cost/day: got %s", got.Assumptions[0].RawMaterialCost)

	// Billing rate (BaseUnitPrice) must be preserved.
	assert.True(t, d("2000").Equal(got.Assumptions[0].BaseUnitPrice), "billing rate must survive override")

	// RoyaltiesCost and LogisticsCost must be zeroed.
	assert.True(t, decimal.Zero.Equal(got.Assumptions[0].RoyaltiesCost))
	assert.True(t, decimal.Zero.Equal(got.Assumptions[0].LogisticsCost))
}

// End-to-end: consulting driver produces correct revenue and COGS.
func TestApplyDriverCompute_Consulting_RevenueE2E(t *testing.T) {
	// 1 consultant, 220 days, 100% utilisation — keeps the math simple.
	// Billing rate = 1500 €/day → Revenue = 220 × 1500 = 330 000
	// Cost/day = 9000 × 12 × 1.45 / 220 = 156600 / 220 = 711.82
	// COGS     = 220 × cost/day = 9000 × 12 × 1.45 = 156 600   (220s cancel exactly)
	params := model.ConsultingParams{
		Headcount:       [5]decimal.Decimal{d("1"), d("1"), d("1"), d("1"), d("1")},
		WorkingDays:     220,
		UtilizationRate: [5]decimal.Decimal{d("1"), d("1"), d("1"), d("1"), d("1")},
		MonthlyGross:    [5]decimal.Decimal{d("9000"), d("9000"), d("9000"), d("9000"), d("9000")},
		EmployerCharges: d("1.45"),
	}
	product := driverProduct(model.DriverConsulting, params)
	bundle := emptyBundle()
	for i := 0; i < MaxYears; i++ {
		bundle.Assumptions[i].BaseUnitPrice = d("1500") // billing rate
	}

	derived, err := ApplyDriverCompute(product, bundle)
	require.NoError(t, err)

	cfg := planCfg()
	result := ComputeProductRevenue(product, derived, cfg)

	for y := 0; y < MaxYears; y++ {
		// Revenue = 220 days × 1500 = 330 000
		assert.True(t, d("330000").Equal(result.Years[y].Turnover),
			"Y%d turnover: got %s", y+1, result.Years[y].Turnover)

		// COGS = 220 × (9000 × 12 × 1.45 / 220) = 9000 × 12 × 1.45 = 156 600
		expectedCOGS := d("156600")
		diff := result.Years[y].COGS.Sub(expectedCOGS).Abs()
		assert.True(t, diff.LessThan(d("1")),
			"Y%d COGS: got %s expected ~%s", y+1, result.Years[y].COGS, expectedCOGS)
	}
}

// ─── SaaS driver ──────────────────────────────────────────────────────────

func TestApplyDriverCompute_SaaS_VolumeAndEconomics(t *testing.T) {
	params := model.SaaSParams{
		ActiveUsers:    [5]model.FlexInt64{1000, 2000, 3000, 4000, 5000},
		MonthlyFee:     [5]decimal.Decimal{d("49"), d("49"), d("55"), d("55"), d("60")},
		InfraCostPPU:   [5]decimal.Decimal{d("3"), d("3"), d("2.5"), d("2.5"), d("2")},
		SupportCostPPU: [5]decimal.Decimal{d("2"), d("2"), d("1.5"), d("1.5"), d("1")},
	}
	product := driverProduct(model.DriverSaaS, params)

	got, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	// Y1: 1000 users, MonthlyFee = 49 → AnnualFee = 588 per user
	assert.Equal(t, int64(1000), got.Volumes[0].UnitsSold)
	assert.True(t, d("588").Equal(got.Assumptions[0].BaseUnitPrice), // 49 × 12
		"Y1 annual fee: got %s", got.Assumptions[0].BaseUnitPrice)

	// Y1: InfraCost + SupportCost = 3 + 2 = 5 → × 12 = 60
	assert.True(t, d("60").Equal(got.Assumptions[0].RawMaterialCost),
		"Y1 annual cost per user: got %s", got.Assumptions[0].RawMaterialCost)

	// Y3: users=3000, fee=55×12=660, cost=(2.5+1.5)×12=48
	assert.Equal(t, int64(3000), got.Volumes[2].UnitsSold)
	assert.True(t, d("660").Equal(got.Assumptions[2].BaseUnitPrice))
	assert.True(t, d("48").Equal(got.Assumptions[2].RawMaterialCost))
}

// End-to-end SaaS revenue check.
func TestApplyDriverCompute_SaaS_RevenueE2E(t *testing.T) {
	// Y1: 500 users × 49/month × 12 = 294 000 revenue
	// Y1: COGS  = 500 × (3+2) × 12 = 30 000
	params := model.SaaSParams{
		ActiveUsers:    [5]model.FlexInt64{500, 500, 500, 500, 500},
		MonthlyFee:     [5]decimal.Decimal{d("49"), d("49"), d("49"), d("49"), d("49")},
		InfraCostPPU:   [5]decimal.Decimal{d("3"), d("3"), d("3"), d("3"), d("3")},
		SupportCostPPU: [5]decimal.Decimal{d("2"), d("2"), d("2"), d("2"), d("2")},
	}
	product := driverProduct(model.DriverSaaS, params)

	derived, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	result := ComputeProductRevenue(product, derived, planCfg())

	for y := 0; y < MaxYears; y++ {
		assert.True(t, d("294000").Equal(result.Years[y].Turnover),
			"Y%d turnover: got %s", y+1, result.Years[y].Turnover)
		assert.True(t, d("30000").Equal(result.Years[y].COGS),
			"Y%d COGS: got %s", y+1, result.Years[y].COGS)
	}
}

// ─── Industry driver ──────────────────────────────────────────────────────

func TestApplyDriverCompute_Industry_ScrapAndSetup(t *testing.T) {
	// Base unit cost = 100 (raw material), scrap rate = 5%, setup = 10 000/year
	// Units sold = 1000 → effective cost = 100/(1-0.05) + 10000/1000
	//                                     = 105.26... + 10 = 115.26...
	params := model.IndustryParams{
		ProductionCapacity: [5]model.FlexInt64{5000, 5000, 5000, 5000, 5000},
		ScrapRate:          [5]decimal.Decimal{d("0.05"), d("0.05"), d("0.05"), d("0.05"), d("0.05")},
		SetupCost:          [5]decimal.Decimal{d("10000"), d("10000"), d("10000"), d("10000"), d("10000")},
	}
	product := driverProduct(model.DriverIndustry, params)

	bundle := emptyBundle()
	for i := 0; i < MaxYears; i++ {
		bundle.Assumptions[i].RawMaterialCost = d("100")
		bundle.Assumptions[i].BaseUnitPrice = d("200")
	}
	bundle.Volumes = []model.ProductSalesVolume{
		{YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
		{YearIndex: 2, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
		{YearIndex: 3, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
		{YearIndex: 4, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
		{YearIndex: 5, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 1000},
	}

	got, err := ApplyDriverCompute(product, bundle)
	require.NoError(t, err)

	// Volumes unchanged for industry driver.
	assert.Equal(t, 5, len(got.Volumes))
	assert.Equal(t, int64(1000), got.Volumes[0].UnitsSold)

	// Effective cost = 100 / 0.95 + 10000 / 1000 = 105.263... + 10 = 115.263...
	expected := d("115.263")
	diff := got.Assumptions[0].RawMaterialCost.Sub(expected).Abs()
	assert.True(t, diff.LessThan(d("0.001")),
		"effective unit cost: got %s", got.Assumptions[0].RawMaterialCost.StringFixed(3))

	// Selling price untouched.
	assert.True(t, d("200").Equal(got.Assumptions[0].BaseUnitPrice))
}

func TestApplyDriverCompute_Industry_ZeroScrapAndSetup(t *testing.T) {
	// With scrap=0 and setup=0, cost should be unchanged.
	params := model.IndustryParams{
		ScrapRate: [5]decimal.Decimal{d("0"), d("0"), d("0"), d("0"), d("0")},
		SetupCost: [5]decimal.Decimal{d("0"), d("0"), d("0"), d("0"), d("0")},
	}
	product := driverProduct(model.DriverIndustry, params)

	bundle := emptyBundle()
	bundle.Assumptions[0].RawMaterialCost = d("50")
	bundle.Volumes = []model.ProductSalesVolume{
		{YearIndex: 1, Zone: model.ZoneFrance, Channel: model.ChannelDirect, UnitsSold: 500},
	}

	got, err := ApplyDriverCompute(product, bundle)
	require.NoError(t, err)

	// No adjustment: 50 / (1-0) + 0/500 = 50
	assert.True(t, d("50").Equal(got.Assumptions[0].RawMaterialCost))
}

// ─── Marketplace driver ───────────────────────────────────────────────────

func TestApplyDriverCompute_Marketplace_Economics(t *testing.T) {
	// 10 000 transactions, GMV=85, take rate=12% → net revenue = 10.20/tx
	// PaymentCost = 1.50, FixedInfra = 24 000 → 24000/10000 = 2.40/tx
	// Total cost/tx = 1.50 + 2.40 = 3.90
	params := model.MarketplaceParams{
		Transactions:      [5]model.FlexInt64{10000, 10000, 10000, 10000, 10000},
		GMVPerTransaction: [5]decimal.Decimal{d("85"), d("85"), d("85"), d("85"), d("85")},
		TakeRate:          [5]decimal.Decimal{d("0.12"), d("0.12"), d("0.12"), d("0.12"), d("0.12")},
		PaymentCost:       [5]decimal.Decimal{d("1.50"), d("1.50"), d("1.50"), d("1.50"), d("1.50")},
		FixedInfraCost:    [5]decimal.Decimal{d("24000"), d("24000"), d("24000"), d("24000"), d("24000")},
	}
	product := driverProduct(model.DriverMarketplace, params)

	got, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	assert.Equal(t, int64(10000), got.Volumes[0].UnitsSold)

	// Net revenue per tx = 85 × 0.12 = 10.20
	assert.True(t, d("10.20").Equal(got.Assumptions[0].BaseUnitPrice),
		"net revenue/tx: got %s", got.Assumptions[0].BaseUnitPrice)

	// Cost/tx = 1.50 + 2.40 = 3.90
	assert.True(t, d("3.90").Equal(got.Assumptions[0].RawMaterialCost),
		"cost/tx: got %s", got.Assumptions[0].RawMaterialCost)
}

// End-to-end marketplace revenue.
func TestApplyDriverCompute_Marketplace_RevenueE2E(t *testing.T) {
	// 10 000 tx × 10.20 net = 102 000 revenue
	// 10 000 tx × 3.90 cost = 39 000 COGS
	params := model.MarketplaceParams{
		Transactions:      [5]model.FlexInt64{10000, 10000, 10000, 10000, 10000},
		GMVPerTransaction: [5]decimal.Decimal{d("85"), d("85"), d("85"), d("85"), d("85")},
		TakeRate:          [5]decimal.Decimal{d("0.12"), d("0.12"), d("0.12"), d("0.12"), d("0.12")},
		PaymentCost:       [5]decimal.Decimal{d("1.50"), d("1.50"), d("1.50"), d("1.50"), d("1.50")},
		FixedInfraCost:    [5]decimal.Decimal{d("24000"), d("24000"), d("24000"), d("24000"), d("24000")},
	}
	product := driverProduct(model.DriverMarketplace, params)

	derived, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	result := ComputeProductRevenue(product, derived, planCfg())

	for y := 0; y < MaxYears; y++ {
		assert.True(t, d("102000").Equal(result.Years[y].Turnover),
			"Y%d turnover: got %s", y+1, result.Years[y].Turnover)
		assert.True(t, d("39000").Equal(result.Years[y].COGS),
			"Y%d COGS: got %s", y+1, result.Years[y].COGS)
	}
}

// ─── Media driver ────────────────────────────────────────────────────────

func TestApplyDriverCompute_Media_PerMilleEconomics(t *testing.T) {
	// 10 000 000 impressions / 1000 = 10 000 per-mille blocks
	// CPM = 4.00, FillRate = 0.75 → revenue/mille = 3.00
	// DeliveryCostPerImpression = 0.0002 → delivery/mille = 0.20
	// ContentCost = 50 000 / 10 000 blocks = 5.00/mille
	// CostPerMille = 0.20 + 5.00 = 5.20
	params := model.MediaParams{
		Impressions:               [5]model.FlexInt64{10_000_000, 10_000_000, 10_000_000, 10_000_000, 10_000_000},
		CPM:                       [5]decimal.Decimal{d("4.00"), d("4.00"), d("4.00"), d("4.00"), d("4.00")},
		FillRate:                  [5]decimal.Decimal{d("0.75"), d("0.75"), d("0.75"), d("0.75"), d("0.75")},
		ContentCost:               [5]decimal.Decimal{d("50000"), d("50000"), d("50000"), d("50000"), d("50000")},
		DeliveryCostPerImpression: [5]decimal.Decimal{d("0.0002"), d("0.0002"), d("0.0002"), d("0.0002"), d("0.0002")},
	}
	product := driverProduct(model.DriverMedia, params)

	got, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	// Volume = 10 000 000 / 1000 = 10 000 per-mille blocks
	assert.Equal(t, int64(10000), got.Volumes[0].UnitsSold)

	// Revenue per mille = 4.00 × 0.75 = 3.00
	assert.True(t, d("3.00").Equal(got.Assumptions[0].BaseUnitPrice),
		"revenue/mille: got %s", got.Assumptions[0].BaseUnitPrice)

	// Cost per mille = 0.0002 × 1000 + 50000 / 10000 = 0.20 + 5.00 = 5.20
	assert.True(t, d("5.20").Equal(got.Assumptions[0].RawMaterialCost),
		"cost/mille: got %s", got.Assumptions[0].RawMaterialCost)
}

// End-to-end media revenue: 10 000 mille × 3.00 = 30 000 revenue
// COGS: 10 000 × 5.20 = 52 000
func TestApplyDriverCompute_Media_RevenueE2E(t *testing.T) {
	params := model.MediaParams{
		Impressions:               [5]model.FlexInt64{10_000_000, 10_000_000, 10_000_000, 10_000_000, 10_000_000},
		CPM:                       [5]decimal.Decimal{d("4.00"), d("4.00"), d("4.00"), d("4.00"), d("4.00")},
		FillRate:                  [5]decimal.Decimal{d("0.75"), d("0.75"), d("0.75"), d("0.75"), d("0.75")},
		ContentCost:               [5]decimal.Decimal{d("50000"), d("50000"), d("50000"), d("50000"), d("50000")},
		DeliveryCostPerImpression: [5]decimal.Decimal{d("0.0002"), d("0.0002"), d("0.0002"), d("0.0002"), d("0.0002")},
	}
	product := driverProduct(model.DriverMedia, params)

	derived, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	result := ComputeProductRevenue(product, derived, planCfg())

	for y := 0; y < MaxYears; y++ {
		assert.True(t, d("30000").Equal(result.Years[y].Turnover),
			"Y%d turnover: got %s", y+1, result.Years[y].Turnover)
		assert.True(t, d("52000").Equal(result.Years[y].COGS),
			"Y%d COGS: got %s", y+1, result.Years[y].COGS)
	}
}

// ─── Session-Based driver ─────────────────────────────────────────────────

func TestApplyDriverCompute_SessionBased_VolumeAndEconomics(t *testing.T) {
	// Setup (Y1, all years identical):
	//   3 trainers × 40 sessions/trainer × 0.90 util = 108 capacity
	//   Planned sessions = 80 → NOT clamped (80 < 108) → ActualSessions = 80
	//   ParticipantsPerSession = 20, FillRate = 0.75 → Realized = 15
	//   PricePerParticipant = 150 → UnitPrice = 15 × 150 = 2 250
	//   TrainerCostPerSession = 400, VariableCostPerParticipant = 15
	//   → UnitCost = 400 + 15 × 15 = 625
	params := model.SessionBasedParams{
		Sessions:                   [5]model.FlexInt64{80, 80, 80, 80, 80},
		ParticipantsPerSession:     [5]decimal.Decimal{d("20"), d("20"), d("20"), d("20"), d("20")},
		FillRate:                   [5]decimal.Decimal{d("0.75"), d("0.75"), d("0.75"), d("0.75"), d("0.75")},
		PricePerParticipant:        [5]decimal.Decimal{d("150"), d("150"), d("150"), d("150"), d("150")},
		TrainerCount:               [5]decimal.Decimal{d("3"), d("3"), d("3"), d("3"), d("3")},
		SessionsPerTrainer:         [5]model.FlexInt64{40, 40, 40, 40, 40},
		UtilizationRate:            [5]decimal.Decimal{d("0.90"), d("0.90"), d("0.90"), d("0.90"), d("0.90")},
		TrainerCostPerSession:      [5]decimal.Decimal{d("400"), d("400"), d("400"), d("400"), d("400")},
		VariableCostPerParticipant: [5]decimal.Decimal{d("15"), d("15"), d("15"), d("15"), d("15")},
	}
	product := driverProduct(model.DriverSessionBased, params)

	got, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	// Volume = ActualSessions = 80 (not clamped)
	assert.Equal(t, MaxYears, len(got.Volumes))
	assert.Equal(t, int64(80), got.Volumes[0].UnitsSold, "Y1 volume not clamped")
	assert.Equal(t, model.ZoneFrance, got.Volumes[0].Zone)
	assert.Equal(t, model.ChannelDirect, got.Volumes[0].Channel)

	// UnitPrice = 20 × 0.75 × 150 = 2 250
	assert.True(t, d("2250").Equal(got.Assumptions[0].BaseUnitPrice),
		"Y1 unit price: got %s", got.Assumptions[0].BaseUnitPrice)

	// UnitCost = 400 + 15 × (20 × 0.75) = 400 + 225 = 625
	assert.True(t, d("625").Equal(got.Assumptions[0].RawMaterialCost),
		"Y1 unit cost: got %s", got.Assumptions[0].RawMaterialCost)

	// Side-costs zeroed, coefficients = 1
	assert.True(t, decimal.Zero.Equal(got.Assumptions[0].RoyaltiesCost))
	assert.True(t, decimal.Zero.Equal(got.Assumptions[0].LogisticsCost))
	assert.True(t, d("1").Equal(got.Assumptions[0].CostCoefficient))
	assert.True(t, d("1").Equal(got.Assumptions[0].PriceCoefficient))
}

func TestApplyDriverCompute_SessionBased_CapacityClamp(t *testing.T) {
	// 2 trainers × 30 sessions/trainer × 0.80 util = 48 capacity
	// Planned = 80 → CLAMPED to 48 (IntPart of 48.0)
	// Realized participants = 20 × 0.75 = 15
	// UnitPrice = 15 × 200 = 3 000
	// UnitCost = 500 + 20 × 15 = 800
	params := model.SessionBasedParams{
		Sessions:                   [5]model.FlexInt64{80, 80, 80, 80, 80},
		ParticipantsPerSession:     [5]decimal.Decimal{d("20"), d("20"), d("20"), d("20"), d("20")},
		FillRate:                   [5]decimal.Decimal{d("0.75"), d("0.75"), d("0.75"), d("0.75"), d("0.75")},
		PricePerParticipant:        [5]decimal.Decimal{d("200"), d("200"), d("200"), d("200"), d("200")},
		TrainerCount:               [5]decimal.Decimal{d("2"), d("2"), d("2"), d("2"), d("2")},
		SessionsPerTrainer:         [5]model.FlexInt64{30, 30, 30, 30, 30},
		UtilizationRate:            [5]decimal.Decimal{d("0.80"), d("0.80"), d("0.80"), d("0.80"), d("0.80")},
		TrainerCostPerSession:      [5]decimal.Decimal{d("500"), d("500"), d("500"), d("500"), d("500")},
		VariableCostPerParticipant: [5]decimal.Decimal{d("20"), d("20"), d("20"), d("20"), d("20")},
	}
	product := driverProduct(model.DriverSessionBased, params)

	got, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	// Volume must be clamped: 2 × 30 × 0.80 = 48
	assert.Equal(t, int64(48), got.Volumes[0].UnitsSold, "sessions must be clamped to trainer capacity")

	// Economics still computed on the (correct) realized participants, not on clamped sessions
	// UnitPrice = 20 × 0.75 × 200 = 3 000
	assert.True(t, d("3000").Equal(got.Assumptions[0].BaseUnitPrice),
		"unit price: got %s", got.Assumptions[0].BaseUnitPrice)

	// UnitCost = 500 + 20 × (20 × 0.75) = 500 + 300 = 800
	assert.True(t, d("800").Equal(got.Assumptions[0].RawMaterialCost),
		"unit cost: got %s", got.Assumptions[0].RawMaterialCost)
}

// End-to-end: session-based driver produces correct revenue and COGS.
//
//	5 trainers × 30 sessions × 0.90 util = 135 capacity → 60 planned is not clamped
//	RealizedParticipants = 25 × 0.80 = 20
//	UnitPrice = 20 × 100 = 2 000   → Revenue = 60 × 2 000 = 120 000
//	UnitCost  = 300 + 10 × 20 = 500 → COGS    = 60 × 500  =  30 000
func TestApplyDriverCompute_SessionBased_RevenueE2E(t *testing.T) {
	params := model.SessionBasedParams{
		Sessions:                   [5]model.FlexInt64{60, 60, 60, 60, 60},
		ParticipantsPerSession:     [5]decimal.Decimal{d("25"), d("25"), d("25"), d("25"), d("25")},
		FillRate:                   [5]decimal.Decimal{d("0.80"), d("0.80"), d("0.80"), d("0.80"), d("0.80")},
		PricePerParticipant:        [5]decimal.Decimal{d("100"), d("100"), d("100"), d("100"), d("100")},
		TrainerCount:               [5]decimal.Decimal{d("5"), d("5"), d("5"), d("5"), d("5")},
		SessionsPerTrainer:         [5]model.FlexInt64{30, 30, 30, 30, 30},
		UtilizationRate:            [5]decimal.Decimal{d("0.90"), d("0.90"), d("0.90"), d("0.90"), d("0.90")},
		TrainerCostPerSession:      [5]decimal.Decimal{d("300"), d("300"), d("300"), d("300"), d("300")},
		VariableCostPerParticipant: [5]decimal.Decimal{d("10"), d("10"), d("10"), d("10"), d("10")},
	}
	product := driverProduct(model.DriverSessionBased, params)

	derived, err := ApplyDriverCompute(product, emptyBundle())
	require.NoError(t, err)

	result := ComputeProductRevenue(product, derived, planCfg())

	for y := 0; y < MaxYears; y++ {
		// Revenue = 60 sessions × (25 × 0.80 × 100) = 60 × 2 000 = 120 000
		assert.True(t, d("120000").Equal(result.Years[y].Turnover),
			"Y%d turnover: got %s", y+1, result.Years[y].Turnover)

		// COGS = 60 sessions × (300 + 10 × 20) = 60 × 500 = 30 000
		assert.True(t, d("30000").Equal(result.Years[y].COGS),
			"Y%d COGS: got %s", y+1, result.Years[y].COGS)
	}
}

// ─── Bad JSON ─────────────────────────────────────────────────────────────

func TestApplyDriverCompute_BadJSON_ReturnsError(t *testing.T) {
	product := model.Product{
		DriverType:   model.DriverSaaS,
		DriverParams: []byte(`{not valid json`),
	}
	_, err := ApplyDriverCompute(product, emptyBundle())
	assert.Error(t, err)
}

func TestApplyDriverCompute_SessionBased_BadJSON_ReturnsError(t *testing.T) {
	product := model.Product{
		DriverType:   model.DriverSessionBased,
		DriverParams: []byte(`{not valid json`),
	}
	_, err := ApplyDriverCompute(product, emptyBundle())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session_based params unmarshal")
}

// ── Fix-3: IndustryParams.UnitsProduced — surplus inventory ─────────────────

// TestApplyIndustryDriver_SurplusInventory verifies that when UnitsProduced > UnitsSold
// the surplus is carried as finished-goods inventory at the effective unit cost,
// and this value is accessible via SurplusInventoryValue in the returned bundle.
func TestApplyIndustryDriver_SurplusInventory(t *testing.T) {
	// Product: 100 units/yr sold at €100 each; unit cost = €40; scrap 0%.
	// Year 0: produce 120, sell 100 → surplus 20 units
	// Year 1: produce 100, sell 100 → no change, inventory stays 20 units
	// Year 2: produce  80, sell 100 → draw-down 20 units → inventory 0
	params := model.IndustryParams{
		UnitsProduced: [5]model.FlexInt64{120, 100, 80, 0, 0},
	}

	bundle := emptyBundle()
	for y := 0; y < MaxYears; y++ {
		bundle.Assumptions[y].RawMaterialCost = d("40")
	}
	bundle.Volumes = []model.ProductSalesVolume{
		{YearIndex: 1, UnitsSold: 100, Zone: model.ZoneFrance, Channel: model.ChannelDirect},
		{YearIndex: 2, UnitsSold: 100, Zone: model.ZoneFrance, Channel: model.ChannelDirect},
		{YearIndex: 3, UnitsSold: 100, Zone: model.ZoneFrance, Channel: model.ChannelDirect},
	}

	product := driverProduct(model.DriverIndustry, params)
	result, err := ApplyDriverCompute(product, bundle)
	assert.NoError(t, err)

	// Year 0: 120 produced − 100 sold = 20 surplus units at €40 each = €800
	assert.True(t, d("800").Equal(result.SurplusInventoryValue[0]),
		"Year 0: surplus 20 units @ €40 = €800, got %s", result.SurplusInventoryValue[0])

	// Year 1: 100 produced − 100 sold = 0 change; cumulative inventory = 20 units @ €40 = €800
	assert.True(t, d("800").Equal(result.SurplusInventoryValue[1]),
		"Year 1: inventory unchanged at 20 units, got %s", result.SurplusInventoryValue[1])

	// Year 2: 80 produced − 100 sold = −20; cumulative = 0 (clamped at 0)
	assert.True(t, decimal.Zero.Equal(result.SurplusInventoryValue[2]),
		"Year 2: inventory drawn to zero, got %s", result.SurplusInventoryValue[2])

	// Years 3 and 4: UnitsProduced == 0 → produce-to-order; SurplusInventoryValue stays 0
	assert.True(t, decimal.Zero.Equal(result.SurplusInventoryValue[3]),
		"Year 3: produce-to-order, surplus = 0")
	assert.True(t, decimal.Zero.Equal(result.SurplusInventoryValue[4]),
		"Year 4: produce-to-order, surplus = 0")
}

// TestApplyIndustryDriver_ProduceToOrder verifies that when UnitsProduced is all
// zeros the bundle is unaffected and SurplusInventoryValue remains zero for all years.
func TestApplyIndustryDriver_ProduceToOrder(t *testing.T) {
	params := model.IndustryParams{
		UnitsProduced: [5]model.FlexInt64{0, 0, 0, 0, 0}, // explicitly produce-to-order
	}

	bundle := emptyBundle()
	bundle.Volumes = []model.ProductSalesVolume{
		{YearIndex: 1, UnitsSold: 500, Zone: model.ZoneFrance, Channel: model.ChannelDirect},
	}
	for y := 0; y < MaxYears; y++ {
		bundle.Assumptions[y].RawMaterialCost = d("10")
	}

	product := driverProduct(model.DriverIndustry, params)
	result, err := ApplyDriverCompute(product, bundle)
	assert.NoError(t, err)

	for y := 0; y < MaxYears; y++ {
		assert.True(t, decimal.Zero.Equal(result.SurplusInventoryValue[y]),
			"Year %d: produce-to-order, SurplusInventoryValue must be zero", y)
	}
}
