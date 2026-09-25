package compute

import (
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test fixtures
// ─────────────────────────────────────────────────────────────────────────────

func makeCompany() model.CapTableCompany {
	return model.CapTableCompany{
		NominalValueCents: 1,
		FounderAlertPct:   decimal.NewFromInt(20),
		MaxPhases:         7,
	}
}

// seedTwoFounders returns two founder shareholders with 500 shares each (total 1 000).
func seedTwoFounders() []model.CapTableShareholder {
	return []model.CapTableShareholder{
		{
			TenantScoped: model.TenantScoped{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
			Name:         "Alice",
			Type:         model.ShareholderFounder,
			ClassType:    model.ShareClassCommon,
			InitShares:   500,
		},
		{
			TenantScoped: model.TenantScoped{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222")},
			Name:         "Bob",
			Type:         model.ShareholderFounder,
			ClassType:    model.ShareClassCommon,
			InitShares:   500,
		},
	}
}

// seedSeriesAInvestor returns an investor shareholder for a Series A round.
func seedSeriesAInvestor() model.CapTableShareholder {
	return model.CapTableShareholder{
		TenantScoped: model.TenantScoped{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333")},
		Name:         "VC Fund A",
		Type:         model.ShareholderInvestor,
		ClassType:    model.ShareClassPreferredA,
		InitShares:   0,
	}
}

// seedSeriesARound returns a funding round that issues 250 preferred-A shares
// at a €1 000 k pre-money valuation and raises €250 k.
// → PostMoney = 1250 k, new shares = 250 → share price = 5 k/share
func seedSeriesARound(roundID uuid.UUID) model.CapTableRound {
	preMoney := decimal.NewFromInt(1000)
	return model.CapTableRound{
		TenantScoped:       model.TenantScoped{ID: roundID},
		PhaseNumber:        1,
		Label:              "Série A",
		EventType:          model.RoundEventFunding,
		ShareClassType:     model.ShareClassPreferredA,
		NewSharesCreated:   250,
		AmountRaisedK:      decimal.NewFromInt(250),
		PreMoneyValuationK: &preMoney,
		NominalValueCents:  1,
		SplitCoefficient:   decimal.NewFromInt(1),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §1  ComputeIngeFi — founding state
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_FoundingState_NoRounds(t *testing.T) {
	company := makeCompany()
	shareholders := seedTwoFounders()

	result := ComputeIngeFi(company, shareholders, nil, nil, nil, nil)

	// Only Phase 0 (founding) should exist
	require.Len(t, result.Report.Phases, 1)
	phase0 := result.Report.Phases[0]
	assert.Equal(t, 0, phase0.PhaseNumber)
	assert.Equal(t, "Création", phase0.Label)

	// Each founder holds 50 %
	require.Len(t, phase0.Positions, 2)
	for _, pos := range phase0.Positions {
		assert.Equal(t, int64(500), pos.SharesAfterRound, "each founder should hold 500 shares")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §2  ComputeIngeFi — one funding round (Series A)
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_OneRound_BasicDilution(t *testing.T) {
	company := makeCompany()
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())
	roundID := uuid.New()
	rounds := []model.CapTableRound{seedSeriesARound(roundID)}

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	// Phase 0 (founding) + Phase 1 (Série A)
	require.Len(t, result.Report.Phases, 2)
	phase1 := result.Report.Phases[1]

	assert.Equal(t, 1, phase1.PhaseNumber)
	assert.Equal(t, "Série A", phase1.Label)

	// Total shares after round = 1000 (founders) + 250 (VC) = 1250
	assert.Equal(t, int64(1250), phase1.TotalSharesAfter)

	// Pre-money = 1000 k, post-money = 1250 k
	assert.True(t, phase1.PreMoneyValuationK.Equal(decimal.NewFromInt(1000)),
		"pre-money should be 1000 k")
	assert.True(t, phase1.PostMoneyValuationK.Equal(decimal.NewFromInt(1250)),
		"post-money should be 1250 k")

	// Share price = PostMoney / TotalShares = 1250 / 1250 = 1 k/share
	assert.True(t, phase1.SharePriceEur.Equal(decimal.NewFromInt(1)),
		"share price should be 1 k/share")

	// VC investor should have 250 shares → 20 % ownership
	var vcPos *model.IngeFiShareholderPosition
	for i := range phase1.Positions {
		if phase1.Positions[i].Name == "VC Fund A" {
			vcPos = &phase1.Positions[i]
		}
	}
	require.NotNil(t, vcPos, "VC Fund A position must exist")
	assert.Equal(t, int64(250), vcPos.SharesAfterRound)
	// PctBasicAfter = 250/1250 = 0.2
	assert.True(t, vcPos.PctBasicAfter.Equal(decimal.NewFromFloat(0.2).Round(4)),
		"VC should own 20%% basic after round")
}

// ─────────────────────────────────────────────────────────────────────────────
// §3  ComputeIngeFi — emission premium
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_EmissionPremium(t *testing.T) {
	company := makeCompany() // NominalValueCents = 1 cent
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())
	rounds := []model.CapTableRound{seedSeriesARound(uuid.New())}

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)
	require.Len(t, result.Report.Phases, 2)

	phase1 := result.Report.Phases[1]

	// nominalK = 1 cent / 100000 = 0.00001 k
	// sharePrice = 1 k/share (from §2)
	// emPremPerShare ≈ 0.99999 k ≈ 1 k (essentially share_price since nominal is tiny)
	assert.True(t, phase1.EmissionPremium.PerShareK.GreaterThan(decimal.Zero),
		"emission premium per share should be positive")
	assert.True(t, phase1.EmissionPremium.TotalK.GreaterThan(decimal.Zero),
		"total emission premium should be positive")
}

// ─────────────────────────────────────────────────────────────────────────────
// §4  ComputeIngeFi — down-round warning
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_DownRound_Warning(t *testing.T) {
	company := makeCompany()
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())

	roundAID := uuid.New()
	roundBID := uuid.New()
	preMoney1 := decimal.NewFromInt(1000)
	preMoney2 := decimal.NewFromInt(500) // down-round

	rounds := []model.CapTableRound{
		{
			TenantScoped:       model.TenantScoped{ID: roundAID},
			PhaseNumber:        1,
			Label:              "Série A",
			EventType:          model.RoundEventFunding,
			ShareClassType:     model.ShareClassPreferredA,
			NewSharesCreated:   250,
			AmountRaisedK:      decimal.NewFromInt(250),
			PreMoneyValuationK: &preMoney1,
			NominalValueCents:  1,
			SplitCoefficient:   decimal.NewFromInt(1),
		},
		{
			TenantScoped:       model.TenantScoped{ID: roundBID},
			PhaseNumber:        2,
			Label:              "Série B (down)",
			EventType:          model.RoundEventFunding,
			ShareClassType:     model.ShareClassPreferredA,
			NewSharesCreated:   100,
			AmountRaisedK:      decimal.NewFromInt(100),
			PreMoneyValuationK: &preMoney2,
			NominalValueCents:  1,
			SplitCoefficient:   decimal.NewFromInt(1),
		},
	}

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	hasDownRoundWarning := false
	for _, w := range result.Warnings {
		if w.Sheet == "captable" && w.Row == "Série B (down)" {
			hasDownRoundWarning = true
		}
	}
	assert.True(t, hasDownRoundWarning, "down-round should produce a warning")
}

// ─────────────────────────────────────────────────────────────────────────────
// §5  ComputeIngeFi — stock split
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_StockSplit(t *testing.T) {
	company := makeCompany()
	shareholders := seedTwoFounders()

	splitRound := model.CapTableRound{
		TenantScoped:      model.TenantScoped{ID: uuid.New()},
		PhaseNumber:       1,
		Label:             "10-for-1 split",
		EventType:         model.RoundEventSplit,
		ShareClassType:    model.ShareClassCommon,
		NewSharesCreated:  0,
		AmountRaisedK:     decimal.Zero,
		NominalValueCents: 1,
		SplitCoefficient:  decimal.NewFromInt(10),
	}

	result := ComputeIngeFi(company, shareholders, nil, []model.CapTableRound{splitRound}, nil, nil)
	require.Len(t, result.Report.Phases, 2)

	phase1 := result.Report.Phases[1]
	// Each founder had 500 shares → after 10× split = 5 000 shares
	for _, pos := range phase1.Positions {
		assert.Equal(t, int64(5000), pos.SharesAfterRound,
			"shares should be multiplied by 10 after split")
	}
	// Nominal value should be halved from 1 cent → 0 (rounded down from 0.1 cent)
	assert.Equal(t, int64(0), phase1.NominalValueCents)
}

// ─────────────────────────────────────────────────────────────────────────────
// §6  ComputeIngeFi — PctGranted mode (no pre-money provided)
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_PctGrantedMode(t *testing.T) {
	company := makeCompany()
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())

	// Raise 200 k for 20 % → post-money = 1000 k, pre-money = 800 k
	round := model.CapTableRound{
		TenantScoped:      model.TenantScoped{ID: uuid.New()},
		PhaseNumber:       1,
		Label:             "Série A via pct",
		EventType:         model.RoundEventFunding,
		ShareClassType:    model.ShareClassPreferredA,
		NewSharesCreated:  250,
		AmountRaisedK:     decimal.NewFromInt(200),
		PctGranted:        decimal.NewFromInt(20),
		NominalValueCents: 1,
		SplitCoefficient:  decimal.NewFromInt(1),
	}

	result := ComputeIngeFi(company, shareholders, nil, []model.CapTableRound{round}, nil, nil)
	require.Len(t, result.Report.Phases, 2)

	phase1 := result.Report.Phases[1]
	// PostMoney = 200 / 0.20 = 1000 k
	assert.True(t, phase1.PostMoneyValuationK.Equal(decimal.NewFromInt(1000)),
		"post-money should derive correctly from pct-granted mode")
	assert.True(t, phase1.PreMoneyValuationK.Equal(decimal.NewFromInt(800)),
		"pre-money = post - raised")
}

// ─────────────────────────────────────────────────────────────────────────────
// §7  ComputeIngeFi — founder alert threshold
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_FounderAlertThreshold(t *testing.T) {
	company := makeCompany()
	company.FounderAlertPct = decimal.NewFromInt(40) // alert if < 40 %

	shareholders := []model.CapTableShareholder{
		{
			TenantScoped: model.TenantScoped{ID: uuid.New()},
			Name:         "Solo Founder",
			Type:         model.ShareholderFounder,
			ClassType:    model.ShareClassCommon,
			InitShares:   1000,
		},
		{
			TenantScoped: model.TenantScoped{ID: uuid.New()},
			Name:         "Investor",
			Type:         model.ShareholderInvestor,
			ClassType:    model.ShareClassPreferredA,
			InitShares:   0,
		},
	}

	preMoney := decimal.NewFromInt(1000)
	// 1000 new shares issued → founder drops to 50 %, still above 40 %
	round := model.CapTableRound{
		TenantScoped:       model.TenantScoped{ID: uuid.New()},
		PhaseNumber:        1,
		Label:              "Round 1",
		EventType:          model.RoundEventFunding,
		ShareClassType:     model.ShareClassPreferredA,
		NewSharesCreated:   1000,
		AmountRaisedK:      decimal.NewFromInt(500),
		PreMoneyValuationK: &preMoney,
		NominalValueCents:  1,
		SplitCoefficient:   decimal.NewFromInt(1),
	}

	result := ComputeIngeFi(company, shareholders, nil, []model.CapTableRound{round}, nil, nil)
	// Founder at 50 % > 40 % threshold → no warning
	hasAlert := false
	for _, w := range result.Warnings {
		if w.Row == "Solo Founder" {
			hasAlert = true
		}
	}
	assert.False(t, hasAlert, "no alert when founder ownership > threshold")

	// Now set threshold above 51 % → should trigger
	company.FounderAlertPct = decimal.NewFromInt(60)
	result2 := ComputeIngeFi(company, shareholders, nil, []model.CapTableRound{round}, nil, nil)
	hasAlert2 := false
	for _, w := range result2.Warnings {
		if w.Row == "Solo Founder" {
			hasAlert2 = true
		}
	}
	assert.True(t, hasAlert2, "alert should fire when founder ownership < threshold")
}

// ─────────────────────────────────────────────────────────────────────────────
// §8  ComputeValuation — mode A: Multiple → IRR
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeValuation_ModeA_MultipleToIRR(t *testing.T) {
	multiple := decimal.NewFromFloat(3.0)
	horizon := decimal.NewFromFloat(5.0)
	vs := model.ValuationScenario{
		CalcType:      model.ValScenMultipleToIRR,
		Label:         "3× in 5 years",
		MoneyMultiple: &multiple,
		HorizonYears:  &horizon,
	}

	result := ComputeValuation(vs)

	// IRR = 3^(1/5) - 1 = 0.24573...
	require.NotNil(t, result.IRRPct)
	// Expect ~24.57 % → 0.2457 rounded to 4dp
	expected := decimal.NewFromFloat(0.2457)
	assert.True(t, result.IRRPct.Sub(expected).Abs().LessThan(decimal.NewFromFloat(0.0001)),
		"IRR should be ~24.57%% for 3× in 5 years, got %s", result.IRRPct.String())
}

func TestComputeValuation_ModeA_ZeroHorizon_NoResult(t *testing.T) {
	multiple := decimal.NewFromFloat(3.0)
	horizon := decimal.NewFromFloat(0.0)
	vs := model.ValuationScenario{
		CalcType:      model.ValScenMultipleToIRR,
		MoneyMultiple: &multiple,
		HorizonYears:  &horizon,
	}
	result := ComputeValuation(vs)
	assert.Nil(t, result.IRRPct, "should not compute IRR when horizon = 0")
}

// ─────────────────────────────────────────────────────────────────────────────
// §9  ComputeValuation — mode B: Investment → Terminal
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeValuation_ModeB_InvestmentToTerminal(t *testing.T) {
	inv := decimal.NewFromFloat(100.0)   // 100 k
	yield := decimal.NewFromFloat(10.0)  // 10 % per year
	horizon := decimal.NewFromFloat(5.0) // 5 years

	vs := model.ValuationScenario{
		CalcType:       model.ValScenInvestmentToTerminal,
		Label:          "100k at 10%% for 5y",
		InvestmentK:    &inv,
		AnnualYieldPct: &yield,
		HorizonYears:   &horizon,
	}

	result := ComputeValuation(vs)

	// Terminal = 100 × 1.10^5 = 161.0510...
	require.NotNil(t, result.TerminalValueK)
	expected := decimal.NewFromFloat(161.05)
	assert.True(t, result.TerminalValueK.Sub(expected).Abs().LessThan(decimal.NewFromFloat(0.01)),
		"terminal value should be ~161.05 k, got %s", result.TerminalValueK.String())

	// Multiple = 161.05 / 100 ≈ 1.6105
	require.NotNil(t, result.MoneyMultiple)
	assert.True(t, result.MoneyMultiple.GreaterThan(decimal.NewFromFloat(1.0)),
		"money multiple > 1")
}

// ─────────────────────────────────────────────────────────────────────────────
// §10 ComputeValuation — mode C: Investment + % → IRR + NPV
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeValuation_ModeC_InvestmentPctToIRR(t *testing.T) {
	inv := decimal.NewFromFloat(200.0)      // 200 k invested
	pct := decimal.NewFromFloat(20.0)       // 20 % at exit
	exitVal := decimal.NewFromFloat(2000.0) // 2 000 k exit
	horizon := decimal.NewFromFloat(5.0)
	discount := decimal.NewFromFloat(10.0)

	vs := model.ValuationScenario{
		CalcType:          model.ValScenInvestmentPctToIRR,
		InvestmentK:       &inv,
		FinalInvestorPct:  &pct,
		ExitCompanyValueK: &exitVal,
		HorizonYears:      &horizon,
		DiscountRatePct:   &discount,
	}

	result := ComputeValuation(vs)

	// Investor share = 2000 × 0.20 = 400 k
	require.NotNil(t, result.InvestorShareK)
	assert.True(t, result.InvestorShareK.Equal(decimal.NewFromFloat(400).Round(2)),
		"investor share should be 400 k")

	// Multiple = 400 / 200 = 2×
	require.NotNil(t, result.MoneyMultiple)
	assert.True(t, result.MoneyMultiple.Equal(decimal.NewFromFloat(2.0).Round(4)),
		"multiple should be 2×")

	// IRR = 2^(1/5) - 1 ≈ 0.1487
	require.NotNil(t, result.IRRPct)
	assert.True(t, result.IRRPct.GreaterThan(decimal.NewFromFloat(0.14)),
		"IRR should be > 14%%")

	// NPV = 400 / 1.1^5 ≈ 248.37 k
	require.NotNil(t, result.NPVK)
	assert.True(t, result.NPVK.GreaterThan(decimal.NewFromFloat(200)),
		"NPV should be positive")
}

// ─────────────────────────────────────────────────────────────────────────────
// §11 ComputeValuation — mode D: New money + % → Pre/Post-money
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeValuation_ModeD_NewMoneyToPremoney(t *testing.T) {
	newMoney := decimal.NewFromFloat(500.0) // 500 k
	pct := decimal.NewFromFloat(20.0)       // 20 % post-money

	vs := model.ValuationScenario{
		CalcType:         model.ValScenNewMoneyToPremoney,
		NewMoneyK:        &newMoney,
		FinalInvestorPct: &pct,
	}

	result := ComputeValuation(vs)

	// PostMoney = 500 / 0.20 = 2500 k
	require.NotNil(t, result.PostMoneyK)
	assert.True(t, result.PostMoneyK.Equal(decimal.NewFromFloat(2500).Round(2)),
		"post-money should be 2500 k, got %s", result.PostMoneyK.String())

	// PreMoney = 2500 - 500 = 2000 k
	require.NotNil(t, result.PreMoneyK)
	assert.True(t, result.PreMoneyK.Equal(decimal.NewFromFloat(2000).Round(2)),
		"pre-money should be 2000 k, got %s", result.PreMoneyK.String())
}

func TestComputeValuation_ModeD_ZeroPct_NoResult(t *testing.T) {
	newMoney := decimal.NewFromFloat(500.0)
	pct := decimal.NewFromFloat(0.0) // zero → division by zero guard

	vs := model.ValuationScenario{
		CalcType:         model.ValScenNewMoneyToPremoney,
		NewMoneyK:        &newMoney,
		FinalInvestorPct: &pct,
	}

	result := ComputeValuation(vs)
	assert.Nil(t, result.PostMoneyK, "should not compute when pct = 0 (division by zero guard)")
}

// ─────────────────────────────────────────────────────────────────────────────
// §12 ComputeStockOptions — plan summary and pool utilisation
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeStockOptions_PlanSummary(t *testing.T) {
	company := makeCompany()
	planID := uuid.New()
	shareholderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	shareholders := seedTwoFounders()
	plans := []model.StockOptionPlan{
		{
			TenantScoped:  model.TenantScoped{ID: planID},
			PlanLabel:     "BSPCE 2024",
			Instrument:    model.SOIBSPCE,
			ExercisePrice: decimal.NewFromFloat(1.0),
			OptionsVoted:  1000,
		},
	}
	grants := []model.OptionGrant{
		{
			PlanID:           planID,
			ShareholderID:    shareholderID,
			OptionsGranted:   600,
			OptionsExercised: 100,
			OptionsCancelled: 50,
		},
	}

	emptyIngefi := ingeFiResult{}
	report := ComputeStockOptions(company, shareholders, nil, plans, grants, emptyIngefi)

	require.Len(t, report.Plans, 1)
	summary := report.Plans[0]

	assert.Equal(t, "BSPCE 2024", summary.PlanLabel)
	assert.Equal(t, int64(1000), summary.Voted)
	assert.Equal(t, int64(600), summary.Attributed)
	assert.Equal(t, int64(100), summary.Exercised)
	assert.Equal(t, int64(50), summary.Cancelled)
	assert.Equal(t, int64(400), summary.Reserve)     // 1000 - 600
	assert.Equal(t, int64(450), summary.Outstanding) // 600 - 100 - 50

	// Pool gauge
	require.Len(t, report.Charts.PoolUtilisationGauge, 1)
	gauge := report.Charts.PoolUtilisationGauge[0]
	assert.Equal(t, "BSPCE 2024", gauge.PlanLabel)
	assert.Equal(t, int64(1000), gauge.Total)
	assert.Equal(t, int64(600), gauge.Used)
	// PctUsed = 600/1000 = 0.6
	assert.True(t, gauge.PctUsed.Equal(decimal.NewFromFloat(0.6).Round(4)))
}

func TestComputeStockOptions_EmptyPlans_EmptyReport(t *testing.T) {
	company := makeCompany()
	report := ComputeStockOptions(company, nil, nil, nil, nil, ingeFiResult{})

	assert.Empty(t, report.Plans)
	assert.Empty(t, report.ByPhase)
	assert.Empty(t, report.Charts.PoolUtilisationGauge)
}

// ─────────────────────────────────────────────────────────────────────────────
// §13 ComputeCapTable — orchestrator integration
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeCapTable_FullOrchestration(t *testing.T) {
	company := makeCompany()
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())
	rounds := []model.CapTableRound{seedSeriesARound(uuid.New())}

	multiple := decimal.NewFromFloat(2.0)
	horizon := decimal.NewFromFloat(3.0)
	vScenarios := []model.ValuationScenario{
		{
			TenantScoped:  model.TenantScoped{ID: uuid.New()},
			CalcType:      model.ValScenMultipleToIRR,
			Label:         "2× in 3 years",
			MoneyMultiple: &multiple,
			HorizonYears:  &horizon,
		},
	}

	report := ComputeCapTable(company, shareholders, nil, rounds, nil, nil, vScenarios)

	// IngéFi
	assert.Len(t, report.IngeFi.Phases, 2, "founding + 1 round")
	assert.NotEmpty(t, report.IngeFi.Matrix.ColumnLabels)
	assert.NotEmpty(t, report.IngeFi.Waterfall.Holders)

	// FastValo
	assert.Len(t, report.FastValo.Scenarios, 1)
	assert.NotNil(t, report.FastValo.Scenarios[0].IRRPct)

	// No error warnings for clean scenario
	assert.Empty(t, report.Warnings)
}

// ─────────────────────────────────────────────────────────────────────────────
// §14 DilutionWaterfall — holder trajectory
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_Waterfall_FounderDilution(t *testing.T) {
	company := makeCompany()
	founderID := uuid.New()
	investorID := uuid.New()

	shareholders := []model.CapTableShareholder{
		{
			TenantScoped: model.TenantScoped{ID: founderID},
			Name:         "Founder",
			Type:         model.ShareholderFounder,
			ClassType:    model.ShareClassCommon,
			InitShares:   1000,
		},
		{
			TenantScoped: model.TenantScoped{ID: investorID},
			Name:         "Investor",
			Type:         model.ShareholderInvestor,
			ClassType:    model.ShareClassPreferredA,
			InitShares:   0,
		},
	}

	preMoney := decimal.NewFromInt(1000)
	rounds := []model.CapTableRound{
		{
			TenantScoped:       model.TenantScoped{ID: uuid.New()},
			PhaseNumber:        1,
			Label:              "Series A",
			EventType:          model.RoundEventFunding,
			ShareClassType:     model.ShareClassPreferredA,
			NewSharesCreated:   500, // 500 new → founder dilutes from 100% to 66.7%
			AmountRaisedK:      decimal.NewFromInt(500),
			PreMoneyValuationK: &preMoney,
			NominalValueCents:  1,
			SplitCoefficient:   decimal.NewFromInt(1),
		},
	}

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	waterfall := result.Report.Waterfall
	require.NotEmpty(t, waterfall.Holders)

	var founderHolder *model.DilutionWaterfallHolder
	for i := range waterfall.Holders {
		if waterfall.Holders[i].Name == "Founder" {
			founderHolder = &waterfall.Holders[i]
		}
	}
	require.NotNil(t, founderHolder, "founder should be in waterfall")

	// InitialPct = 100%, FinalPct ≈ 66.67%
	assert.True(t, founderHolder.InitialPct.Equal(decimal.NewFromInt(1)),
		"founder starts at 100%%")
	assert.True(t, founderHolder.FinalPct.LessThan(decimal.NewFromInt(1)),
		"founder is diluted after round")
	assert.True(t, founderHolder.TotalDilution.GreaterThan(decimal.Zero),
		"total dilution should be positive")
}

// ─────────────────────────────────────────────────────────────────────────────
// §15 CapTableMatrix — ownership grid
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_Matrix_ColumnLabels(t *testing.T) {
	company := makeCompany()
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())
	rounds := []model.CapTableRound{seedSeriesARound(uuid.New())}

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)
	matrix := result.Report.Matrix

	// Founding ("Création") + 1 round = 2 columns
	assert.Len(t, matrix.ColumnLabels, 2)
	assert.Equal(t, "Création", matrix.ColumnLabels[0])
	assert.Equal(t, "Série A", matrix.ColumnLabels[1])

	// One row per shareholder
	assert.Len(t, matrix.Rows, 3)
}

