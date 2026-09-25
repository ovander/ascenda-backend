package model

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompetitionParams_Validate(t *testing.T) {
	valid := func() CompetitionParams {
		var p CompetitionParams
		p.Events[0], p.Cuts[0], p.Top10s[0], p.Wins[0] = 20, 12, 3, 2
		p.CaddieShare[0], p.CoachShare[0] = decimal.NewFromFloat(0.08), decimal.NewFromFloat(0.05)
		return p
	}
	require.NoError(t, valid().Validate())

	cases := map[string]struct {
		mutate func(*CompetitionParams)
		want   string
	}{
		"cuts above events":     {func(p *CompetitionParams) { p.Cuts[0] = 21 }, "year 1: cuts made (21) exceed events played (20)"},
		"wins+top10 above cuts": {func(p *CompetitionParams) { p.Wins[2], p.Cuts[2], p.Events[2] = 3, 2, 5 }, "year 3: wins plus top-10 finishes (3) exceed cuts made (2)"},
		"negative events":       {func(p *CompetitionParams) { p.Events[1] = -1 }, "year 2: events must not be negative"},
		"negative travel":       {func(p *CompetitionParams) { p.TravelPerEvent[4] = decimal.NewFromInt(-5) }, "year 5: travelPerEvent must not be negative"},
		"negative coach fee":    {func(p *CompetitionParams) { p.CoachAnnualFee[3] = decimal.NewFromInt(-1) }, "year 4: coachAnnualFee must not be negative"},
		"share above one":       {func(p *CompetitionParams) { p.CoachShare[0] = decimal.NewFromInt(7) }, "year 1: coachShare must be a fraction between 0 and 1"},
		"shares together above 1": {func(p *CompetitionParams) {
			p.CaddieShare[0], p.CoachShare[0] = decimal.NewFromFloat(0.6), decimal.NewFromFloat(0.5)
		}, "year 1: caddie and coach shares together exceed 100 % of winnings"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := valid()
			tc.mutate(&p)
			err := p.Validate()
			require.Error(t, err)
			assert.Equal(t, tc.want, err.Error())
		})
	}
}

func TestContractParams_Validate(t *testing.T) {
	ok := ContractParams{Contracts: []ContractLine{{Partner: "Club", Amounts: [5]decimal.Decimal{decimal.NewFromInt(4000)}}}}
	require.NoError(t, ok.Validate())

	neg := ContractParams{Contracts: []ContractLine{{Partner: "X", Amounts: [5]decimal.Decimal{decimal.Zero, decimal.NewFromInt(-1)}}}}
	assert.EqualError(t, neg.Validate(), "contract 1, year 2: amount must not be negative")

	bonus := ContractParams{Contracts: []ContractLine{{BonusPerWin: decimal.NewFromInt(-1)}}}
	assert.EqualError(t, bonus.Validate(), "contract 1: bonus per win must not be negative")

	many := ContractParams{Contracts: make([]ContractLine, MaxContractsPerProduct+1)}
	assert.Error(t, many.Validate())
}

func TestValidateDriverParams_OnlyChecksDriversWithRules(t *testing.T) {
	bad := json.RawMessage(`{"events":[1,0,0,0,0],"cuts":[2,0,0,0,0]}`)
	assert.Error(t, ValidateDriverParams(DriverCompetition, bad))
	assert.NoError(t, ValidateDriverParams(DriverSaaS, bad), "other drivers are not validated here")
	assert.NoError(t, ValidateDriverParams(DriverCompetition, nil), "params not set yet")
	assert.Error(t, ValidateDriverParams(DriverContract, json.RawMessage(`{"contracts":"nope"}`)))
}
