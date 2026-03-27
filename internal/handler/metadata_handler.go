package handler

import (
	"net/http"
	"runtime"

	"github.com/sirupsen/logrus"
	"ascenda/internal/model"
)

// MetadataHandler serves enum lists and application info so the frontend
// never needs to hardcode option lists.
type MetadataHandler struct {
	version   string
	buildTime string
	gitCommit string
	logger    *logrus.Entry

	// Pre-built response — immutable after construction.
	cachedMetadata MetadataResponse
	cachedVersion  VersionResponse
}

// NewMetadataHandler creates a new MetadataHandler and pre-builds the cached
// responses (they never change at runtime).
func NewMetadataHandler(version, buildTime, gitCommit string, logger *logrus.Entry) *MetadataHandler {
	h := &MetadataHandler{
		version:   version,
		buildTime: buildTime,
		gitCommit: gitCommit,
		logger:    logger,
	}
	h.cachedMetadata = h.buildMetadata()
	h.cachedVersion = h.buildVersion()
	return h
}

// ──────────────────────────────────────────────────────────────────────────
// GET /api/v1/version
// ──────────────────────────────────────────────────────────────────────────

// VersionResponse is the JSON shape returned by GET /api/v1/version.
type VersionResponse struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime,omitempty"`
	GitCommit string `json:"gitCommit,omitempty"`
	GoVersion string `json:"goVersion"`
}

func (h *MetadataHandler) buildVersion() VersionResponse {
	return VersionResponse{
		Version:   h.version,
		BuildTime: h.buildTime,
		GitCommit: h.gitCommit,
		GoVersion: runtime.Version(),
	}
}

// GetVersion returns application version and build info.
func (h *MetadataHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.cachedVersion)
}

// ──────────────────────────────────────────────────────────────────────────
// GET /api/v1/metadata
// ──────────────────────────────────────────────────────────────────────────

// MetadataResponse is the JSON shape returned by GET /api/v1/metadata.
type MetadataResponse struct {
	AssetCategories  []AssetCategoryMeta  `json:"assetCategories"`
	StaffCategories  []StaffCategoryMeta  `json:"staffCategories"`
	StaffFunctions   []string             `json:"staffFunctions"`
	OpexLines        []OpexLineMeta       `json:"opexLines"`
	OpexSubcategories []string            `json:"opexSubcategories"`
	PnlLines         []PnlLineMeta        `json:"pnlLines"`
	FiplanLines      []string             `json:"fiplanLines"`
	WcrLines         []string             `json:"wcrLines"`
	PnlCashLines     []string             `json:"pnlCashLines"`
	CashLines        []string             `json:"cashLines"`
	BudgetLines      []BudgetLineMeta     `json:"budgetLines"`
	DistributionRules []string            `json:"distributionRules"`
	SalesChannels    []string             `json:"salesChannels"`
	GeoZones         []string             `json:"geoZones"`
}

// AssetCategoryMeta describes one capex asset category.
type AssetCategoryMeta struct {
	ID                  string `json:"id"`
	IsDepreciable       bool   `json:"isDepreciable"`
	IsStaffLinked       bool   `json:"isStaffLinked"`
	DefaultDeprecYears  int    `json:"defaultDepreciationYears"`
}

// StaffCategoryMeta describes one staff category with its parent function.
type StaffCategoryMeta struct {
	ID       string `json:"id"`
	Function string `json:"function"`
}

// OpexLineMeta describes one opex line with its configuration.
type OpexLineMeta struct {
	ID            string `json:"id"`
	Subcategory   string `json:"subcategory"`
	SortOrder     int    `json:"sortOrder"`
	IsUserInput   bool   `json:"isUserInput"`
	CostDriver    string `json:"costDriver"`
	SettingsField string `json:"settingsField,omitempty"`
}

// PnlLineMeta describes a P&L line, distinguishing user-input lines.
type PnlLineMeta struct {
	ID          string `json:"id"`
	IsUserInput bool   `json:"isUserInput"`
}

// BudgetLineMeta describes a budget line ID.
type BudgetLineMeta struct {
	ID string `json:"id"`
}

