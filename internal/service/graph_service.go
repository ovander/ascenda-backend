package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"ascenda/internal/compute"
	"ascenda/internal/pkg/apierror"
)

// ChartDataset mirrors the frontend ChartDataset interface.
type ChartDataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	Type            string    `json:"type,omitempty"`
	BackgroundColor string    `json:"backgroundColor,omitempty"`
	BorderColor     string    `json:"borderColor,omitempty"`
}

// ChartData mirrors the frontend ChartData interface.
type ChartData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

// GraphService builds ChartData from the computed plan output.
type GraphService struct {
	reportService *ReportService
	logger        *logrus.Entry
}

// NewGraphService creates a new GraphService.
func NewGraphService(reportService *ReportService, logger *logrus.Entry) *GraphService {
	return &GraphService{reportService: reportService, logger: logger}
}

// GetAllAnnualCharts returns all annual charts in a single call to avoid rate-limit
// exhaustion when multiple charts are needed simultaneously.
func (s *GraphService) GetAllAnnualCharts(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]*ChartData, error) {
	full, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("graph: failed to get full report")
		return nil, apierror.Internal("failed to compute graph data")
	}

	g := compute.ComputeGraphs(full)

	labels := make([]string, 5)
	for i, py := range full.PnL.Years {
		if py.Year > 0 {
			labels[i] = fmt.Sprintf("%d", py.Year)
		} else {
			labels[i] = fmt.Sprintf("Year %d", i+1)
		}
	}

	result := map[string]*ChartData{
		"sales-analysis": {
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Domestic Sales", Data: decimalsToFloats(g.SalesAnalysis.DomesticSales[:])},
				{Label: "Export Sales",   Data: decimalsToFloats(g.SalesAnalysis.ExportSales[:])},
			},
		},
		"cost-structure": {
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "COGS",               Data: decimalsToFloats(g.CostStructure.COGS[:])},
				{Label: "External Expenses",  Data: decimalsToFloats(g.CostStructure.ExternalExp[:])},
				{Label: "Payroll",            Data: decimalsToFloats(g.CostStructure.PayrollExp[:])},
				{Label: "Depreciation",       Data: decimalsToFloats(g.CostStructure.Depreciation[:])},
				{Label: "Other Opex",         Data: decimalsToFloats(g.CostStructure.OtherOpex[:])},
			},
		},
		"revenue-profit-cash": {
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Revenue",         Data: decimalsToFloats(g.RevProfitCash.Revenue[:])},
				{Label: "Net Profit",      Data: decimalsToFloats(g.RevProfitCash.NetProfit[:])},
				{Label: "Cash Flow",       Data: decimalsToFloats(g.RevProfitCash.CashFlow[:]),       Type: "line"},
				{Label: "Cumulative Cash", Data: decimalsToFloats(g.RevProfitCash.CumulativeCash[:]), Type: "line"},
			},
		},
		"requirements-vs-cash": {
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Total Requirements", Data: decimalsToFloats(g.FinRequirements.Requirements[:])},
				{Label: "Total Resources",    Data: decimalsToFloats(g.FinRequirements.Resources[:])},
				{Label: "Cumulative Cash",    Data: decimalsToFloats(g.FinRequirements.CumulativeCash[:]), Type: "line"},
			},
		},
		"balance-sheet-structure": {
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Equity",          Data: decimalsToFloats(g.BalanceSheet.Equity[:])},
				{Label: "Long-Term Debt",  Data: decimalsToFloats(g.BalanceSheet.LongTermDebt[:])},
				{Label: "Short-Term Debt", Data: decimalsToFloats(g.BalanceSheet.ShortTermDebt[:])},
			},
		},
		"headcount-annual": {
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "R&D",        Data: decimalsToFloats(g.HeadcountAnnual.RnD[:])},
				{Label: "Production", Data: decimalsToFloats(g.HeadcountAnnual.Production[:])},
				{Label: "Sales",      Data: decimalsToFloats(g.HeadcountAnnual.Sales[:])},
				{Label: "G&A",        Data: decimalsToFloats(g.HeadcountAnnual.GnA[:])},
			},
		},
		"pnl-cascade": {
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Revenue",      Data: decimalsToFloats(g.PnLCascade.Revenue[:])},
				{Label: "Gross Margin", Data: decimalsToFloats(g.PnLCascade.GrossMargin[:])},
				{Label: "EBITDA",       Data: decimalsToFloats(g.PnLCascade.EBITDA[:])},
				{Label: "EBIT",         Data: decimalsToFloats(g.PnLCascade.EBIT[:]),       Type: "line"},
				{Label: "Net Profit",   Data: decimalsToFloats(g.PnLCascade.NetProfit[:]),  Type: "line"},
			},
		},
	}

	return result, nil
}