// ─────────────────────────────────────────────────────────────────────────────
// §16 Non-regression: consistent pct sum ≈ 1 (100%)
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_PctSumIsOne_AfterEachRound(t *testing.T) {
	company := makeCompany()
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())
	rounds := []model.CapTableRound{seedSeriesARound(uuid.New())}

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	for _, phase := range result.Report.Phases {
		var sumPct decimal.Decimal
		for _, pos := range phase.Positions {
			sumPct = sumPct.Add(pos.PctBasicAfter)
		}
		// Allow tiny rounding difference (< 0.001)
		diff := sumPct.Sub(decimal.NewFromInt(1)).Abs()
		assert.True(t, diff.LessThan(decimal.NewFromFloat(0.001)),
			"Phase %d: sum of basic pct should be ~1 (100%%), got %s", phase.PhaseNumber, sumPct.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §17 Non-regression: fully-diluted % ≤ basic % for diluted holders
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_FullyDilutedLteBasic_ForExistingHolders(t *testing.T) {
	company := makeCompany()
	planID := uuid.New()
	roundID := uuid.New()
	founderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	investorID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())
	rounds := []model.CapTableRound{seedSeriesARound(roundID)}
	plans := []model.StockOptionPlan{
		{
			TenantScoped:  model.TenantScoped{ID: planID},
			PlanLabel:     "Pool",
			Instrument:    model.SOIBSPCE,
			ExercisePrice: decimal.NewFromFloat(1.0),
			OptionsVoted:  200,
		},
	}
	grants := []model.OptionGrant{
		{
			PlanID:         planID,
			ShareholderID:  investorID,
			RoundID:        roundID,
			OptionsGranted: 200,
		},
	}
	_ = founderID

	result := ComputeIngeFi(company, shareholders, nil, rounds, grants, plans)

	phase1 := result.Report.Phases[1]
	for _, pos := range phase1.Positions {
		// Existing holders (no new options) should have FD <= Basic
		assert.True(t,
			pos.PctFullyDiluted.LessThanOrEqual(pos.PctBasicAfter.Add(decimal.NewFromFloat(0.0001))),
			"Holder %s: FD pct (%s) should be ≤ basic pct (%s) for diluted holders",
			pos.Name, pos.PctFullyDiluted.String(), pos.PctBasicAfter.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §18 Non-regression: shares are conserved across rounds (no shares lost)
// ─────────────────────────────────────────────────────────────────────────────

func TestComputeIngeFi_SharesConserved(t *testing.T) {
	company := makeCompany()
	shareholders := append(seedTwoFounders(), seedSeriesAInvestor())
	roundID := uuid.New()
	rounds := []model.CapTableRound{seedSeriesARound(roundID)}

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	phase0 := result.Report.Phases[0]
	phase1 := result.Report.Phases[1]

	var sharesPhase0, sharesPhase1 int64
	for _, pos := range phase0.Positions {
		sharesPhase0 += pos.SharesAfterRound
	}
	for _, pos := range phase1.Positions {
		sharesPhase1 += pos.SharesAfterRound
	}

	// After a funding round: phase1 total = phase0 total + new shares
	assert.Equal(t, sharesPhase0+250, sharesPhase1,
		"total shares should increase by exactly new shares created (250)")
}
