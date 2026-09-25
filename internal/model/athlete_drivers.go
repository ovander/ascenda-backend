package model

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

// Athlete business drivers: a professional athlete's revenue model, split
// into prize money (competition driver) and contract revenue such as
// sponsorship or image rights (contract driver). Each driver belongs to its
// own product so the streams stay on separate revenue lines.

// CompetitionParams drives prize money and per-event costs from results.
//
//	OrdinaryCuts[y] = Cuts[y] − Wins[y] − Top10s[y]
//	Gains[y]        = Wins × PrizePerWin + Top10s × PrizePerTop10
//	                + OrdinaryCuts × PrizePerCut + OtherPrizeMoney
//	Volume[y]       = Events[y]                (1 when no event but other prize money)
//	UnitPrice[y]    = Gains[y] / Volume[y]
//	DirectCost[y]   = Events × (EntryFee + Travel + CaddieFee)   (per event)
//	                + CoachAnnualFee                             (per year)
//	                + (CaddieShare + CoachShare) × Gains
//	UnitCost[y]     = DirectCost[y] / Volume[y]
//
// Top10s excludes wins. Shares are fractions of total gains (0.07 = 7 %).
// The caddie is paid per event; the coach is paid a fixed fee per year;
// both also take a share of winnings.
type CompetitionParams struct {
	// Circuit[0..4] names the tour played that year (informational; the UI
	// uses it to pre-fill the prize economics).
	Circuit [5]string `json:"circuit"`

	Events [5]FlexInt64 `json:"events"`
	Cuts   [5]FlexInt64 `json:"cuts"`
	Top10s [5]FlexInt64 `json:"top10s"`
	Wins   [5]FlexInt64 `json:"wins"`

	PrizePerWin     [5]decimal.Decimal `json:"prizePerWin"`
	PrizePerTop10   [5]decimal.Decimal `json:"prizePerTop10"`
	PrizePerCut     [5]decimal.Decimal `json:"prizePerCut"`
	OtherPrizeMoney [5]decimal.Decimal `json:"otherPrizeMoney"`

	EntryFeePerEvent  [5]decimal.Decimal `json:"entryFeePerEvent"`
	TravelPerEvent    [5]decimal.Decimal `json:"travelPerEvent"`
	CaddieFeePerEvent [5]decimal.Decimal `json:"caddieFeePerEvent"`
	CaddieShare       [5]decimal.Decimal `json:"caddieShare"`
	CoachAnnualFee    [5]decimal.Decimal `json:"coachAnnualFee"`
	CoachShare        [5]decimal.Decimal `json:"coachShare"`

	// CoachFeePerEvent is the coach's fixed fee per event from the first
	// version of the driver, before the coach fee became annual. It is
	// still read so that parameters saved then keep their cost
	// (fee × events, added to the annual fee); the forms no longer write it.
	CoachFeePerEvent [5]decimal.Decimal `json:"coachFeePerEvent,omitempty"`
}

// CoachFixed returns the coach's fixed cost for year index y: the annual fee
// plus any legacy per-event fee times the events played.
func (p CompetitionParams) CoachFixed(y int) decimal.Decimal {
	return p.CoachAnnualFee[y].Add(p.CoachFeePerEvent[y].Mul(decimal.NewFromInt(int64(p.Events[y]))))
}

// ContractLine is one sponsorship or image-rights contract.
type ContractLine struct {
	Partner string `json:"partner"`
	// Amounts[0..4] is the fixed contract value per year; 0 = not active.
	Amounts [5]decimal.Decimal `json:"amounts"`
	// BonusPerWin is paid for each competition win in a year the contract
	// is active (amount > 0).
	BonusPerWin decimal.Decimal `json:"bonusPerWin"`
}

// ContractParams drives contract revenue.
//
//	Revenue[y] = Σ Amounts[y] + Wins[y] × Σ BonusPerWin (contracts active in y)
//
// Wins come from the competition products of the same scenario.
type ContractParams struct {
	Contracts []ContractLine `json:"contracts"`
}