// GetAnnualChart returns a ChartData payload for one of the named annual charts:
//
//	"sales-analysis", "cost-structure", "revenue-profit-cash", "requirements-vs-cash",
//	"balance-sheet-structure", "headcount-annual", "pnl-cascade"
func (s *GraphService) GetAnnualChart(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*ChartData, error) {
	full, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("graph: failed to get full report")
		return nil, apierror.Internal("failed to compute graph data")
	}

	g := compute.ComputeGraphs(full)

	// Build year labels from PnL.Years
	labels := make([]string, 5)
	for i, py := range full.PnL.Years {
		if py.Year > 0 {
			labels[i] = fmt.Sprintf("%d", py.Year)
		} else {
			labels[i] = fmt.Sprintf("Year %d", i+1)
		}
	}

	switch name {
	case "sales-analysis":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Domestic Sales", Data: decimalsToFloats(g.SalesAnalysis.DomesticSales[:])},
				{Label: "Export Sales", Data: decimalsToFloats(g.SalesAnalysis.ExportSales[:])},
			},
		}, nil

	case "cost-structure":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "COGS", Data: decimalsToFloats(g.CostStructure.COGS[:])},
				{Label: "External Expenses", Data: decimalsToFloats(g.CostStructure.ExternalExp[:])},
				{Label: "Payroll", Data: decimalsToFloats(g.CostStructure.PayrollExp[:])},
				{Label: "Depreciation", Data: decimalsToFloats(g.CostStructure.Depreciation[:])},
				{Label: "Other Opex", Data: decimalsToFloats(g.CostStructure.OtherOpex[:])},
			},
		}, nil

	case "revenue-profit-cash":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Revenue", Data: decimalsToFloats(g.RevProfitCash.Revenue[:])},
				{Label: "Net Profit", Data: decimalsToFloats(g.RevProfitCash.NetProfit[:])},
				{Label: "Cash Flow", Data: decimalsToFloats(g.RevProfitCash.CashFlow[:]), Type: "line"},
				{Label: "Cumulative Cash", Data: decimalsToFloats(g.RevProfitCash.CumulativeCash[:]), Type: "line"},
			},
		}, nil

	case "requirements-vs-cash":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Total Requirements", Data: decimalsToFloats(g.FinRequirements.Requirements[:])},
				{Label: "Total Resources", Data: decimalsToFloats(g.FinRequirements.Resources[:])},
				{Label: "Cumulative Cash", Data: decimalsToFloats(g.FinRequirements.CumulativeCash[:]), Type: "line"},
			},
		}, nil

	case "balance-sheet-structure":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Equity",          Data: decimalsToFloats(g.BalanceSheet.Equity[:])},
				{Label: "Long-Term Debt",  Data: decimalsToFloats(g.BalanceSheet.LongTermDebt[:])},
				{Label: "Short-Term Debt", Data: decimalsToFloats(g.BalanceSheet.ShortTermDebt[:])},
			},
		}, nil

	case "headcount-annual":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "R&D",        Data: decimalsToFloats(g.HeadcountAnnual.RnD[:])},
				{Label: "Production", Data: decimalsToFloats(g.HeadcountAnnual.Production[:])},
				{Label: "Sales",      Data: decimalsToFloats(g.HeadcountAnnual.Sales[:])},
				{Label: "G&A",        Data: decimalsToFloats(g.HeadcountAnnual.GnA[:])},
			},
		}, nil

	case "pnl-cascade":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Revenue",      Data: decimalsToFloats(g.PnLCascade.Revenue[:])},
				{Label: "Gross Margin", Data: decimalsToFloats(g.PnLCascade.GrossMargin[:])},
				{Label: "EBITDA",       Data: decimalsToFloats(g.PnLCascade.EBITDA[:])},
				{Label: "EBIT",         Data: decimalsToFloats(g.PnLCascade.EBIT[:]), Type: "line"},
				{Label: "Net Profit",   Data: decimalsToFloats(g.PnLCascade.NetProfit[:]), Type: "line"},
			},
		}, nil

	default:
		return nil, apierror.NotFound("annual chart", name)
	}
}

