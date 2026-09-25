package compute

// Integration and non-regression tests for the Cap Table module.
// These tests simulate realistic French startup equity journeys from founding
// through Series B to verify end-to-end correctness of the dilution engine.
//
// Test scenario: "SaaSco SAS"
// ─────────────────────────────────────────────────────────────────────────────
//  Founding:    Alice 700 shares (70%) + Bob 300 shares (30%)  — 1 000 total
//  BSPCE pool:  200 BSPCE options at Seed phase (20% of initial capital)
//  Seed round:  +500 shares @ 500 k pre-money, raise 250 k → post 750 k
//  Series A:    +300 shares @ 1 500 k pre-money, raise 300 k → post 1 800 k
//  Valuation:   Mode A (3× in 5y → IRR), Mode D (500 k for 20% → premoney)
// ─────────────────────────────────────────────────────────────────────────────

import (
	"testing"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fixture builders
// ─────────────────────────────────────────────────────────────────────────────

var (
	aliceID      = uuid.MustParse("aaaa0000-0000-0000-0000-000000000001")
	bobID        = uuid.MustParse("bbbb0000-0000-0000-0000-000000000002")
	seedVCID     = uuid.MustParse("cccc0000-0000-0000-0000-000000000003")
	seriesAID    = uuid.MustParse("dddd0000-0000-0000-0000-000000000004")
	seedRndID    = uuid.MustParse("eeee0000-0000-0000-0000-000000000005")
	seriesARndID = uuid.MustParse("ffff0000-0000-0000-0000-000000000006")
	planID       = uuid.MustParse("1111aaaa-0000-0000-0000-000000000007")
	grantID1     = uuid.MustParse("2222bbbb-0000-0000-0000-000000000008")
	valo1ID      = uuid.MustParse("3333cccc-0000-0000-0000-000000000009")
	valo2ID      = uuid.MustParse("4444dddd-0000-0000-0000-000000000010")
)

func saascoCompany() model.CapTableCompany {
	return model.CapTableCompany{
		TenantScoped:      model.TenantScoped{ID: uuid.New()},
		CompanyName:       "SaaSco SAS",
		Currency:          "EUR",
		NominalValueCents: 1,
		InitialShares:     1000,
		FounderAlertPct:   decimal.NewFromInt(20),
		MaxPhases:         7,
	}
}

func saascoShareholders() []model.CapTableShareholder {
	return []model.CapTableShareholder{
		{
			TenantScoped: model.TenantScoped{ID: aliceID},
			Name:         "Alice (fondatrice)",
			Type:         model.ShareholderFounder,
			ClassType:    model.ShareClassCommon,
			InitShares:   700,
		},
		{
			TenantScoped: model.TenantScoped{ID: bobID},
			Name:         "Bob (associé)",
			Type:         model.ShareholderAssociate,
			ClassType:    model.ShareClassCommon,
			InitShares:   300,
		},
		{
			TenantScoped: model.TenantScoped{ID: seedVCID},
			Name:         "Seed Fund",
			Type:         model.ShareholderInvestor,
			ClassType:    model.ShareClassPreferredA,
			InitShares:   0,
		},
		{
			TenantScoped: model.TenantScoped{ID: seriesAID},
			Name:         "Series A Fund",
			Type:         model.ShareholderInvestor,
			ClassType:    model.ShareClassPreferredB,
			InitShares:   0,
		},
	}
}

func saascoRounds() []model.CapTableRound {
	preSeed := decimal.NewFromInt(500)
	preSeriesA := decimal.NewFromInt(1500)

	return []model.CapTableRound{
		{
			TenantScoped:       model.TenantScoped{ID: seedRndID},
			PhaseNumber:        1,
			Label:              "Seed",
			EventType:          model.RoundEventFunding,
			ShareClassType:     model.ShareClassPreferredA,
			NewSharesCreated:   500,
			AmountRaisedK:      decimal.NewFromInt(250),
			PreMoneyValuationK: &preSeed,
			NominalValueCents:  1,
			SplitCoefficient:   decimal.NewFromInt(1),
		},
		{
			TenantScoped:       model.TenantScoped{ID: seriesARndID},
			PhaseNumber:        2,
			Label:              "Série A",
			EventType:          model.RoundEventFunding,
			ShareClassType:     model.ShareClassPreferredB,
			NewSharesCreated:   300,
			AmountRaisedK:      decimal.NewFromInt(300),
			PreMoneyValuationK: &preSeriesA,
			NominalValueCents:  1,
			SplitCoefficient:   decimal.NewFromInt(1),
		},
	}
}

func saascoPlan() model.StockOptionPlan {
	return model.StockOptionPlan{
		TenantScoped:  model.TenantScoped{ID: planID},
		PlanLabel:     "BSPCE SaaSco 2024",
		Instrument:    model.SOIBSPCE,
		ExercisePrice: decimal.NewFromFloat(0.5),
		OptionsVoted:  200,
	}
}

func saascoGrants() []model.OptionGrant {
	return []model.OptionGrant{
		{
			TenantScoped:     model.TenantScoped{ID: grantID1},
			PlanID:           planID,
			ShareholderID:    aliceID,
			RoundID:          seedRndID,
			OptionsGranted:   150,
			OptionsExercised: 0,
			OptionsCancelled: 0,
		},
	}
}

func saascoValuations() []model.ValuationScenario {
	multiple := decimal.NewFromFloat(3.0)
	horizon5 := decimal.NewFromFloat(5.0)
	newMoney := decimal.NewFromFloat(500.0)
	pct20 := decimal.NewFromFloat(20.0)

	return []model.ValuationScenario{
		{
			TenantScoped:  model.TenantScoped{ID: valo1ID},
			CalcType:      model.ValScenMultipleToIRR,
			Label:         "Bull case 3×",
			MoneyMultiple: &multiple,
			HorizonYears:  &horizon5,
		},
		{
			TenantScoped:     model.TenantScoped{ID: valo2ID},
			CalcType:         model.ValScenNewMoneyToPremoney,
			Label:            "Series B term sheet",
			NewMoneyK:        &newMoney,
			FinalInvestorPct: &pct20,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration test 1: Full journey — phases, ownership %, valuation
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_SaaSco_FullJourney(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()
	plans := []model.StockOptionPlan{saascoPlan()}
	grants := saascoGrants()
	vScenarios := saascoValuations()

	report := ComputeCapTable(company, shareholders, nil, rounds, plans, grants, vScenarios)

	// ── Phase count ──────────────────────────────────────────────────────────
	// Phase 0 (founding) + Seed (1) + Série A (2) = 3 phases
	require.Len(t, report.IngeFi.Phases, 3, "expected founding + 2 funding phases")

	// ── Founding (Phase 0) ───────────────────────────────────────────────────
	phase0 := report.IngeFi.Phases[0]
	assert.Equal(t, 0, phase0.PhaseNumber)
	assert.Equal(t, int64(1000), phase0.TotalSharesAfter,
		"founding: Alice 700 + Bob 300 = 1 000 shares")

	var alicePct0 decimal.Decimal
	for _, pos := range phase0.Positions {
		if pos.Name == "Alice (fondatrice)" {
			alicePct0 = pos.PctBasicAfter
		}
	}
	// Alice = 700/1000 = 70 %
	assert.True(t, alicePct0.Equal(decimal.NewFromFloat(0.7).Round(4)),
		"Alice should own 70%% at founding, got %s", alicePct0.String())

	// ── Seed (Phase 1) ───────────────────────────────────────────────────────
	phase1 := report.IngeFi.Phases[1]
	assert.Equal(t, "Seed", phase1.Label)
	// Shares: 1000 + 500 new = 1500
	assert.Equal(t, int64(1500), phase1.TotalSharesAfter)
	// PostMoney = 500 + 250 = 750 k
	assert.True(t, phase1.PostMoneyValuationK.Equal(decimal.NewFromInt(750)))
	// SharePrice = 750 / 1500 = 0.5 k/share
	assert.True(t, phase1.SharePriceEur.Equal(decimal.NewFromFloat(0.5)))

	// Alice: 700 shares → 700/1500 ≈ 46.67 %
	var alicePct1 decimal.Decimal
	for _, pos := range phase1.Positions {
		if pos.Name == "Alice (fondatrice)" {
			alicePct1 = pos.PctBasicAfter
		}
	}
	expectedAlicePct1 := decimal.NewFromFloat(700.0 / 1500.0).Round(4)
	assert.True(t, alicePct1.Equal(expectedAlicePct1),
		"Alice pct at Seed should be ~46.67%%, got %s", alicePct1.String())

	// ── Série A (Phase 2) ────────────────────────────────────────────────────
	phase2 := report.IngeFi.Phases[2]
	assert.Equal(t, "Série A", phase2.Label)
	// Shares: 1500 + 300 new = 1800
	assert.Equal(t, int64(1800), phase2.TotalSharesAfter)
	// PostMoney = 1500 + 300 = 1800 k
	assert.True(t, phase2.PostMoneyValuationK.Equal(decimal.NewFromInt(1800)))

	// ── No down-round warnings ────────────────────────────────────────────────
	// PostMoney goes 750 → 1800 → no down-round
	for _, w := range report.Warnings {
		assert.NotContains(t, w.Message, "Down-round",
			"clean up-rounds should produce no down-round warning")
	}

	// ── Matrix structure ─────────────────────────────────────────────────────
	matrix := report.IngeFi.Matrix
	assert.Len(t, matrix.ColumnLabels, 3, "3 phases = 3 columns")
	assert.Equal(t, "Création", matrix.ColumnLabels[0])
	assert.Equal(t, "Seed", matrix.ColumnLabels[1])
	assert.Equal(t, "Série A", matrix.ColumnLabels[2])
	// 4 shareholders (Alice, Bob, Seed Fund, Series A Fund)
	assert.Len(t, matrix.Rows, 4)

	// ── Waterfall ────────────────────────────────────────────────────────────
	waterfall := report.IngeFi.Waterfall
	require.NotEmpty(t, waterfall.Holders)
	var aliceHolder *model.DilutionWaterfallHolder
	for i := range waterfall.Holders {
		if waterfall.Holders[i].Name == "Alice (fondatrice)" {
			aliceHolder = &waterfall.Holders[i]
		}
	}
	require.NotNil(t, aliceHolder, "Alice must appear in waterfall")
	// Alice starts at 70 % and is diluted by two rounds
	assert.True(t, aliceHolder.InitialPct.Equal(decimal.NewFromFloat(0.7)),
		"Alice initial pct should be 0.7 (70%%)")
	assert.True(t, aliceHolder.FinalPct.LessThan(decimal.NewFromFloat(0.7)),
		"Alice should be diluted by end of Série A")

	// ── BSPCE plan summary ───────────────────────────────────────────────────
	require.Len(t, report.StockOption.Plans, 1)
	plan := report.StockOption.Plans[0]
	assert.Equal(t, int64(200), plan.Voted)
	assert.Equal(t, int64(150), plan.Attributed)
	assert.Equal(t, int64(150), plan.Outstanding)
	assert.Equal(t, int64(50), plan.Reserve) // 200 - 150

	// ── FastValo: Mode A (3× in 5y → IRR ≈ 24.57%) ──────────────────────────
	require.Len(t, report.FastValo.Scenarios, 2)
	modeA := report.FastValo.Scenarios[0]
	require.NotNil(t, modeA.IRRPct, "Mode A must produce IRR")
	// IRR = 3^(1/5) - 1 ≈ 0.2457
	assert.True(t, modeA.IRRPct.GreaterThan(decimal.NewFromFloat(0.24)),
		"IRR should be > 24%%, got %s", modeA.IRRPct.String())

	// ── FastValo: Mode D (500 k for 20% → PostMoney = 2500 k) ───────────────
	modeD := report.FastValo.Scenarios[1]
	require.NotNil(t, modeD.PostMoneyK)
	assert.True(t, modeD.PostMoneyK.Equal(decimal.NewFromFloat(2500).Round(2)),
		"PostMoney should be 2500 k, got %s", modeD.PostMoneyK.String())
	require.NotNil(t, modeD.PreMoneyK)
	assert.True(t, modeD.PreMoneyK.Equal(decimal.NewFromFloat(2000).Round(2)),
		"PreMoney should be 2000 k, got %s", modeD.PreMoneyK.String())
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 1: Total shares always equal sum of all holder shares
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_TotalSharesMatchHolderSum(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	for _, phase := range result.Report.Phases {
		var holderSum int64
		for _, pos := range phase.Positions {
			holderSum += pos.SharesAfterRound
		}
		assert.Equal(t, phase.TotalSharesAfter, holderSum,
			"Phase %d (%s): TotalSharesAfter (%d) must equal sum of holder shares (%d)",
			phase.PhaseNumber, phase.Label, phase.TotalSharesAfter, holderSum)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 2: Basic ownership percentages sum to 1 at every phase
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_OwnershipPctSum(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	for _, phase := range result.Report.Phases {
		var sumPct decimal.Decimal
		for _, pos := range phase.Positions {
			sumPct = sumPct.Add(pos.PctBasicAfter)
		}
		diff := sumPct.Sub(decimal.NewFromInt(1)).Abs()
		assert.True(t, diff.LessThan(decimal.NewFromFloat(0.001)),
			"Phase %d: ownership pct sum should be ≈1 (100%%), got %s", phase.PhaseNumber, sumPct)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 3: PostMoney grows monotonically in up-round sequence
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_ValuationGrowsMonotonically(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	phases := result.Report.Phases
	require.True(t, len(phases) >= 2)

	for i := 1; i < len(phases); i++ {
		prev := phases[i-1]
		curr := phases[i]
		if !prev.PostMoneyValuationK.IsZero() && !curr.PostMoneyValuationK.IsZero() {
			assert.True(t,
				curr.PostMoneyValuationK.GreaterThanOrEqual(prev.PostMoneyValuationK),
				"Phase %d (%s): post-money (%s) should be >= previous (%s)",
				curr.PhaseNumber, curr.Label,
				curr.PostMoneyValuationK, prev.PostMoneyValuationK)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 4: Existing shareholders never gain shares from a new round
// (unless they are the investor for that round)
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_ExistingHoldersSharesDontIncreaseAcrossRounds(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	phases := result.Report.Phases
	for i := 1; i < len(phases); i++ {
		prev := phases[i-1]
		curr := phases[i]

		// Build map from previous phase
		prevShares := make(map[string]int64, len(prev.Positions))
		for _, pos := range prev.Positions {
			prevShares[pos.Name] = pos.SharesAfterRound
		}

		for _, pos := range curr.Positions {
			prevCount := prevShares[pos.Name]
			// A holder can only gain shares at the round where they are the investor.
			// For all other holders: shares should be >= prev (could be same or higher after split).
			// In our test there are no splits, so shares for non-investor holders should be constant.
			if prevCount > 0 && pos.NewSharesReceived == 0 {
				assert.Equal(t, prevCount, pos.SharesAfterRound,
					"Phase %d→%d: %s had %d shares before and received 0 new → should still have %d",
					prev.PhaseNumber, curr.PhaseNumber, pos.Name, prevCount, prevCount)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 5: ValuationMultiple correctly derived from consecutive phases
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_ValuationMultiple(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	phases := result.Report.Phases
	require.True(t, len(phases) >= 3)

	// Phase 1: PostMoney = 750, Phase 2: PostMoney = 1800
	// Multiple = 1800/750 = 2.4
	phase2 := phases[2]
	phase1 := phases[1]
	expectedMultiple := phase2.PostMoneyValuationK.Div(phase1.PostMoneyValuationK).Round(4)
	assert.True(t, phase2.ValuationMultiple.Equal(expectedMultiple),
		"Phase 2 multiple should be %s, got %s", expectedMultiple, phase2.ValuationMultiple)
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 6: Emission premium is zero when share price < nominal value
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_EmissionPremiumNeverNegative(t *testing.T) {
	company := saascoCompany()
	company.NominalValueCents = 100_000 // 1 k per share nominal → higher than any share price here
	shareholders := saascoShareholders()
	rounds := saascoRounds()

	result := ComputeIngeFi(company, shareholders, nil, rounds, nil, nil)

	for _, phase := range result.Report.Phases[1:] { // skip phase 0
		assert.True(t, phase.EmissionPremium.PerShareK.GreaterThanOrEqual(decimal.Zero),
			"Phase %d: emission premium per share should never be negative, got %s",
			phase.PhaseNumber, phase.EmissionPremium.PerShareK)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 7: BSPCE plan reserve + outstanding + exercised + cancelled = voted
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_PlanVotedAccountability(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()
	plans := []model.StockOptionPlan{saascoPlan()}
	grants := saascoGrants()

	ingeFiRes := ComputeIngeFi(company, shareholders, nil, rounds, grants, plans)
	soReport := ComputeStockOptions(company, shareholders, rounds, plans, grants, ingeFiRes)

	for _, planSummary := range soReport.Plans {
		total := planSummary.Reserve + planSummary.Outstanding + planSummary.Exercised + planSummary.Cancelled
		assert.Equal(t, planSummary.Voted, total,
			"Plan %s: reserve+outstanding+exercised+cancelled should equal voted",
			planSummary.PlanLabel)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 8: ValuationScenarioResult has no nil fields for complete inputs
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_AllValuationModesReturnResults(t *testing.T) {
	multiple := decimal.NewFromFloat(3.0)
	horizon := decimal.NewFromFloat(5.0)
	inv := decimal.NewFromFloat(200.0)
	yield := decimal.NewFromFloat(10.0)
	pct := decimal.NewFromFloat(20.0)
	exitVal := decimal.NewFromFloat(2000.0)
	newMoney := decimal.NewFromFloat(500.0)
	discount := decimal.NewFromFloat(10.0)

	tests := []struct {
		name  string
		vs    model.ValuationScenario
		check func(t *testing.T, r model.ValuationScenarioResult)
	}{
		{
			name: "Mode A: multiple→IRR",
			vs: model.ValuationScenario{
				CalcType:      model.ValScenMultipleToIRR,
				MoneyMultiple: &multiple, HorizonYears: &horizon,
			},
			check: func(t *testing.T, r model.ValuationScenarioResult) {
				require.NotNil(t, r.IRRPct, "Mode A must set IRRPct")
				require.NotNil(t, r.MoneyMultiple, "Mode A must echo MoneyMultiple")
			},
		},
		{
			name: "Mode B: investment→terminal",
			vs: model.ValuationScenario{
				CalcType:    model.ValScenInvestmentToTerminal,
				InvestmentK: &inv, AnnualYieldPct: &yield, HorizonYears: &horizon,
			},
			check: func(t *testing.T, r model.ValuationScenarioResult) {
				require.NotNil(t, r.TerminalValueK, "Mode B must set TerminalValueK")
				require.NotNil(t, r.MoneyMultiple, "Mode B must set MoneyMultiple")
			},
		},
		{
			name: "Mode C: investment+pct→IRR+NPV",
			vs: model.ValuationScenario{
				CalcType:    model.ValScenInvestmentPctToIRR,
				InvestmentK: &inv, FinalInvestorPct: &pct,
				ExitCompanyValueK: &exitVal, HorizonYears: &horizon, DiscountRatePct: &discount,
			},
			check: func(t *testing.T, r model.ValuationScenarioResult) {
				require.NotNil(t, r.IRRPct, "Mode C must set IRRPct")
				require.NotNil(t, r.InvestorShareK, "Mode C must set InvestorShareK")
				require.NotNil(t, r.NPVK, "Mode C must set NPVK")
			},
		},
		{
			name: "Mode D: newmoney+pct→premoney",
			vs: model.ValuationScenario{
				CalcType:  model.ValScenNewMoneyToPremoney,
				NewMoneyK: &newMoney, FinalInvestorPct: &pct,
			},
			check: func(t *testing.T, r model.ValuationScenarioResult) {
				require.NotNil(t, r.PreMoneyK, "Mode D must set PreMoneyK")
				require.NotNil(t, r.PostMoneyK, "Mode D must set PostMoneyK")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ComputeValuation(tc.vs)
			tc.check(t, result)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 9: ComputeCapTable is idempotent (same input → same output)
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_ComputeCapTable_Idempotent(t *testing.T) {
	company := saascoCompany()
	shareholders := saascoShareholders()
	rounds := saascoRounds()
	plans := []model.StockOptionPlan{saascoPlan()}
	grants := saascoGrants()
	vScenarios := saascoValuations()

	report1 := ComputeCapTable(company, shareholders, nil, rounds, plans, grants, vScenarios)
	report2 := ComputeCapTable(company, shareholders, nil, rounds, plans, grants, vScenarios)

	require.Equal(t, len(report1.IngeFi.Phases), len(report2.IngeFi.Phases))
	for i := range report1.IngeFi.Phases {
		assert.Equal(t,
			report1.IngeFi.Phases[i].PostMoneyValuationK.String(),
			report2.IngeFi.Phases[i].PostMoneyValuationK.String(),
			"Phase %d: post-money should be identical on repeated compute", i)
		assert.Equal(t,
			len(report1.IngeFi.Phases[i].Positions),
			len(report2.IngeFi.Phases[i].Positions),
			"Phase %d: position count should be identical on repeated compute", i)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-regression 10: Empty inputs produce valid zero-state report
// ─────────────────────────────────────────────────────────────────────────────

func TestNonRegression_EmptyInputs_NoError(t *testing.T) {
	company := model.CapTableCompany{
		NominalValueCents: 1,
		FounderAlertPct:   decimal.NewFromInt(20),
	}

	// Should not panic
	report := ComputeCapTable(company, nil, nil, nil, nil, nil, nil)

	// Single phase 0 (founding) with no positions
	require.Len(t, report.IngeFi.Phases, 1, "founding phase should always be present")
	assert.Empty(t, report.IngeFi.Phases[0].Positions)
	assert.Empty(t, report.StockOption.Plans)
	assert.Empty(t, report.FastValo.Scenarios)
	assert.Empty(t, report.Warnings)
}
