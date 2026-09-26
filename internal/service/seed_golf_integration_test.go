//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ascenda/internal/config"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"ascenda/internal/testdb"
)

// TestGolfDemo_SeedsAndComputes seeds the golf demo into the test database and
// computes the whole plan the way the app does: the athlete drivers produce the
// revenue, the plan carries no office rent, royalty or stock, and cash stays
// positive.
func TestGolfDemo_SeedsAndComputes(t *testing.T) {
	ctx := context.Background()
	db := testdb.Tx(t, integrationDB)
	svc := NewServiceBundle(repo.NewRepoBundle(db), &config.Config{Env: "test"}, logrus.NewEntry(logrus.New()))
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001") // seeded by migration 000013

	require.NoError(t, svc.Seed.createDemoPlan(ctx, tenantID, uuid.New(), golfDemo()))

	var scenario model.Scenario
	require.NoError(t, db.Joins("JOIN business_plans ON business_plans.id = scenarios.plan_id").
		Where("business_plans.tenant_id = ? AND business_plans.is_demo", tenantID).
		First(&scenario).Error)
	out, err := svc.Report.GetFullReport(ctx, tenantID, scenario.ID)
	require.NoError(t, err)

	sales := [5]float64{97000, 193500, 399000, 752000, 1381000}
	for y, year := range out.PnL.Years {
		assert.InDelta(t, sales[y], year.Sales.InexactFloat64(), 0.01, "Y%d sales", y+1)
		assert.True(t, year.StoredProduction.IsZero(), "Y%d: no stock, so no stored production", y+1)
	}
	assert.True(t, out.PnL.Years[4].NetProfit.GreaterThan(out.PnL.Years[0].NetProfit), "profit grows with the career")

	for _, sub := range out.Opex.Subcategories {
		for _, line := range sub.Lines {
			if line.LineID == model.LinePropertyRentals || line.LineID == model.LineRoyaltyPatents {
				for y := range line.Years {
					assert.True(t, line.Years[y].IsZero(), "%s Y%d should be zero", line.LineID, y+1)
				}
			}
		}
	}

	for _, year := range out.Cash.Years {
		for m, balance := range year.ClosingBalance {
			assert.True(t, balance.IsPositive(), "Y%d month %d: cash %s", year.YearIndex, m+1, balance.StringFixed(0))
		}
	}
}