// MaxContractsPerProduct bounds the contract list of one product.
const MaxContractsPerProduct = 50

var decimalOne = decimal.NewFromInt(1)

// Validate reports the first inconsistency in the parameters, naming the
// year (1-based) and the field so the user can fix it.
func (p CompetitionParams) Validate() error {
	for y := 0; y < 5; y++ {
		year := y + 1
		events, cuts, top10s, wins := int64(p.Events[y]), int64(p.Cuts[y]), int64(p.Top10s[y]), int64(p.Wins[y])
		for name, v := range map[string]int64{"events": events, "cuts": cuts, "top10s": top10s, "wins": wins} {
			if v < 0 {
				return fmt.Errorf("year %d: %s must not be negative", year, name)
			}
		}
		if cuts > events {
			return fmt.Errorf("year %d: cuts made (%d) exceed events played (%d)", year, cuts, events)
		}
		if wins+top10s > cuts {
			return fmt.Errorf("year %d: wins plus top-10 finishes (%d) exceed cuts made (%d)", year, wins+top10s, cuts)
		}
		amounts := map[string]decimal.Decimal{
			"prizePerWin": p.PrizePerWin[y], "prizePerTop10": p.PrizePerTop10[y], "prizePerCut": p.PrizePerCut[y],
			"otherPrizeMoney": p.OtherPrizeMoney[y], "entryFeePerEvent": p.EntryFeePerEvent[y],
			"travelPerEvent": p.TravelPerEvent[y], "caddieFeePerEvent": p.CaddieFeePerEvent[y],
			"coachAnnualFee": p.CoachAnnualFee[y], "coachFeePerEvent": p.CoachFeePerEvent[y],
		}
		for name, v := range amounts {
			if v.IsNegative() {
				return fmt.Errorf("year %d: %s must not be negative", year, name)
			}
		}
		for name, v := range map[string]decimal.Decimal{"caddieShare": p.CaddieShare[y], "coachShare": p.CoachShare[y]} {
			if v.IsNegative() || v.GreaterThan(decimalOne) {
				return fmt.Errorf("year %d: %s must be a fraction between 0 and 1", year, name)
			}
		}
		if p.CaddieShare[y].Add(p.CoachShare[y]).GreaterThan(decimalOne) {
			return fmt.Errorf("year %d: caddie and coach shares together exceed 100 %% of winnings", year)
		}
		if len(p.Circuit[y]) > 100 {
			return fmt.Errorf("year %d: circuit name is longer than 100 characters", year)
		}
	}
	return nil
}

// Validate reports the first invalid contract.
func (p ContractParams) Validate() error {
	if len(p.Contracts) > MaxContractsPerProduct {
		return fmt.Errorf("at most %d contracts per product", MaxContractsPerProduct)
	}
	for i, c := range p.Contracts {
		if len(c.Partner) > 255 {
			return fmt.Errorf("contract %d: partner name is longer than 255 characters", i+1)
		}
		if c.BonusPerWin.IsNegative() {
			return fmt.Errorf("contract %d: bonus per win must not be negative", i+1)
		}
		for y, a := range c.Amounts {
			if a.IsNegative() {
				return fmt.Errorf("contract %d, year %d: amount must not be negative", i+1, y+1)
			}
		}
	}
	return nil
}

// ValidateDriverParams checks the parameters of the drivers that define
// validation rules (competition and contract). Other drivers, and products
// whose parameters are not set yet, pass unchanged.
func ValidateDriverParams(dt DriverType, raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	switch dt {
	case DriverCompetition:
		var p CompetitionParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("invalid competition parameters: %w", err)
		}
		return p.Validate()
	case DriverContract:
		var p ContractParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("invalid contract parameters: %w", err)
		}
		return p.Validate()
	}
	return nil
}
