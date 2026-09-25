package compute

// Acceptance tests for the athlete drivers against a reference five-year
// forecast of a touring golf professional (spreadsheet model, three career
// scenarios). Inputs are the spreadsheet's assumptions; expected values are
// the spreadsheet's own results: prize money (main circuit + other events),
// direct costs (entry, travel, caddie fixed + share of winnings per event,
// and the coach's annual fee), and sponsorship and image-rights revenue
// including the bonus per win.

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tourEconomics struct{ win, top10, cut float64 }

var (
	alps      = tourEconomics{7455, 1647, 692}
	challenge = tourEconomics{45000, 12000, 2500}
	dpWorld   = tourEconomics{380000, 90000, 12000}
)

type contractRow struct {
	amounts [5]float64
	bonus   float64
}

type referenceScenario struct {
	name                                    string
	tours                                   [5]tourEconomics
	events, cuts, top10s, wins              [5]int64
	other, entry, travel, caddieFix, caddie [5]float64
	coachAnnual                             [5]float64
	sponsoring, image                       []contractRow
	// expected spreadsheet results
	expGains, expCosts, expSponsoring, expImage [5]float64
}

var referenceScenarios = []referenceScenario{
	{
		name:   "pessimistic",
		tours:  [5]tourEconomics{alps, challenge, challenge, challenge, dpWorld},
		events: [5]int64{21, 22, 24, 24, 26}, cuts: [5]int64{13, 10, 13, 16, 14},
		top10s: [5]int64{3, 1, 2, 5, 1}, wins: [5]int64{4, 0, 0, 1, 0},
		other: [5]float64{1776.67, 2000, 2000, 2000, 0},
		entry: [5]float64{350, 300, 300, 300, 0}, travel: [5]float64{1100, 2100, 2100, 2200, 3800},
		caddieFix: [5]float64{0, 1000, 1000, 1200, 2000}, caddie: [5]float64{0, 0.07, 0.07, 0.07, 0.08},
		coachAnnual: [5]float64{8000, 14000, 14000, 15000, 30000},
		sponsoring: []contractRow{
			{[5]float64{0, 6000, 6000, 6000, 30000}, 500},
			{[5]float64{0, 4000, 4000, 4000, 5000}, 0},
			{[5]float64{0, 0, 0, 0, 10000}, 0},
		},
		image:         []contractRow{{[5]float64{0, 0, 0, 0, 10000}, 0}},
		expGains:      [5]float64{40689.67, 36500, 53500, 132000, 246000},
		expCosts:      [5]float64{30450 + 8000, 77355 + 14000, 85345 + 14000, 98040 + 15000, 170480 + 30000},
		expSponsoring: [5]float64{0, 10000, 10000, 10500, 45000},
		expImage:      [5]float64{0, 0, 0, 0, 10000},
	},
	{
		name:   "medium",
		tours:  [5]tourEconomics{alps, challenge, challenge, dpWorld, dpWorld},
		events: [5]int64{21, 24, 26, 26, 28}, cuts: [5]int64{13, 14, 18, 15, 18},
		top10s: [5]int64{3, 2, 5, 1, 3}, wins: [5]int64{4, 0, 1, 0, 0},
		other: [5]float64{1776.67, 2000, 3000, 0, 0},
		entry: [5]float64{350, 300, 300, 0, 0}, travel: [5]float64{1100, 2200, 2300, 3800, 4000},
		caddieFix: [5]float64{0, 1200, 1300, 2000, 2200}, caddie: [5]float64{0, 0.07, 0.07, 0.08, 0.08},
		coachAnnual: [5]float64{8000, 15000, 18000, 30000, 35000},
		sponsoring: []contractRow{
			{[5]float64{0, 6000, 6000, 30000, 50000}, 1000},
			{[5]float64{0, 4000, 4000, 5000, 5000}, 0},
			{[5]float64{0, 0, 0, 10000, 30000}, 1000},
		},
		image:         []contractRow{{[5]float64{0, 0, 0, 10000, 25000}, 0}},
		expGains:      [5]float64{40689.67, 56000, 138000, 258000, 450000},
		expCosts:      [5]float64{30450 + 8000, 92720 + 15000, 111060 + 18000, 171440 + 30000, 209600 + 35000},
		expSponsoring: [5]float64{0, 10000, 11000, 45000, 85000},
		expImage:      [5]float64{0, 0, 0, 10000, 25000},
	},
	{
		name:   "optimistic",
		tours:  [5]tourEconomics{alps, challenge, dpWorld, dpWorld, dpWorld},
		events: [5]int64{21, 24, 26, 28, 28}, cuts: [5]int64{13, 17, 17, 20, 21},
		top10s: [5]int64{3, 6, 2, 4, 5}, wins: [5]int64{4, 1, 0, 1, 1},
		other: [5]float64{1776.67, 3000, 0, 0, 0},
		entry: [5]float64{350, 300, 0, 0, 0}, travel: [5]float64{1100, 2300, 3800, 4200, 4500},
		caddieFix: [5]float64{0, 1300, 2000, 2200, 2500}, caddie: [5]float64{0, 0.07, 0.08, 0.08, 0.09},
		coachAnnual: [5]float64{8000, 18000, 30000, 40000, 50000},
		sponsoring: []contractRow{
			{[5]float64{0, 6000, 30000, 50000, 70000}, 2000},
			{[5]float64{0, 4000, 5000, 5000, 5000}, 0},
			{[5]float64{0, 0, 10000, 30000, 50000}, 2000},
		},
		image:         []contractRow{{[5]float64{0, 0, 10000, 25000, 40000}, 0}},
		expGains:      [5]float64{40689.67, 145000, 360000, 920000, 1010000},
		expCosts:      [5]float64{30450 + 8000, 103750 + 18000, 179600 + 30000, 252800 + 40000, 286900 + 50000},
		expSponsoring: [5]float64{0, 12000, 45000, 89000, 129000},
		expImage:      [5]float64{0, 0, 10000, 25000, 40000},
	},
}