// GetMonthlyChart returns a ChartData payload for one of the named monthly charts:
//
//	"cash-equity-debt", "operating-cash-flows", "invoicing-ebitda", "headcount"
func (s *GraphService) GetMonthlyChart(ctx context.Context, tenantID, scenarioID uuid.UUID, name string) (*ChartData, error) {
	full, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("graph: failed to get full report")
		return nil, apierror.Internal("failed to compute graph data")
	}

	g := compute.ComputeGraphs2(full)
	labels := g.Months[:]

	switch name {
	case "cash-equity-debt":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Cash Balance", Data: decimalsToFloats(g.CashEquityDebt.CashBalance[:])},
				{Label: "Equity", Data: decimalsToFloats(g.CashEquityDebt.Equity[:]), Type: "line"},
				{Label: "Total Debt", Data: decimalsToFloats(g.CashEquityDebt.TotalDebt[:]), Type: "line"},
			},
		}, nil

	case "operating-cash-flows":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Inflows", Data: decimalsToFloats(g.OperatingCash.Inflows[:])},
				{Label: "Outflows", Data: decimalsToFloats(g.OperatingCash.Outflows[:])},
				{Label: "Net Cash", Data: decimalsToFloats(g.OperatingCash.NetCash[:]), Type: "line"},
			},
		}, nil

	case "invoicing-ebitda":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "Monthly Revenue", Data: decimalsToFloats(g.InvoicingEBITDA.MonthlyRevenue[:])},
				{Label: "Monthly EBITDA", Data: decimalsToFloats(g.InvoicingEBITDA.MonthlyEBITDA[:])},
				{Label: "Cumulative Revenue", Data: decimalsToFloats(g.InvoicingEBITDA.CumulRevenue[:]), Type: "line"},
			},
		}, nil

	case "headcount":
		return &ChartData{
			Labels: labels,
			Datasets: []ChartDataset{
				{Label: "R&D", Data: decimalsToFloats(g.HeadcountByFunc.RnD[:])},
				{Label: "Production", Data: decimalsToFloats(g.HeadcountByFunc.Production[:])},
				{Label: "Sales", Data: decimalsToFloats(g.HeadcountByFunc.Sales[:])},
				{Label: "G&A", Data: decimalsToFloats(g.HeadcountByFunc.GnA[:])},
				{Label: "Total", Data: decimalsToFloats(g.HeadcountByFunc.Total[:]), Type: "line"},
			},
		}, nil

	default:
		return nil, apierror.NotFound("monthly chart", name)
	}
}

// decimalsToFloats converts []decimal.Decimal to []float64.
func decimalsToFloats(vals []decimal.Decimal) []float64 {
	out := make([]float64, len(vals))
	for i, v := range vals {
		f, _ := v.Float64()
		out[i] = f
	}
	return out
}
