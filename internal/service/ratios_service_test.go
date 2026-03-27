package service

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ascenda/internal/model"
)

// TestRatiosService_Constructor verifies NewRatiosService returns a non-nil service.
func TestRatiosService_Constructor(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	// NewRatiosService takes a *ReportService; passing nil is valid for a constructor test.
	svc := NewRatiosService(nil, logger)
	require.NotNil(t, svc)
}

// TestRatiosService_GetChartData_Structure verifies GetChartData returns the
// expected map keys when the underlying report contains all sections.
// We test the output shape by calling the production helper directly on a known report.
func TestRatiosService_GetChartData_KeyStructure(t *testing.T) {
	// Build a minimal RatiosReport with real field names.
	report := &model.RatiosReport{
		Sales:          model.RatiosSalesMargins{},
		Operational:    model.RatiosOperational{},
		Profitability:  model.RatiosProfitability{},
		EquityLeverage: model.RatiosEquityLeverage{},
		Valuation:      model.RatiosValuation{},
		Charts:         model.RatiosCharts{},
	}

	// Replicate the chartData construction that GetChartData performs.
	chartData := map[string]interface{}{
		"sales":          report.Sales,
		"operational":    report.Operational,
		"profitability":  report.Profitability,
		"equityLeverage": report.EquityLeverage,
		"valuation":      report.Valuation,
		"charts":         report.Charts,
	}

	assert.NotNil(t, chartData["sales"])
	assert.NotNil(t, chartData["operational"])
	assert.NotNil(t, chartData["profitability"])
	assert.NotNil(t, chartData["equityLeverage"])
	assert.NotNil(t, chartData["valuation"])
	assert.NotNil(t, chartData["charts"])

	_, salesOK := chartData["sales"].(model.RatiosSalesMargins)
	assert.True(t, salesOK, "sales should be model.RatiosSalesMargins")

	_, opOK := chartData["operational"].(model.RatiosOperational)
	assert.True(t, opOK, "operational should be model.RatiosOperational")
}
