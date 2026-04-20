package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
)

// PnLService orchestrates P&L CRUD and computation.
type PnLService struct {
	pnlRepo       repo.PnLRepository
	reportService *ReportService
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// NewPnLService creates a new PnLService.
func NewPnLService(pnlRepo repo.PnLRepository, reportService *ReportService, emitter *event.Emitter, logger *logrus.Entry) *PnLService {
	return &PnLService{
		pnlRepo:       pnlRepo,
		reportService: reportService,
		emitter:       emitter,
		logger:        logger,
	}
}

// ListManualEntries lists all manual P&L entries for a scenario.
func (s *PnLService) ListManualEntries(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.PnlManualEntry, error) {
	ptrEntries, err := s.pnlRepo.ListByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list pnl entries")
		return nil, apierror.Internal("failed to list pnl entries")
	}

	entries := make([]model.PnlManualEntry, len(ptrEntries))
	for i, e := range ptrEntries {
		entries[i] = *e
	}

	return entries, nil
}

// UpdateManualEntries updates or creates manual P&L entries.
func (s *PnLService) UpdateManualEntries(ctx context.Context, tenantID, scenarioID uuid.UUID, entries []model.PnlManualEntry) error {
	for i := range entries {
		entries[i].TenantID = tenantID
		entries[i].ScenarioID = scenarioID
		if entries[i].ID == uuid.Nil {
			entries[i].ID = uuid.New()
		}
	}

	if err := s.pnlRepo.BatchUpsert(tenantID, scenarioID, entries); err != nil {
		s.logger.WithError(err).Error("failed to save pnl entries")
		return apierror.Internal("failed to save pnl entries")
	}

	lines := countUnique(entries, func(e model.PnlManualEntry) string { return string(e.LineID) })
	s.logger.WithField("scenario_id", scenarioID).Info("pnl entries updated")
	s.emitter.Publish(event.Event{
		Type: event.DataChanged, TenantID: tenantID, UserID: ctxutil.GetUserID(ctx), ScenarioID: scenarioID,
		EntityType: "pnl", Action: event.ActionUpdate,
		Changes: marshalChanges(map[string]any{"rows": len(entries), "lines": lines}),
	})
	return nil
}

// GetReport delegates to ReportService for full plan computation and extracts P&L.
func (s *PnLService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.PnlReport, error) {
	fullReport, err := s.reportService.GetFullReport(ctx, tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to compute pnl report")
		return nil, apierror.Internal("failed to compute pnl report")
	}

	s.logger.WithField("scenario_id", scenarioID).Info("pnl report computed")
	return &fullReport.PnL, nil
}

// GetChartData returns chart-formatted P&L data using actual PnlReport fields.
func (s *PnLService) GetChartData(ctx context.Context, tenantID, scenarioID uuid.UUID) (map[string]interface{}, error) {
	report, err := s.GetReport(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	// Extract per-year arrays from the Years struct for chart consumption
	var sales, operatingRevenue, cogs, consumption, addedValue [5]decimal.Decimal
	var payroll, ebitda, depreciation, ebit [5]decimal.Decimal
	var financialRevenues, financialExpenses, preTaxEarnings [5]decimal.Decimal
	var corporateTax, netProfit [5]decimal.Decimal
	for i, y := range report.Years {
		sales[i] = y.Sales
		operatingRevenue[i] = y.TotalOperatingRevenue
		cogs[i] = y.COGS
		consumption[i] = y.TotalConsumption
		addedValue[i] = y.AddedValue
		payroll[i] = y.PayrollExpenses
		ebitda[i] = y.EBITDA
		depreciation[i] = y.Depreciation
		ebit[i] = y.EBIT
		financialRevenues[i] = y.FinancialRevenues
		financialExpenses[i] = y.FinancialExpenses
		preTaxEarnings[i] = y.PreTaxEarnings
		corporateTax[i] = y.CorporateTax
		netProfit[i] = y.NetProfit
	}

	chartData := map[string]interface{}{
		"years":             report.ChartData.Years,
		"sales":             sales,
		"operatingRevenue":  operatingRevenue,
		"cogs":              cogs,
		"consumption":       consumption,
		"addedValue":        addedValue,
		"payroll":           payroll,
		"ebitda":            ebitda,
		"ebitdaPositive":    report.ChartData.EBITDAPositive,
		"ebitdaNegative":    report.ChartData.EBITDANegative,
		"otherOpex":         report.ChartData.OtherOpex,
		"payrollExpenses":   report.ChartData.PayrollExpenses,
		"depreciation":      depreciation,
		"ebit":              ebit,
		"financialRevenues": financialRevenues,
		"financialExpenses": financialExpenses,
		"preTaxEarnings":    preTaxEarnings,
		"corporateTax":      corporateTax,
		"netProfit":         netProfit,
	}

	return chartData, nil
}
