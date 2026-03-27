package model

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DriverType enum
// ---------------------------------------------------------------------------

func TestDriverTypeValues(t *testing.T) {
	types := []DriverType{
		DriverGeneric,
		DriverSaaS,
		DriverConsulting,
		DriverIndustry,
		DriverMarketplace,
		DriverMedia,
		DriverSessionBased,
	}
	expectedStrings := []string{
		"generic",
		"saas",
		"consulting",
		"industry",
		"marketplace",
		"media",
		"session_based",
	}

	for i, dt := range types {
		assert.Equal(t, expectedStrings[i], string(dt),
			"DriverType constant %d has wrong string value", i)
	}
}

func TestDriverTypeFromString(t *testing.T) {
	// All known driver types should round-trip through string conversion.
	known := map[string]DriverType{
		"generic":      DriverGeneric,
		"saas":         DriverSaaS,
		"consulting":   DriverConsulting,
		"industry":     DriverIndustry,
		"marketplace":  DriverMarketplace,
		"media":        DriverMedia,
		"session_based": DriverSessionBased,
	}
	for str, expected := range known {
		got := DriverType(str)
		assert.Equal(t, expected, got, "DriverType(%q) mismatch", str)
	}
}

// ---------------------------------------------------------------------------
// ConsultingParams — JSON round-trip
// ---------------------------------------------------------------------------