func (h *MetadataHandler) buildMetadata() MetadataResponse {
	// ── Asset Categories ──
	assetCats := make([]AssetCategoryMeta, len(model.AllAssetCategories))
	for i, cat := range model.AllAssetCategories {
		assetCats[i] = AssetCategoryMeta{
			ID:                 string(cat),
			IsDepreciable:      !model.NonDepreciableCategories[cat],
			IsStaffLinked:      model.StaffLinkedCategories[cat],
			DefaultDeprecYears: model.DefaultDepreciationYears[cat],
		}
	}

	// ── Staff Categories ──
	staffCats := make([]StaffCategoryMeta, len(model.AllCategories))
	for i, cat := range model.AllCategories {
		staffCats[i] = StaffCategoryMeta{
			ID:       string(cat),
			Function: string(model.CategoryFunction[cat]),
		}
	}

	staffFunctions := []string{
		string(model.FunctionRnD),
		string(model.FunctionProduction),
		string(model.FunctionSales),
		string(model.FunctionGnA),
	}

	// ── Opex Lines ──
	opexLines := make([]OpexLineMeta, len(model.AllOpexLines))
	for i, l := range model.AllOpexLines {
		opexLines[i] = OpexLineMeta{
			ID:            string(l.ID),
			Subcategory:   string(l.Subcategory),
			SortOrder:     l.SortOrder,
			IsUserInput:   l.IsUserInput,
			CostDriver:    l.CostDriver,
			SettingsField: l.SettingsField,
		}
	}

	opexSubcats := []string{
		string(model.OpexSubPremises),
		string(model.OpexSubLeasing),
		string(model.OpexSubProfessional),
		string(model.OpexSubRoyalties),
		string(model.OpexSubTravel),
		string(model.OpexSubMarketing),
		string(model.OpexSubHR),
	}

	// ── P&L Lines ──
	allPnlLineIDs := []model.PnlLineID{
		model.PnlSales, model.PnlExportSalesMemo, model.PnlCapitalizedProd,
		model.PnlStoredProd, model.PnlTotalOperatingRev, model.PnlCOGS,
		model.PnlInventoryChange, model.PnlExternalExpenses, model.PnlTotalConsumption,
		model.PnlAddedValue, model.PnlTaxesAndDuties, model.PnlPayrollExpenses,
		model.PnlEBITDA, model.PnlDepreciation, model.PnlImpairment,
		model.PnlGrantsOtherRevenue, model.PnlOtherOperatingExp, model.PnlEBIT,
		model.PnlFinancialRevenues, model.PnlFinancialExpenses, model.PnlPreTaxEarnings,
		model.PnlExtraordinaryIncome, model.PnlExtraordinaryExpense,
		model.PnlEmployeeParticipation, model.PnlCorporateTax, model.PnlTaxCredits,
		model.PnlNetProfit, model.PnlStaffHeadcount, model.PnlCashFlow,
	}
	userInputSet := make(map[model.PnlLineID]bool)
	for _, id := range model.UserInputPnlLines {
		userInputSet[id] = true
	}
	pnlLines := make([]PnlLineMeta, len(allPnlLineIDs))
	for i, id := range allPnlLineIDs {
		pnlLines[i] = PnlLineMeta{
			ID:          string(id),
			IsUserInput: userInputSet[id],
		}
	}

	// ── FiPlan Lines ──
	fiplanLines := make([]string, len(model.AllFiplanInputLines))
	for i, id := range model.AllFiplanInputLines {
		fiplanLines[i] = string(id)
	}

	// ── WCR Lines ──
	wcrLines := make([]string, len(model.AllWCRInputLines))
	for i, id := range model.AllWCRInputLines {
		wcrLines[i] = string(id)
	}

	// ── PnlCash Lines ──
	pnlCashLines := []string{
		string(model.PnlCashMiscSalesCosts),
	}

	// ── Cash Lines ──
	cashLines := []string{
		string(model.CashOtherRevenues),
		string(model.CashOpexRentTelecom), string(model.CashOpexLeasing),
		string(model.CashOpexFees), string(model.CashOpexRoyalties),
		string(model.CashOpexTravel), string(model.CashOpexAdvertising),
		string(model.CashOpexOther),
		string(model.CashCapexLandBuilding), string(model.CashCapexPatentsRD),
		string(model.CashCapexPrototypes), string(model.CashCapexITVehicles),
		string(model.CashCorporateTax), string(model.CashVATPayments),
		string(model.CashCapitalIncrease), string(model.CashDividends),
		string(model.CashCurrentAccount), string(model.CashLTLoans),
		string(model.CashRepayableGrants), string(model.CashLoanRepayment),
		string(model.CashGrantRepayment), string(model.CashDiscountingExpense),
		string(model.CashOverdraftInterest),
		string(model.CashUnitsDirectSub), string(model.CashUnitsIndirectSub),
		string(model.CashHeadcountSub), string(model.CashIncentivesSub),
	}

	// ── Budget Lines ──
	allBudgetLineIDs := []model.BudgetLineID{
		model.BudgetSalesRevenue, model.BudgetOtherRevenue, model.BudgetTotalRevenue,
		model.BudgetRawMaterials, model.BudgetSubcontracting, model.BudgetDirectLabor,
		model.BudgetTotalCOGS, model.BudgetGrossMargin,
		model.BudgetRentExpenses, model.BudgetLeasingExpenses, model.BudgetProfFees,
		model.BudgetRoyalties, model.BudgetTravelExpenses, model.BudgetMarketingExp,
		model.BudgetHRExpenses, model.BudgetTotalExternal,
		model.BudgetPayroll, model.BudgetIncentives, model.BudgetTotalStaff,
		model.BudgetTaxesDuties, model.BudgetEBITDA, model.BudgetDepreciation,
		model.BudgetEBIT, model.BudgetFinancialIncome, model.BudgetFinancialExp,
		model.BudgetPreTaxProfit, model.BudgetCorporateTax, model.BudgetNetProfit,
	}
	budgetLines := make([]BudgetLineMeta, len(allBudgetLineIDs))
	for i, id := range allBudgetLineIDs {
		budgetLines[i] = BudgetLineMeta{ID: string(id)}
	}

	// ── Distribution Rules ──
	distRules := []string{
		string(model.DistEvenSpread),
		string(model.DistLumpM1),
		string(model.DistFromSchedule),
		string(model.DistManualOnly),
	}

	// ── Sales Channels & Geo Zones ──
	salesChannels := []string{
		string(model.ChannelDirect),
		string(model.ChannelIndirect),
	}
	geoZones := []string{
		string(model.ZoneFrance),
		string(model.ZoneEurope),
		string(model.ZoneExport),
	}

	return MetadataResponse{
		AssetCategories:   assetCats,
		StaffCategories:   staffCats,
		StaffFunctions:    staffFunctions,
		OpexLines:         opexLines,
		OpexSubcategories: opexSubcats,
		PnlLines:          pnlLines,
		FiplanLines:       fiplanLines,
		WcrLines:          wcrLines,
		PnlCashLines:      pnlCashLines,
		CashLines:         cashLines,
		BudgetLines:       budgetLines,
		DistributionRules: distRules,
		SalesChannels:     salesChannels,
		GeoZones:          geoZones,
	}
}

// GetMetadata returns all enum lists and configuration metadata.
func (h *MetadataHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.cachedMetadata)
}
