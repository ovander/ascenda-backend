package compute

// Athlete drivers — see model/athlete_drivers.go for the formulas.

import (
	"encoding/json"

	"ascenda/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DriverContext carries scenario-level facts that a driver needs from other
// products of the same scenario.
type DriverContext struct {
	// Wins[y] is the total number of wins entered on the scenario's
	// competition products for year y+1. The contract driver pays each
	// active contract's bonus per win on it.
	Wins [MaxYears]int64
}

// BuildDriverContext derives the DriverContext of a scenario from its
// products. Products whose competition parameters cannot be read contribute
// nothing; they fail on their own when their driver is applied.
func BuildDriverContext(products []model.Product) DriverContext {
	var ctx DriverContext
	for _, p := range products {
		if p.DriverType != model.DriverCompetition || len(p.DriverParams) == 0 {
			continue
		}
		var params model.CompetitionParams
		if err := json.Unmarshal(p.DriverParams, &params); err != nil {
			continue
		}
		for y := 0; y < MaxYears; y++ {
			ctx.Wins[y] += int64(params.Wins[y])
		}
	}
	return ctx
}

// competitionGains returns the year's prize money: results × prize
// economics plus prize money earned outside the main circuit.
func competitionGains(p model.CompetitionParams, y int) decimal.Decimal {
	wins := decimal.NewFromInt(int64(p.Wins[y]))
	top10s := decimal.NewFromInt(int64(p.Top10s[y]))
	ordinary := int64(p.Cuts[y]) - int64(p.Wins[y]) - int64(p.Top10s[y])
	if ordinary < 0 {
		ordinary = 0
	}
	return wins.Mul(p.PrizePerWin[y]).
		Add(top10s.Mul(p.PrizePerTop10[y])).
		Add(decimal.NewFromInt(ordinary).Mul(p.PrizePerCut[y])).
		Add(p.OtherPrizeMoney[y])
}

func applyCompetitionDriver(p model.CompetitionParams, bundle ProductInputBundle, productID uuid.UUID) ProductInputBundle {
	one := decimal.NewFromInt(1)
	newVolumes := make([]model.ProductSalesVolume, 0, MaxYears)

	for y := 0; y < MaxYears; y++ {
		gains := competitionGains(p, y)
		events := int64(p.Events[y])
		directCost := p.EntryFeePerEvent[y].
			Add(p.TravelPerEvent[y]).
			Add(p.CaddieFeePerEvent[y]).
			Mul(decimal.NewFromInt(events)).
			Add(p.CoachFixed(y)).
			Add(p.CaddieShare[y].Add(p.CoachShare[y]).Mul(gains))

		var volume int64
		unitPrice, unitCost := decimal.Zero, decimal.Zero
		switch {
		case events > 0:
			volume = events
			unitPrice = gains.DivRound(decimal.NewFromInt(events), 10)
			unitCost = directCost.DivRound(decimal.NewFromInt(events), 10)
		case gains.IsPositive() || directCost.IsPositive():
			// No event on the main circuit, but prize money elsewhere or a
			// coach still under contract: carry the year as one unit so
			// neither the revenue nor the cost is lost.
			volume = 1
			unitPrice = gains
			unitCost = directCost
		}

		newVolumes = append(newVolumes, model.ProductSalesVolume{
			ProductID: productID,
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: volume,
		})

		bundle.Assumptions[y].BaseUnitPrice = unitPrice
		bundle.Assumptions[y].RawMaterialCost = unitCost
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		bundle.Assumptions[y].PriceCoefficient = one
	}

	bundle.Volumes = newVolumes
	return bundle
}

// contractRevenue returns the year's contract revenue: the fixed amounts of
// every contract plus, for each win, the bonus of every contract active that
// year (amount > 0).
func contractRevenue(p model.ContractParams, y int, wins int64) decimal.Decimal {
	total, bonus := decimal.Zero, decimal.Zero
	for _, c := range p.Contracts {
		total = total.Add(c.Amounts[y])
		if c.Amounts[y].IsPositive() {
			bonus = bonus.Add(c.BonusPerWin)
		}
	}
	return total.Add(bonus.Mul(decimal.NewFromInt(wins)))
}

func applyContractDriver(p model.ContractParams, bundle ProductInputBundle, productID uuid.UUID, ctx DriverContext) ProductInputBundle {
	one := decimal.NewFromInt(1)
	newVolumes := make([]model.ProductSalesVolume, 0, MaxYears)

	for y := 0; y < MaxYears; y++ {
		revenue := contractRevenue(p, y, ctx.Wins[y])

		var volume int64
		if revenue.IsPositive() {
			volume = 1 // one unit per year: the year's contract bundle
		}
		newVolumes = append(newVolumes, model.ProductSalesVolume{
			ProductID: productID,
			YearIndex: y + 1,
			Zone:      model.ZoneFrance,
			Channel:   model.ChannelDirect,
			UnitsSold: volume,
		})

		bundle.Assumptions[y].BaseUnitPrice = revenue
		bundle.Assumptions[y].RawMaterialCost = decimal.Zero
		bundle.Assumptions[y].RoyaltiesCost = decimal.Zero
		bundle.Assumptions[y].LogisticsCost = decimal.Zero
		bundle.Assumptions[y].CostCoefficient = one
		bundle.Assumptions[y].PriceCoefficient = one
	}

	bundle.Volumes = newVolumes
	return bundle
}