func decf(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

func (s referenceScenario) competitionParams() model.CompetitionParams {
	var p model.CompetitionParams
	for y := 0; y < 5; y++ {
		p.Events[y], p.Cuts[y], p.Top10s[y], p.Wins[y] = model.FlexInt64(s.events[y]), model.FlexInt64(s.cuts[y]), model.FlexInt64(s.top10s[y]), model.FlexInt64(s.wins[y])
		p.PrizePerWin[y], p.PrizePerTop10[y], p.PrizePerCut[y] = decf(s.tours[y].win), decf(s.tours[y].top10), decf(s.tours[y].cut)
		p.OtherPrizeMoney[y] = decf(s.other[y])
		p.EntryFeePerEvent[y], p.TravelPerEvent[y] = decf(s.entry[y]), decf(s.travel[y])
		p.CaddieFeePerEvent[y], p.CaddieShare[y] = decf(s.caddieFix[y]), decf(s.caddie[y])
		p.CoachAnnualFee[y] = decf(s.coachAnnual[y])
	}
	return p
}

func contractParams(rows []contractRow) model.ContractParams {
	var p model.ContractParams
	for _, r := range rows {
		var c model.ContractLine
		for y := 0; y < 5; y++ {
			c.Amounts[y] = decf(r.amounts[y])
		}
		c.BonusPerWin = decf(r.bonus)
		p.Contracts = append(p.Contracts, c)
	}
	return p
}

func product(t *testing.T, name string, dt model.DriverType, params any) model.Product {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	return model.Product{TenantScoped: model.TenantScoped{ID: uuid.New()}, Name: name, DriverType: dt, DriverParams: raw}
}

// revenueOf runs the product through the driver and the standard revenue
// engine, returning turnover and COGS per year.
func revenueOf(t *testing.T, p model.Product, ctx DriverContext) (turnover, cogs [5]decimal.Decimal) {
	t.Helper()
	bundle, err := ApplyDriverComputeWithContext(p, ProductInputBundle{}, ctx)
	require.NoError(t, err)
	summary := ComputeProductRevenue(p, bundle, model.PlanConfig{ForecastStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	for y := 0; y < 5; y++ {
		turnover[y], cogs[y] = summary.Years[y].Turnover, summary.Years[y].COGS
	}
	return turnover, cogs
}

func assertCents(t *testing.T, want float64, got decimal.Decimal, msg string, args ...any) {
	t.Helper()
	label := fmt.Sprintf(msg, args...)
	assert.True(t, got.Round(2).Equal(decf(want).Round(2)), "%s: want %.2f, got %s", label, want, got.Round(2))
}

func TestAthleteDrivers_ReproduceReferenceForecast(t *testing.T) {
	for _, s := range referenceScenarios {
		t.Run(s.name, func(t *testing.T) {
			prize := product(t, "Prize money", model.DriverCompetition, s.competitionParams())
			sponsoring := product(t, "Sponsoring", model.DriverContract, contractParams(s.sponsoring))
			image := product(t, "Image rights", model.DriverContract, contractParams(s.image))
			ctx := BuildDriverContext([]model.Product{prize, sponsoring, image})

			prizeRevenue, prizeCosts := revenueOf(t, prize, ctx)
			sponsoringRevenue, sponsoringCosts := revenueOf(t, sponsoring, ctx)
			imageRevenue, _ := revenueOf(t, image, ctx)

			for y := 0; y < 5; y++ {
				assertCents(t, s.expGains[y], prizeRevenue[y], "year %d prize money", y+1)
				assertCents(t, s.expCosts[y], prizeCosts[y], "year %d direct costs", y+1)
				assertCents(t, s.expSponsoring[y], sponsoringRevenue[y], "year %d sponsoring", y+1)
				assertCents(t, s.expImage[y], imageRevenue[y], "year %d image rights", y+1)
				assert.True(t, sponsoringCosts[y].IsZero(), "contracts carry no direct cost")
			}
		})
	}
}

func TestCompetitionDriver_CoachIsAnnualFeePlusShareOfWinnings(t *testing.T) {
	var p model.CompetitionParams
	p.Events[0], p.Cuts[0], p.Wins[0] = 10, 5, 1
	p.PrizePerWin[0], p.PrizePerCut[0] = decf(40000), decf(2000) // gains = 40 000 + 4 × 2 000 = 48 000
	p.CaddieFeePerEvent[0], p.CaddieShare[0] = decf(1000), decf(0.07)
	p.CoachAnnualFee[0], p.CoachShare[0] = decf(12000), decf(0.05)

	revenue, costs := revenueOf(t, product(t, "Prize", model.DriverCompetition, p), DriverContext{})

	assertCents(t, 48000, revenue[0], "gains")
	// caddie 10 × 1 000, coach 12 000 for the year, shares 12 % × 48 000
	assertCents(t, 10000+12000+5760, costs[0], "caddie per event, coach per year, both plus a share of winnings")
}

func TestCompetitionDriver_LegacyPerEventCoachFeeKeepsItsCost(t *testing.T) {
	// Parameters saved by the first version of the driver carry a coach fee
	// per event; it is costed as fee × events on top of the annual fee.
	raw := []byte(`{"events":[8,0,0,0,0],"cuts":[4,0,0,0,0],"prizePerCut":[1000,0,0,0,0],"coachFeePerEvent":[500,0,0,0,0]}`)
	p := model.Product{TenantScoped: model.TenantScoped{ID: uuid.New()}, DriverType: model.DriverCompetition, DriverParams: raw}

	revenue, costs := revenueOf(t, p, DriverContext{})

	assertCents(t, 4000, revenue[0], "gains")
	assertCents(t, 4000, costs[0], "8 events × 500 legacy coach fee")
}

func TestCompetitionDriver_CoachPaidInAYearWithoutEventsIsKept(t *testing.T) {
	var p model.CompetitionParams
	p.CoachAnnualFee[1] = decf(9000)

	revenue, costs := revenueOf(t, product(t, "Prize", model.DriverCompetition, p), DriverContext{})

	assert.True(t, revenue[1].IsZero())
	assertCents(t, 9000, costs[1], "the coach's annual fee is a cost even without events")
}

func TestCompetitionDriver_OtherPrizeMoneyWithoutEventsIsKept(t *testing.T) {
	var p model.CompetitionParams
	p.OtherPrizeMoney[2] = decf(3000)
	p.CaddieShare[2] = decf(0.1)

	revenue, costs := revenueOf(t, product(t, "Prize", model.DriverCompetition, p), DriverContext{})

	assertCents(t, 3000, revenue[2], "prize money outside the main circuit")
	assertCents(t, 300, costs[2], "share of winnings still applies")
	assert.True(t, revenue[0].IsZero(), "a year with nothing entered stays at zero")
}

func TestContractDriver_BonusOnlyForContractsActiveThatYear(t *testing.T) {
	p := model.ContractParams{Contracts: []model.ContractLine{
		{Partner: "Active", Amounts: [5]decimal.Decimal{decf(10000)}, BonusPerWin: decf(1000)},
		{Partner: "Not signed yet", BonusPerWin: decf(5000)},
	}}
	ctx := DriverContext{Wins: [MaxYears]int64{3}}

	revenue, _ := revenueOf(t, product(t, "Sponsoring", model.DriverContract, p), ctx)

	assertCents(t, 13000, revenue[0], "10 000 + 3 wins × 1 000; the inactive contract pays no bonus")
}

func TestBuildDriverContext_SumsWinsAcrossCompetitionProducts(t *testing.T) {
	var a, b model.CompetitionParams
	a.Wins[1], b.Wins[1] = 2, 1
	products := []model.Product{
		product(t, "Main tour", model.DriverCompetition, a),
		product(t, "Second tour", model.DriverCompetition, b),
		product(t, "Sponsoring", model.DriverContract, model.ContractParams{}),
		{DriverType: model.DriverCompetition, DriverParams: json.RawMessage(`{not json`)},
	}

	ctx := BuildDriverContext(products)

	assert.Equal(t, [MaxYears]int64{0, 3, 0, 0, 0}, ctx.Wins)
}

func TestCompetitionDriver_InvalidResultsAreRejected(t *testing.T) {
	var p model.CompetitionParams
	p.Events[0], p.Cuts[0] = 5, 6

	_, err := ApplyDriverComputeWithContext(product(t, "Prize", model.DriverCompetition, p), ProductInputBundle{}, DriverContext{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "year 1: cuts made (6) exceed events played (5)")
}