func TestConsultingParamsJSONRoundTrip(t *testing.T) {
	original := ConsultingParams{
		Headcount:       [5]decimal.Decimal{d("2"), d("3"), d("4"), d("4"), d("5")},
		WorkingDays:     220,
		UtilizationRate: [5]decimal.Decimal{d("0.80"), d("0.82"), d("0.85"), d("0.85"), d("0.87")},
		MonthlyGross:    [5]decimal.Decimal{d("6500"), d("6700"), d("6900"), d("7100"), d("7300")},
		EmployerCharges: d("1.45"),
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ConsultingParams
	require.NoError(t, json.Unmarshal(data, &decoded))

	for i := 0; i < 5; i++ {
		assert.True(t, original.Headcount[i].Equal(decoded.Headcount[i]),
			"Headcount[%d]: want %s got %s", i, original.Headcount[i], decoded.Headcount[i])
		assert.True(t, original.UtilizationRate[i].Equal(decoded.UtilizationRate[i]),
			"UtilizationRate[%d]", i)
		assert.True(t, original.MonthlyGross[i].Equal(decoded.MonthlyGross[i]),
			"MonthlyGross[%d]", i)
	}
	assert.Equal(t, original.WorkingDays, decoded.WorkingDays)
	assert.True(t, original.EmployerCharges.Equal(decoded.EmployerCharges))
}

// ---------------------------------------------------------------------------
// SaaSParams — JSON round-trip
// ---------------------------------------------------------------------------

func TestSaaSParamsJSONRoundTrip(t *testing.T) {
	original := SaaSParams{
		ActiveUsers:    [5]FlexInt64{500, 1200, 2500, 4000, 6000},
		MonthlyFee:     [5]decimal.Decimal{d("49"), d("49"), d("55"), d("55"), d("60")},
		InfraCostPPU:   [5]decimal.Decimal{d("3"), d("3"), d("2.5"), d("2.5"), d("2")},
		SupportCostPPU: [5]decimal.Decimal{d("2"), d("2"), d("1.8"), d("1.8"), d("1.5")},
		ChurnRate:      [5]decimal.Decimal{d("0.02"), d("0.02"), d("0.015"), d("0.015"), d("0.01")},
		ExpansionRate:  [5]decimal.Decimal{d("0.05"), d("0.06"), d("0.07"), d("0.07"), d("0.08")},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded SaaSParams
	require.NoError(t, json.Unmarshal(data, &decoded))

	for i := 0; i < 5; i++ {
		assert.Equal(t, original.ActiveUsers[i], decoded.ActiveUsers[i],
			"ActiveUsers[%d]", i)
		assert.True(t, original.MonthlyFee[i].Equal(decoded.MonthlyFee[i]),
			"MonthlyFee[%d]", i)
		assert.True(t, original.InfraCostPPU[i].Equal(decoded.InfraCostPPU[i]),
			"InfraCostPPU[%d]", i)
		assert.True(t, original.SupportCostPPU[i].Equal(decoded.SupportCostPPU[i]),
			"SupportCostPPU[%d]", i)
		assert.True(t, original.ChurnRate[i].Equal(decoded.ChurnRate[i]),
			"ChurnRate[%d]", i)
		assert.True(t, original.ExpansionRate[i].Equal(decoded.ExpansionRate[i]),
			"ExpansionRate[%d]", i)
	}
}

// SaaS with omitempty fields absent — ChurnRate and ExpansionRate default to zero.
func TestSaaSParamsOmitEmpty(t *testing.T) {
	minimal := SaaSParams{
		ActiveUsers:    [5]FlexInt64{100, 200, 300, 400, 500},
		MonthlyFee:     [5]decimal.Decimal{d("29"), d("29"), d("29"), d("29"), d("29")},
		InfraCostPPU:   [5]decimal.Decimal{d("1"), d("1"), d("1"), d("1"), d("1")},
		SupportCostPPU: [5]decimal.Decimal{d("0.5"), d("0.5"), d("0.5"), d("0.5"), d("0.5")},
		// ChurnRate and ExpansionRate intentionally left zero
	}

	data, err := json.Marshal(minimal)
	require.NoError(t, err)

	// The JSON should still be parseable even without churnRate / expansionRate.
	var decoded SaaSParams
	require.NoError(t, json.Unmarshal(data, &decoded))
	for i := 0; i < 5; i++ {
		assert.True(t, decoded.ChurnRate[i].IsZero(), "ChurnRate[%d] should be zero", i)
		assert.True(t, decoded.ExpansionRate[i].IsZero(), "ExpansionRate[%d] should be zero", i)
	}
}

// ---------------------------------------------------------------------------
// IndustryParams — JSON round-trip
// ---------------------------------------------------------------------------

func TestIndustryParamsJSONRoundTrip(t *testing.T) {
	original := IndustryParams{
		ProductionCapacity: [5]FlexInt64{5000, 6000, 8000, 10000, 12000},
		ScrapRate:          [5]decimal.Decimal{d("0.03"), d("0.025"), d("0.02"), d("0.02"), d("0.015")},
		SetupCost:          [5]decimal.Decimal{d("15000"), d("15000"), d("20000"), d("20000"), d("20000")},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded IndustryParams
	require.NoError(t, json.Unmarshal(data, &decoded))

	for i := 0; i < 5; i++ {
		assert.Equal(t, original.ProductionCapacity[i], decoded.ProductionCapacity[i],
			"ProductionCapacity[%d]", i)
		assert.True(t, original.ScrapRate[i].Equal(decoded.ScrapRate[i]),
			"ScrapRate[%d]", i)
		assert.True(t, original.SetupCost[i].Equal(decoded.SetupCost[i]),
			"SetupCost[%d]", i)
	}
}

// ---------------------------------------------------------------------------
// MarketplaceParams — JSON round-trip
// ---------------------------------------------------------------------------

func TestMarketplaceParamsJSONRoundTrip(t *testing.T) {
	original := MarketplaceParams{
		Transactions:      [5]FlexInt64{10000, 25000, 60000, 120000, 200000},
		GMVPerTransaction: [5]decimal.Decimal{d("85"), d("87"), d("90"), d("92"), d("95")},
		TakeRate:          [5]decimal.Decimal{d("0.12"), d("0.12"), d("0.13"), d("0.13"), d("0.14")},
		PaymentCost:       [5]decimal.Decimal{d("1.50"), d("1.50"), d("1.40"), d("1.40"), d("1.30")},
		FixedInfraCost:    [5]decimal.Decimal{d("24000"), d("36000"), d("60000"), d("96000"), d("120000")},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded MarketplaceParams
	require.NoError(t, json.Unmarshal(data, &decoded))

	for i := 0; i < 5; i++ {
		assert.Equal(t, original.Transactions[i], decoded.Transactions[i],
			"Transactions[%d]", i)
		assert.True(t, original.GMVPerTransaction[i].Equal(decoded.GMVPerTransaction[i]),
			"GMVPerTransaction[%d]", i)
		assert.True(t, original.TakeRate[i].Equal(decoded.TakeRate[i]),
			"TakeRate[%d]", i)
		assert.True(t, original.PaymentCost[i].Equal(decoded.PaymentCost[i]),
			"PaymentCost[%d]", i)
		assert.True(t, original.FixedInfraCost[i].Equal(decoded.FixedInfraCost[i]),
			"FixedInfraCost[%d]", i)
	}
}

// ---------------------------------------------------------------------------
// MediaParams — JSON round-trip
// ---------------------------------------------------------------------------

func TestMediaParamsJSONRoundTrip(t *testing.T) {
	original := MediaParams{
		Impressions:               [5]FlexInt64{5_000_000, 12_000_000, 30_000_000, 60_000_000, 100_000_000},
		CPM:                       [5]decimal.Decimal{d("3.50"), d("3.80"), d("4.00"), d("4.20"), d("4.50")},
		FillRate:                  [5]decimal.Decimal{d("0.65"), d("0.70"), d("0.75"), d("0.78"), d("0.80")},
		ContentCost:               [5]decimal.Decimal{d("50000"), d("80000"), d("150000"), d("250000"), d("350000")},
		DeliveryCostPerImpression: [5]decimal.Decimal{d("0.00015"), d("0.00014"), d("0.00013"), d("0.00012"), d("0.00011")},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded MediaParams
	require.NoError(t, json.Unmarshal(data, &decoded))

	for i := 0; i < 5; i++ {
		assert.Equal(t, original.Impressions[i], decoded.Impressions[i],
			"Impressions[%d]", i)
		assert.True(t, original.CPM[i].Equal(decoded.CPM[i]),
			"CPM[%d]", i)
		assert.True(t, original.FillRate[i].Equal(decoded.FillRate[i]),
			"FillRate[%d]", i)
		assert.True(t, original.ContentCost[i].Equal(decoded.ContentCost[i]),
			"ContentCost[%d]", i)
		assert.True(t, original.DeliveryCostPerImpression[i].Equal(decoded.DeliveryCostPerImpression[i]),
			"DeliveryCostPerImpression[%d]", i)
	}
}

// ---------------------------------------------------------------------------
// Product.DriverParams — embedded JSONB round-trip via json.RawMessage
// ---------------------------------------------------------------------------

// TestProductDriverParamsStoredAsRawJSON verifies that a Product's DriverParams
// field can store and retrieve the typed struct without information loss.
func TestProductDriverParamsStoredAsRawJSON(t *testing.T) {
	params := ConsultingParams{
		Headcount:       [5]decimal.Decimal{d("1"), d("2"), d("2"), d("3"), d("3")},
		WorkingDays:     187,
		UtilizationRate: [5]decimal.Decimal{d("1"), d("1"), d("1"), d("1"), d("1")},
		MonthlyGross:    [5]decimal.Decimal{d("9000"), d("9000"), d("9500"), d("9500"), d("10000")},
		EmployerCharges: d("1.45"),
	}

	rawParams, err := json.Marshal(params)
	require.NoError(t, err)

	product := Product{
		Name:         "Strategy & Advisory",
		DriverType:   DriverConsulting,
		DriverParams: rawParams,
	}

	// Serialize and deserialize the whole Product struct (simulating what the
	// DTO layer + JSON API transport does).
	productJSON, err := json.Marshal(product)
	require.NoError(t, err)

	var decoded Product
	require.NoError(t, json.Unmarshal(productJSON, &decoded))

	assert.Equal(t, DriverConsulting, decoded.DriverType)
	require.NotNil(t, decoded.DriverParams)

	var decodedParams ConsultingParams
	require.NoError(t, json.Unmarshal(decoded.DriverParams, &decodedParams))

	assert.Equal(t, params.WorkingDays, decodedParams.WorkingDays)
	assert.True(t, params.EmployerCharges.Equal(decodedParams.EmployerCharges))
	for i := 0; i < 5; i++ {
		assert.True(t, params.Headcount[i].Equal(decodedParams.Headcount[i]),
			"Headcount[%d]", i)
		assert.True(t, params.MonthlyGross[i].Equal(decodedParams.MonthlyGross[i]),
			"MonthlyGross[%d]", i)
	}
}

// TestProductGenericDriverHasNilParams verifies that a generic product (backward
// compatible) can be serialised with no DriverParams (omitempty).
func TestProductGenericDriverHasNilParams(t *testing.T) {
	product := Product{
		Name:       "Legacy Product",
		DriverType: DriverGeneric,
		// DriverParams intentionally nil
	}

	data, err := json.Marshal(product)
	require.NoError(t, err)

	// driverParams should be absent from the JSON because of omitempty.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	_, hasDriverParams := raw["driverParams"]
	assert.False(t, hasDriverParams, "driverParams should be omitted for generic products")

	// Round-trip back.
	var decoded Product
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, DriverGeneric, decoded.DriverType)
	assert.Nil(t, decoded.DriverParams)
}

// ---------------------------------------------------------------------------
// Consulting cost-per-day derivation sanity check
// ---------------------------------------------------------------------------

// TestConsultingCostPerDayFormula validates the formula used in the seed service:
//
//	CostPerDay = (FTE × MonthlyGross × 12 × EmployerCharges) / (FTE × WorkingDays × UtilizationRate)
//	           = (MonthlyGross × 12 × EmployerCharges) / (WorkingDays × UtilizationRate)   [1-FTE case]
//
// Y1 Strategy & Advisory: 1 FTE × 9 000 × 12 × 1.45 / (1 × 220 × 0.85)
//
//	= 156 540 / 187 ≈ 837.43 €/day.
func TestConsultingCostPerDayFormula(t *testing.T) {
	params := ConsultingParams{
		Headcount:       [5]decimal.Decimal{d("1"), d("1"), d("1"), d("1"), d("1")},
		WorkingDays:     220,
		UtilizationRate: [5]decimal.Decimal{d("0.85"), d("0.85"), d("0.85"), d("0.85"), d("0.85")},
		MonthlyGross:    [5]decimal.Decimal{d("9000"), d("9000"), d("9000"), d("9000"), d("9000")},
		EmployerCharges: d("1.45"),
	}

	for i := 0; i < 5; i++ {
		annualCost := params.Headcount[i].
			Mul(params.MonthlyGross[i]).
			Mul(d("12")).
			Mul(params.EmployerCharges)

		wd := decimal.NewFromInt(int64(params.WorkingDays))
		// Total team billable days = FTE × WorkingDays × UtilizationRate
		teamBillableDays := params.Headcount[i].Mul(wd).Mul(params.UtilizationRate[i])
		costPerDay := annualCost.Div(teamBillableDays)

		// 1 × 9000 × 12 × 1.45 / (1 × 220 × 0.85) = 156 600 / 187 = 837.43
		expected := decimal.NewFromFloat(837.43)
		diff := costPerDay.Sub(expected).Abs()
		assert.True(t, diff.LessThan(d("0.10")),
			"Y%d cost/day: got %s, expected ~%s", i+1, costPerDay.StringFixed(2), expected.StringFixed(2))
	}
}

// TestConsultingDeliveryY1CostPerDay validates the Delivery / Implementation
// seed value: 4 FTE × 6 500 × 12 × 1.45 / (4 × 220 × 0.75) ≈ 686 €/day.
//
// The denominator is team billable days (FTE × WorkingDays × UtilizationRate),
// giving the total variable cost per billable day regardless of team size.
func TestConsultingDeliveryY1CostPerDay(t *testing.T) {
	fte := d("4")
	monthlyGross := d("6500")
	months := d("12")
	employerFactor := d("1.45")
	workingDays := decimal.NewFromInt(220)
	utilization := d("0.75")

	annualCost := fte.Mul(monthlyGross).Mul(months).Mul(employerFactor)
	// Team billable days = FTE × WorkingDays × UtilizationRate = 4 × 220 × 0.75 = 660
	teamBillableDays := fte.Mul(workingDays).Mul(utilization)
	costPerDay := annualCost.Div(teamBillableDays)

	// 4 × 6500 × 12 × 1.45 / (4 × 220 × 0.75) = 452 400 / 660 ≈ 686.36
	expected := decimal.NewFromFloat(686)
	diff := costPerDay.Sub(expected).Abs()
	assert.True(t, diff.LessThan(d("1")),
		"Y1 delivery cost/day: got %s, expected ~%s", costPerDay.StringFixed(2), expected.StringFixed(2))
}

// ---------------------------------------------------------------------------
// SessionBasedParams — JSON round-trip
// ---------------------------------------------------------------------------

func TestSessionBasedParamsJSONRoundTrip(t *testing.T) {
	original := SessionBasedParams{
		Sessions:               [5]FlexInt64{100, 140, 180, 220, 260},
		ParticipantsPerSession: [5]decimal.Decimal{d("20"), d("20"), d("22"), d("22"), d("25")},
		FillRate:               [5]decimal.Decimal{d("0.75"), d("0.78"), d("0.80"), d("0.82"), d("0.85")},
		PricePerParticipant:    [5]decimal.Decimal{d("150"), d("155"), d("160"), d("165"), d("170")},
		TrainerCount:           [5]decimal.Decimal{d("3"), d("4"), d("5"), d("6"), d("6")},
		SessionsPerTrainer:     [5]FlexInt64{40, 40, 40, 40, 45},
		UtilizationRate:        [5]decimal.Decimal{d("0.90"), d("0.90"), d("0.90"), d("0.90"), d("0.90")},
		TrainerCostPerSession:  [5]decimal.Decimal{d("400"), d("400"), d("420"), d("420"), d("450")},
		VariableCostPerParticipant: [5]decimal.Decimal{d("15"), d("15"), d("14"), d("14"), d("13")},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded SessionBasedParams
	require.NoError(t, json.Unmarshal(data, &decoded))

	for i := 0; i < 5; i++ {
		assert.Equal(t, original.Sessions[i], decoded.Sessions[i], "Sessions[%d]", i)
		assert.Equal(t, original.SessionsPerTrainer[i], decoded.SessionsPerTrainer[i], "SessionsPerTrainer[%d]", i)
		assert.True(t, original.ParticipantsPerSession[i].Equal(decoded.ParticipantsPerSession[i]), "ParticipantsPerSession[%d]", i)
		assert.True(t, original.FillRate[i].Equal(decoded.FillRate[i]), "FillRate[%d]", i)
		assert.True(t, original.PricePerParticipant[i].Equal(decoded.PricePerParticipant[i]), "PricePerParticipant[%d]", i)
		assert.True(t, original.TrainerCount[i].Equal(decoded.TrainerCount[i]), "TrainerCount[%d]", i)
		assert.True(t, original.UtilizationRate[i].Equal(decoded.UtilizationRate[i]), "UtilizationRate[%d]", i)
		assert.True(t, original.TrainerCostPerSession[i].Equal(decoded.TrainerCostPerSession[i]), "TrainerCostPerSession[%d]", i)
		assert.True(t, original.VariableCostPerParticipant[i].Equal(decoded.VariableCostPerParticipant[i]), "VariableCostPerParticipant[%d]", i)
	}
}

// ---------------------------------------------------------------------------
// SessionBasedParams — economic formula sanity checks
// ---------------------------------------------------------------------------

// TestSessionBasedUnitEconomicsFormula verifies the per-session unit price and unit
// cost formulae match the model spec.
//
// Setup (Y1):
//
//	ParticipantsPerSession = 20, FillRate = 0.75 → RealizedParticipants = 15
//	PricePerParticipant    = 150 → UnitPrice = 15 × 150 = 2 250 €
//	TrainerCostPerSession  = 400
//	VariableCostPerParticipant = 15 → UnitCost = 400 + 15 × 15 = 625 €
func TestSessionBasedUnitEconomicsFormula(t *testing.T) {
	participantsPerSession := d("20")
	fillRate := d("0.75")
	pricePerParticipant := d("150")
	trainerCostPerSession := d("400")
	variableCostPerParticipant := d("15")

	realized := participantsPerSession.Mul(fillRate) // 15
	unitPrice := realized.Mul(pricePerParticipant)   // 2 250
	unitCost := trainerCostPerSession.Add(variableCostPerParticipant.Mul(realized)) // 625

	assert.True(t, realized.Equal(d("15")), "realized participants: got %s", realized)
	assert.True(t, unitPrice.Equal(d("2250")), "unit price: got %s", unitPrice)
	assert.True(t, unitCost.Equal(d("625")), "unit cost: got %s", unitCost)
}

// TestSessionBasedCapacityClamp verifies that planned sessions are capped by
// TrainerCount × SessionsPerTrainer × UtilizationRate.
//
//	3 trainers × 40 sessions × 0.90 util = 108 capacity
//	Planned = 140 → ActualSessions = 108 (clamped)
func TestSessionBasedCapacityClamp(t *testing.T) {
	trainerCount := d("3")
	sessionsPerTrainer := int64(40)
	utilizationRate := d("0.90")
	plannedSessions := int64(140)

	capacity := trainerCount.
		Mul(decimal.NewFromInt(sessionsPerTrainer)).
		Mul(utilizationRate) // 108

	plannedDec := decimal.NewFromInt(plannedSessions)
	actualDec := plannedDec
	if capacity.LessThan(plannedDec) {
		actualDec = capacity
	}
	actual := actualDec.IntPart()

	assert.Equal(t, int64(108), actual, "capacity-clamped sessions should be 108")
}

// TestSessionBasedCapacityNotClamped verifies that when planned sessions are
// within trainer capacity the plan is NOT reduced.
//
//	5 trainers × 40 sessions × 0.90 util = 180 capacity
//	Planned = 150 → ActualSessions = 150 (no clamp)
func TestSessionBasedCapacityNotClamped(t *testing.T) {
	trainerCount := d("5")
	sessionsPerTrainer := int64(40)
	utilizationRate := d("0.90")
	plannedSessions := int64(150)

	capacity := trainerCount.
		Mul(decimal.NewFromInt(sessionsPerTrainer)).
		Mul(utilizationRate) // 180

	plannedDec := decimal.NewFromInt(plannedSessions)
	actualDec := plannedDec
	if capacity.LessThan(plannedDec) {
		actualDec = capacity
	}
	actual := actualDec.IntPart()

	assert.Equal(t, int64(150), actual, "sessions should not be clamped when under capacity")
}

// TestSessionBasedProductRoundTrip verifies that a Product with DriverSessionBased
// stores and retrieves SessionBasedParams correctly through json.RawMessage.
func TestSessionBasedProductRoundTrip(t *testing.T) {
	params := SessionBasedParams{
		Sessions:               [5]FlexInt64{80, 100, 120, 140, 160},
		ParticipantsPerSession: [5]decimal.Decimal{d("15"), d("15"), d("18"), d("18"), d("20")},
		FillRate:               [5]decimal.Decimal{d("0.80"), d("0.82"), d("0.83"), d("0.85"), d("0.87")},
		PricePerParticipant:    [5]decimal.Decimal{d("200"), d("200"), d("210"), d("210"), d("220")},
		TrainerCount:           [5]decimal.Decimal{d("2"), d("2"), d("3"), d("3"), d("4")},
		SessionsPerTrainer:     [5]FlexInt64{50, 50, 50, 50, 50},
		UtilizationRate:        [5]decimal.Decimal{d("0.85"), d("0.85"), d("0.85"), d("0.87"), d("0.87")},
		TrainerCostPerSession:  [5]decimal.Decimal{d("350"), d("350"), d("370"), d("370"), d("390")},
		VariableCostPerParticipant: [5]decimal.Decimal{d("10"), d("10"), d("10"), d("9"), d("9")},
	}

	rawParams, err := json.Marshal(params)
	require.NoError(t, err)

	product := Product{
		Name:         "Leadership Workshop",
		DriverType:   DriverSessionBased,
		DriverParams: rawParams,
	}

	productJSON, err := json.Marshal(product)
	require.NoError(t, err)

	var decoded Product
	require.NoError(t, json.Unmarshal(productJSON, &decoded))

	assert.Equal(t, DriverSessionBased, decoded.DriverType)
	require.NotNil(t, decoded.DriverParams)

	var decodedParams SessionBasedParams
	require.NoError(t, json.Unmarshal(decoded.DriverParams, &decodedParams))

	for i := 0; i < 5; i++ {
		assert.Equal(t, params.Sessions[i], decodedParams.Sessions[i], "Sessions[%d]", i)
		assert.Equal(t, params.SessionsPerTrainer[i], decodedParams.SessionsPerTrainer[i], "SessionsPerTrainer[%d]", i)
		assert.True(t, params.FillRate[i].Equal(decodedParams.FillRate[i]), "FillRate[%d]", i)
		assert.True(t, params.TrainerCostPerSession[i].Equal(decodedParams.TrainerCostPerSession[i]), "TrainerCostPerSession[%d]", i)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// d is a shorthand for decimal.RequireFromString.
func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic("invalid decimal literal in test: " + s)
	}
	return v
}
