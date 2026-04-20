package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"ascenda/internal/compute"
	"ascenda/internal/event"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"ascenda/internal/repo"
)

// fiplanSyncer abstracts the FiPlan operations needed by CapTableService,
// keeping the two services loosely coupled and independently testable.
type fiplanSyncer interface {
	UpsertCapitalIncreaseEntry(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int, amount decimal.Decimal, roundID uuid.UUID, roundLabel string) error
	ClearCapTableLink(ctx context.Context, tenantID, scenarioID uuid.UUID, yearIndex int) error
}

// CapTableService orchestrates all three cap table sub-modules.
type CapTableService struct {
	capTableRepo repo.CapTableRepository
	settingsRepo repo.SettingsRepository
	fiplanSvc    fiplanSyncer // optional — nil for freemium tenants
	emitter      *event.Emitter
	logger       *logrus.Entry
}

// NewCapTableService creates a new CapTableService.
// fiplanSvc may be nil; SyncRoundToFiplan will return an error if called without it.
func NewCapTableService(
	capTableRepo repo.CapTableRepository,
	settingsRepo repo.SettingsRepository,
	fiplanSvc fiplanSyncer,
	emitter *event.Emitter,
	logger *logrus.Entry,
) *CapTableService {
	return &CapTableService{
		capTableRepo: capTableRepo,
		settingsRepo: settingsRepo,
		fiplanSvc:    fiplanSvc,
		emitter:      emitter,
		logger:       logger,
	}
}

// ─── CapTableCompany ──────────────────────────────────────────────────────────

// GetCompany returns the company config for a scenario, or a country-populated
// default if none exists yet.
func (s *CapTableService) GetCompany(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableCompany, error) {
	company, err := s.capTableRepo.GetCompany(tenantID, scenarioID)
	if err != nil {
		// Lazily build a default from the scenario's country setting.
		def, defErr := s.buildDefaultCompany(tenantID, scenarioID)
		if defErr != nil {
			s.logger.WithError(err).Warn("cap table company not found, returning bare default")
			def = &model.CapTableCompany{ScenarioID: scenarioID}
			def.TenantID = tenantID
		}
		return def, nil
	}
	return company, nil
}

// UpsertCompany creates or updates the company config for a scenario.
func (s *CapTableService) UpsertCompany(ctx context.Context, company *model.CapTableCompany) error {
	company.TenantID = company.TenantID // already set by handler
	if err := s.capTableRepo.UpsertCompany(company); err != nil {
		s.logger.WithError(err).Error("failed to upsert cap table company")
		return apierror.Internal("failed to save cap table company")
	}
	s.publishWithChanges(ctx, company.TenantID, company.ScenarioID, "cap_table_company", event.ActionUpdate, map[string]any{
		"legalForm":  company.LegalForm,
		"currency":   company.Currency,
		"maxPhases":  company.MaxPhases,
	})
	return nil
}

// buildDefaultCompany constructs a country-aware default CapTableCompany.
func (s *CapTableService) buildDefaultCompany(tenantID, scenarioID uuid.UUID) (*model.CapTableCompany, error) {
	cfg, err := s.settingsRepo.GetConfig(tenantID, scenarioID)
	if err != nil || cfg == nil {
		return nil, err
	}
	profile := model.GetCapTableCountryProfile(cfg.Country)
	c := &model.CapTableCompany{
		ScenarioID:       scenarioID,
		Currency:         profile.Currency,
		CurrencySymbol:   profile.CurrencySymbol,
		NominalValueCents: int64(profile.DefaultNominalValueCents),
		DisplayLanguage:  profile.PrimaryLanguage,
		DateFormat:       profile.DateFormat,
		BookEquityTerm:   profile.BookEquityTerm,
		ShareCapitalTerm: profile.ShareCapitalTerm,
		MaxPhases:        7,
	}
	c.TenantID = tenantID
	if len(profile.LegalForms) > 0 {
		c.LegalForm = profile.DefaultLegalForm
	}
	return c, nil
}

// ─── Country profile ──────────────────────────────────────────────────────────

// GetCountryProfile returns the cap table country profile for the given ISO code.
func (s *CapTableService) GetCountryProfile(code string) model.CapTableCountryProfile {
	return model.GetCapTableCountryProfile(code)
}

// ─── CapTableShareClass ───────────────────────────────────────────────────────

// ListShareClasses returns all share classes for a scenario.
func (s *CapTableService) ListShareClasses(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareClass, error) {
	rows, err := s.capTableRepo.ListShareClasses(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list share classes")
		return nil, apierror.Internal("failed to list share classes")
	}
	out := make([]model.CapTableShareClass, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	return out, nil
}

// UpsertShareClass creates or updates a share class.
func (s *CapTableService) UpsertShareClass(ctx context.Context, sc *model.CapTableShareClass) error {
	if err := s.capTableRepo.UpsertShareClass(sc); err != nil {
		s.logger.WithError(err).Error("failed to upsert share class")
		return apierror.Internal("failed to save share class")
	}
	s.publishWithChanges(ctx, sc.TenantID, sc.ScenarioID, "cap_table_share_class", event.ActionUpdate, map[string]any{
		"classType": sc.ClassType,
		"label":     sc.Label,
	})
	return nil
}

// DeleteShareClass removes a share class by ID.
func (s *CapTableService) DeleteShareClass(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.capTableRepo.DeleteShareClass(tenantID, id); err != nil {
		s.logger.WithError(err).Error("failed to delete share class")
		return apierror.Internal("failed to delete share class")
	}
	return nil
}

// ─── CapTableShareholder ──────────────────────────────────────────────────────

// ListShareholders returns all shareholders for a scenario.
func (s *CapTableService) ListShareholders(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableShareholder, error) {
	rows, err := s.capTableRepo.ListShareholders(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list shareholders")
		return nil, apierror.Internal("failed to list shareholders")
	}
	out := make([]model.CapTableShareholder, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	return out, nil
}

// CreateShareholder inserts a new shareholder.
func (s *CapTableService) CreateShareholder(ctx context.Context, sh *model.CapTableShareholder) error {
	if sh.ID == uuid.Nil {
		sh.ID = uuid.New()
	}
	if err := s.capTableRepo.CreateShareholder(sh); err != nil {
		s.logger.WithError(err).Error("failed to create shareholder")
		return apierror.Internal("failed to create shareholder")
	}
	s.publishWithChanges(ctx, sh.TenantID, sh.ScenarioID, "cap_table_shareholder", event.ActionCreate, map[string]any{
		"name": sh.Name,
		"type": sh.Type,
	})
	return nil
}

// UpdateShareholder saves changes to a shareholder.
func (s *CapTableService) UpdateShareholder(ctx context.Context, sh *model.CapTableShareholder) error {
	if err := s.capTableRepo.UpdateShareholder(sh); err != nil {
		s.logger.WithError(err).Error("failed to update shareholder")
		return apierror.Internal("failed to update shareholder")
	}
	s.publishWithChanges(ctx, sh.TenantID, sh.ScenarioID, "cap_table_shareholder", event.ActionUpdate, map[string]any{
		"name": sh.Name,
		"type": sh.Type,
	})
	return nil
}

// DeleteShareholder removes a shareholder.
// Returns apierror.NotFound when no row matched the tenant+id filter so the
// handler can emit HTTP 404 instead of 500 for wrong-tenant deletes.
func (s *CapTableService) DeleteShareholder(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.capTableRepo.DeleteShareholder(tenantID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apierror.NotFound("shareholder", id.String())
		}
		s.logger.WithError(err).Error("failed to delete shareholder")
		return apierror.Internal("failed to delete shareholder")
	}
	return nil
}

// ─── CapTableRound ────────────────────────────────────────────────────────────

// ListRounds returns all funding rounds for a scenario.
// IsSyncAmountDivergent is computed here: true when FiplanSynced=true and
// AmountRaisedK has changed since the last sync (negotiation divergence).
func (s *CapTableService) ListRounds(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableRound, error) {
	rows, err := s.capTableRepo.ListRounds(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list rounds")
		return nil, apierror.Internal("failed to list rounds")
	}
	out := make([]model.CapTableRound, len(rows))
	for i, r := range rows {
		out[i] = *r
		if r.FiplanSynced && r.FiplanSyncedAmountK != nil {
			out[i].IsSyncAmountDivergent = !r.FiplanSyncedAmountK.Equal(r.AmountRaisedK)
		}
		if r.OpeningBalanceSynced && r.OpeningBalanceSyncedAmountK != nil {
			out[i].IsOpeningBalanceDivergent = !r.OpeningBalanceSyncedAmountK.Equal(r.AmountRaisedK)
		}
	}
	return out, nil
}

// CreateRound inserts a new funding round.
func (s *CapTableService) CreateRound(ctx context.Context, rnd *model.CapTableRound) error {
	if rnd.ID == uuid.Nil {
		rnd.ID = uuid.New()
	}
	if err := s.capTableRepo.CreateRound(rnd); err != nil {
		s.logger.WithError(err).Error("failed to create round")
		return apierror.Internal("failed to create round")
	}
	s.publishWithChanges(ctx, rnd.TenantID, rnd.ScenarioID, "cap_table_round", event.ActionCreate, map[string]any{
		"label":       rnd.Label,
		"eventType":   rnd.EventType,
		"phaseNumber": rnd.PhaseNumber,
	})
	return nil
}

// UpdateRound saves changes to a funding round.
// If the round was previously synced to FiPlan (FiplanSynced=true) and the
// AmountRaisedK has changed, the sync is automatically re-applied so the
// FiPlan capital_increase entry stays in step with the Cap Table.
func (s *CapTableService) UpdateRound(ctx context.Context, rnd *model.CapTableRound) error {
	if err := s.capTableRepo.UpdateRound(rnd); err != nil {
		s.logger.WithError(err).Error("failed to update round")
		return apierror.Internal("failed to update round")
	}
	s.publishWithChanges(ctx, rnd.TenantID, rnd.ScenarioID, "cap_table_round", event.ActionUpdate, map[string]any{
		"label":       rnd.Label,
		"eventType":   rnd.EventType,
		"phaseNumber": rnd.PhaseNumber,
	})

	// Auto-resync: if the round is already linked to FiPlan and the amount may
	// have changed, push the updated amount without requiring a manual sync click.
	if rnd.FiplanSynced && rnd.FiscalYearIndex != nil && s.fiplanSvc != nil {
		if _, err := s.SyncRoundToFiplan(ctx, rnd.TenantID, rnd.ID, *rnd.FiscalYearIndex); err != nil {
			// Non-fatal: log the failure but don't block the round update.
			s.logger.WithError(err).Warn("auto-sync of cap table round to FiPlan failed")
		}
	}
	return nil
}

// DeleteRound removes a funding round.
func (s *CapTableService) DeleteRound(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.capTableRepo.DeleteRound(tenantID, id); err != nil {
		s.logger.WithError(err).Error("failed to delete round")
		return apierror.Internal("failed to delete round")
	}
	return nil
}

// ─── StockOptionPlan ─────────────────────────────────────────────────────────

// ListPlans returns all stock option plans for a scenario.
func (s *CapTableService) ListPlans(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.StockOptionPlan, error) {
	rows, err := s.capTableRepo.ListPlans(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list plans")
		return nil, apierror.Internal("failed to list option plans")
	}
	out := make([]model.StockOptionPlan, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	return out, nil
}

// CreatePlan inserts a new stock option plan.
func (s *CapTableService) CreatePlan(ctx context.Context, plan *model.StockOptionPlan) error {
	if plan.ID == uuid.Nil {
		plan.ID = uuid.New()
	}
	if err := s.capTableRepo.CreatePlan(plan); err != nil {
		s.logger.WithError(err).Error("failed to create option plan")
		return apierror.Internal("failed to create option plan")
	}
	s.publishWithChanges(ctx, plan.TenantID, plan.ScenarioID, "stock_option_plan", event.ActionCreate, map[string]any{
		"planLabel":  plan.PlanLabel,
		"instrument": plan.Instrument,
	})
	return nil
}

// UpdatePlan saves changes to a stock option plan.
func (s *CapTableService) UpdatePlan(ctx context.Context, plan *model.StockOptionPlan) error {
	if err := s.capTableRepo.UpdatePlan(plan); err != nil {
		s.logger.WithError(err).Error("failed to update option plan")
		return apierror.Internal("failed to update option plan")
	}
	s.publishWithChanges(ctx, plan.TenantID, plan.ScenarioID, "stock_option_plan", event.ActionUpdate, map[string]any{
		"planLabel":  plan.PlanLabel,
		"instrument": plan.Instrument,
	})
	return nil
}

// DeletePlan removes a stock option plan.
func (s *CapTableService) DeletePlan(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.capTableRepo.DeletePlan(tenantID, id); err != nil {
		s.logger.WithError(err).Error("failed to delete option plan")
		return apierror.Internal("failed to delete option plan")
	}
	return nil
}

// ─── OptionGrant ─────────────────────────────────────────────────────────────

// ListGrants returns all option grants for a scenario.
func (s *CapTableService) ListGrants(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.OptionGrant, error) {
	rows, err := s.capTableRepo.ListGrantsByScenario(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list option grants")
		return nil, apierror.Internal("failed to list option grants")
	}
	out := make([]model.OptionGrant, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	return out, nil
}

// CreateGrant inserts a new option grant.
func (s *CapTableService) CreateGrant(ctx context.Context, grant *model.OptionGrant) error {
	if grant.ID == uuid.Nil {
		grant.ID = uuid.New()
	}
	if err := s.capTableRepo.CreateGrant(grant); err != nil {
		s.logger.WithError(err).Error("failed to create option grant")
		return apierror.Internal("failed to create option grant")
	}
	return nil
}

// UpdateGrant saves changes to an option grant.
func (s *CapTableService) UpdateGrant(ctx context.Context, grant *model.OptionGrant) error {
	if err := s.capTableRepo.UpdateGrant(grant); err != nil {
		s.logger.WithError(err).Error("failed to update option grant")
		return apierror.Internal("failed to update option grant")
	}
	return nil
}

// DeleteGrant removes an option grant.
func (s *CapTableService) DeleteGrant(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.capTableRepo.DeleteGrant(tenantID, id); err != nil {
		s.logger.WithError(err).Error("failed to delete option grant")
		return apierror.Internal("failed to delete option grant")
	}
	return nil
}

// ─── ValuationScenario ────────────────────────────────────────────────────────

// ListValuationScenarios returns all valuation scenarios for a Ascenda scenario.
func (s *CapTableService) ListValuationScenarios(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.ValuationScenario, error) {
	rows, err := s.capTableRepo.ListValuationScenarios(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list valuation scenarios")
		return nil, apierror.Internal("failed to list valuation scenarios")
	}
	out := make([]model.ValuationScenario, len(rows))
	for i, r := range rows {
		// Re-compute outputs on read.
		result := compute.ComputeValuation(*r)
		r.OutIRRPct = result.IRRPct
		r.OutTerminalValueK = result.TerminalValueK
		r.OutMultiple = result.MoneyMultiple
		r.OutInvestorShareK = result.InvestorShareK
		r.OutNPVK = result.NPVK
		r.OutPreMoneyK = result.PreMoneyK
		r.OutPostMoneyK = result.PostMoneyK
		out[i] = *r
	}
	return out, nil
}

// CreateValuationScenario inserts a new valuation scenario and computes outputs.
func (s *CapTableService) CreateValuationScenario(ctx context.Context, vs *model.ValuationScenario) error {
	if vs.ID == uuid.Nil {
		vs.ID = uuid.New()
	}
	s.computeAndStoreValo(vs)
	if err := s.capTableRepo.CreateValuationScenario(vs); err != nil {
		s.logger.WithError(err).Error("failed to create valuation scenario")
		return apierror.Internal("failed to create valuation scenario")
	}
	return nil
}

// UpdateValuationScenario saves changes to a valuation scenario and recomputes.
func (s *CapTableService) UpdateValuationScenario(ctx context.Context, vs *model.ValuationScenario) error {
	s.computeAndStoreValo(vs)
	if err := s.capTableRepo.UpdateValuationScenario(vs); err != nil {
		s.logger.WithError(err).Error("failed to update valuation scenario")
		return apierror.Internal("failed to update valuation scenario")
	}
	return nil
}

// DeleteValuationScenario removes a valuation scenario.
func (s *CapTableService) DeleteValuationScenario(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.capTableRepo.DeleteValuationScenario(tenantID, id); err != nil {
		s.logger.WithError(err).Error("failed to delete valuation scenario")
		return apierror.Internal("failed to delete valuation scenario")
	}
	return nil
}

// ComputeValuationScenario recomputes outputs for one scenario (on-demand).
func (s *CapTableService) ComputeValuationScenario(ctx context.Context, tenantID, id uuid.UUID) (*model.ValuationScenarioResult, error) {
	vs, err := s.capTableRepo.GetValuationScenario(tenantID, id)
	if err != nil {
		return nil, apierror.NotFound("valuation scenario", id.String())
	}
	result := compute.ComputeValuation(*vs)
	return &result, nil
}

// computeAndStoreValo runs the solver and writes outputs back onto the entity.
func (s *CapTableService) computeAndStoreValo(vs *model.ValuationScenario) {
	result := compute.ComputeValuation(*vs)
	vs.OutIRRPct = result.IRRPct
	vs.OutTerminalValueK = result.TerminalValueK
	vs.OutMultiple = result.MoneyMultiple
	vs.OutInvestorShareK = result.InvestorShareK
	vs.OutNPVK = result.NPVK
	vs.OutPreMoneyK = result.PreMoneyK
	vs.OutPostMoneyK = result.PostMoneyK
}

// ─── CapTableScenarioBranch ───────────────────────────────────────────────────

// ListBranches returns all scenario branches for a Ascenda scenario.
func (s *CapTableService) ListBranches(ctx context.Context, tenantID, scenarioID uuid.UUID) ([]model.CapTableScenarioBranch, error) {
	rows, err := s.capTableRepo.ListBranches(tenantID, scenarioID)
	if err != nil {
		s.logger.WithError(err).Error("failed to list scenario branches")
		return nil, apierror.Internal("failed to list scenario branches")
	}
	out := make([]model.CapTableScenarioBranch, len(rows))
	for i, r := range rows {
		out[i] = *r
	}
	return out, nil
}

// CreateBranch inserts a new scenario branch.
func (s *CapTableService) CreateBranch(ctx context.Context, branch *model.CapTableScenarioBranch) error {
	if branch.ID == uuid.Nil {
		branch.ID = uuid.New()
	}
	if err := s.capTableRepo.CreateBranch(branch); err != nil {
		s.logger.WithError(err).Error("failed to create scenario branch")
		return apierror.Internal("failed to create scenario branch")
	}
	return nil
}

// UpdateBranch saves changes to a scenario branch.
func (s *CapTableService) UpdateBranch(ctx context.Context, branch *model.CapTableScenarioBranch) error {
	if err := s.capTableRepo.UpdateBranch(branch); err != nil {
		s.logger.WithError(err).Error("failed to update scenario branch")
		return apierror.Internal("failed to update scenario branch")
	}
	return nil
}

// ─── FiPlan sync ─────────────────────────────────────────────────────────────

// SyncRoundToFiplan pushes the round's AmountRaisedK to the capital_increase
// FiPlan entry for fiscalYearIndex (0-based). Creates or overwrites the entry.
// Requires a Pro+ subscription (enforced at the handler/router level).
func (s *CapTableService) SyncRoundToFiplan(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID, fiscalYearIndex int) (*model.CapTableRound, error) {
	if s.fiplanSvc == nil {
		return nil, apierror.Internal("FiPlan sync is not available")
	}
	if fiscalYearIndex < 0 || fiscalYearIndex > 4 {
		return nil, apierror.BadRequest("fiscalYearIndex must be between 0 and 4")
	}

	round, err := s.capTableRepo.GetRound(tenantID, roundID)
	if err != nil {
		return nil, apierror.NotFound("round", roundID.String())
	}

	// AmountRaisedK is stored in k€ — convert to full € for FiPlan.
	amountFull := round.AmountRaisedK.Mul(decimal.NewFromInt(1000))

	if err := s.fiplanSvc.UpsertCapitalIncreaseEntry(
		ctx, tenantID, round.ScenarioID, fiscalYearIndex,
		amountFull, round.ID, round.Label,
	); err != nil {
		return nil, err
	}

	// Mark the round as synced and stamp the amount at sync time.
	// FiplanSyncedAmountK is the divergence baseline: if AmountRaisedK changes
	// after this point, IsSyncAmountDivergent will flip to true on next read.
	round.FiscalYearIndex = &fiscalYearIndex
	round.FiplanSynced = true
	// Copy the amount into a new variable so FiplanSyncedAmountK is an independent
	// snapshot. Storing &round.AmountRaisedK would make the pointer chase later
	// mutations of the same struct field.
	syncedAmount := round.AmountRaisedK
	round.FiplanSyncedAmountK = &syncedAmount
	if err := s.capTableRepo.UpdateRound(round); err != nil {
		s.logger.WithError(err).Error("failed to update round sync status")
		return nil, apierror.Internal("failed to update round sync status")
	}

	s.publishWithChanges(ctx, tenantID, round.ScenarioID, "cap_table_round", event.ActionUpdate, map[string]any{
		"roundId":              round.ID,
		"fiscalYearIndex":      fiscalYearIndex,
		"fiplanSynced":         true,
		"fiplanSyncedAmountK":  round.AmountRaisedK,
	})
	return round, nil
}

// ─── Opening balance sync ─────────────────────────────────────────────────────

// SyncRoundToOpeningBalance writes a founding-capital round into the scenario's
// opening balance (share_capital + cash_and_securities).  It sums all rounds
// already marked OpeningBalanceSynced plus this one so multiple co-founders can
// each be applied independently and the totals stay consistent.
//
// This is mutually exclusive with FiplanSynced: a round that seeds the opening
// balance represents capital deposited before company registration (French SAS /
// SARL: capital social bloqué avant immatriculation) and must NOT appear as a
// FiPlan forecast resource — that would double-count it.
func (s *CapTableService) SyncRoundToOpeningBalance(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error) {
	round, err := s.capTableRepo.GetRound(tenantID, roundID)
	if err != nil {
		return nil, apierror.NotFound("round", roundID.String())
	}
	if round.FiplanSynced {
		return nil, apierror.BadRequest("round is already synced to FiPlan — unlink it first before applying to opening balance")
	}

	// Gather all OTHER rounds already synced to opening balance for this scenario.
	allRounds, err := s.capTableRepo.ListRounds(tenantID, round.ScenarioID)
	if err != nil {
		return nil, apierror.Internal("failed to list rounds")
	}
	totalK := round.AmountRaisedK
	for _, r := range allRounds {
		if r.ID != roundID && r.OpeningBalanceSynced {
			totalK = totalK.Add(r.AmountRaisedK)
		}
	}
	// Convert k-units to full currency (opening balance stores absolute values).
	totalFull := totalK.Mul(decimal.NewFromInt(1000))

	// Compute what the previous total founding-capital contribution was, so we
	// can delta-adjust CashAndSecurities rather than overwriting it (the user
	// may have entered other cash that is unrelated to share capital).
	previousTotalK := decimal.Zero
	for _, r := range allRounds {
		if r.OpeningBalanceSynced && r.OpeningBalanceSyncedAmountK != nil {
			previousTotalK = previousTotalK.Add(*r.OpeningBalanceSyncedAmountK)
		}
	}
	previousTotal := previousTotalK.Mul(decimal.NewFromInt(1000))

	// Load current opening balance (creates a zero-value struct if not yet saved).
	ob, err := s.settingsRepo.GetOpeningBalance(tenantID, round.ScenarioID)
	if err != nil || ob == nil {
		ob = &model.OpeningBalance{
			TenantScoped: model.TenantScoped{TenantID: tenantID},
			ScenarioID:   round.ScenarioID,
		}
	}

	// Replace the share-capital slice with the new total; adjust cash by the delta.
	delta := totalFull.Sub(previousTotal)
	ob.ShareCapital = totalFull
	ob.CashAndSecurities = ob.CashAndSecurities.Add(delta)

	if err := s.settingsRepo.UpsertOpeningBalance(ob); err != nil {
		s.logger.WithError(err).Error("failed to upsert opening balance")
		return nil, apierror.Internal("failed to update opening balance")
	}

	syncedAmt := round.AmountRaisedK
	round.OpeningBalanceSynced = true
	round.OpeningBalanceSyncedAmountK = &syncedAmt
	round.IsOpeningBalanceDivergent = false
	if err := s.capTableRepo.UpdateRound(round); err != nil {
		s.logger.WithError(err).Error("failed to update round opening balance sync status")
		return nil, apierror.Internal("failed to update round sync status")
	}

	s.publishWithChanges(ctx, tenantID, round.ScenarioID, "cap_table_round", event.ActionUpdate, map[string]any{
		"roundId":                    round.ID,
		"openingBalanceSynced":       true,
		"openingBalanceSyncedAmountK": round.AmountRaisedK,
	})
	return round, nil
}

// UnsyncRoundFromOpeningBalance clears the opening-balance link for a round and
// subtracts its contribution from opening_balance.share_capital and cash.
func (s *CapTableService) UnsyncRoundFromOpeningBalance(ctx context.Context, tenantID, roundID uuid.UUID) (*model.CapTableRound, error) {
	round, err := s.capTableRepo.GetRound(tenantID, roundID)
	if err != nil {
		return nil, apierror.NotFound("round", roundID.String())
	}
	if !round.OpeningBalanceSynced || round.OpeningBalanceSyncedAmountK == nil {
		return nil, apierror.BadRequest("round is not currently linked to opening balance")
	}

	ob, err := s.settingsRepo.GetOpeningBalance(tenantID, round.ScenarioID)
	if err != nil || ob == nil {
		ob = &model.OpeningBalance{
			TenantScoped: model.TenantScoped{TenantID: tenantID},
			ScenarioID:   round.ScenarioID,
		}
	}

	removed := round.OpeningBalanceSyncedAmountK.Mul(decimal.NewFromInt(1000))
	ob.ShareCapital = ob.ShareCapital.Sub(removed)
	if ob.ShareCapital.IsNegative() {
		ob.ShareCapital = decimal.Zero
	}
	ob.CashAndSecurities = ob.CashAndSecurities.Sub(removed)
	if ob.CashAndSecurities.IsNegative() {
		ob.CashAndSecurities = decimal.Zero
	}

	if err := s.settingsRepo.UpsertOpeningBalance(ob); err != nil {
		return nil, apierror.Internal("failed to update opening balance")
	}

	round.OpeningBalanceSynced = false
	round.OpeningBalanceSyncedAmountK = nil
	round.IsOpeningBalanceDivergent = false
	if err := s.capTableRepo.UpdateRound(round); err != nil {
		return nil, apierror.Internal("failed to update round sync status")
	}

	s.publishWithChanges(ctx, tenantID, round.ScenarioID, "cap_table_round", event.ActionUpdate, map[string]any{
		"roundId":              round.ID,
		"openingBalanceSynced": false,
	})
	return round, nil
}

// UnlinkFromFiplan removes the FiPlan link from a round and clears the
// cap_table_round_id / cap_table_round_label fields on the FiPlan entry.
// The amount in FiPlan is preserved; the user can adjust it manually.
func (s *CapTableService) UnlinkFromFiplan(ctx context.Context, tenantID uuid.UUID, roundID uuid.UUID) (*model.CapTableRound, error) {
	if s.fiplanSvc == nil {
		return nil, apierror.Internal("FiPlan sync is not available")
	}

	round, err := s.capTableRepo.GetRound(tenantID, roundID)
	if err != nil {
		return nil, apierror.NotFound("round", roundID.String())
	}
	if !round.FiplanSynced || round.FiscalYearIndex == nil {
		return nil, apierror.BadRequest("round is not currently linked to FiPlan")
	}

	if err := s.fiplanSvc.ClearCapTableLink(ctx, tenantID, round.ScenarioID, *round.FiscalYearIndex); err != nil {
		return nil, err
	}

	round.FiplanSynced = false
	round.FiscalYearIndex = nil
	round.FiplanSyncedAmountK = nil
	round.IsSyncAmountDivergent = false
	if err := s.capTableRepo.UpdateRound(round); err != nil {
		s.logger.WithError(err).Error("failed to clear round sync status")
		return nil, apierror.Internal("failed to clear round sync status")
	}

	s.publishWithChanges(ctx, tenantID, round.ScenarioID, "cap_table_round", event.ActionUpdate, map[string]any{
		"roundId":      round.ID,
		"fiplanSynced": false,
	})
	return round, nil
}

// ─── Full cap table report ────────────────────────────────────────────────────

// GetReport computes and returns the complete CapTableReport for a scenario.
func (s *CapTableService) GetReport(ctx context.Context, tenantID, scenarioID uuid.UUID) (*model.CapTableReport, error) {
	company, err := s.GetCompany(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	shareholders, err := s.ListShareholders(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	classes, err := s.ListShareClasses(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	rounds, err := s.ListRounds(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	plans, err := s.ListPlans(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	grants, err := s.ListGrants(ctx, tenantID, scenarioID)
	if err != nil {
		return nil, err
	}

	vScenarios, err := s.capTableRepo.ListValuationScenarios(tenantID, scenarioID)
	if err != nil {
		return nil, apierror.Internal("failed to load valuation scenarios")
	}
	vScen := make([]model.ValuationScenario, len(vScenarios))
	for i, vs := range vScenarios {
		vScen[i] = *vs
	}

	report := compute.ComputeCapTable(
		*company, shareholders, classes, rounds, plans, grants, vScen,
	)
	return &report, nil
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func (s *CapTableService) publish(ctx context.Context, tenantID, scenarioID uuid.UUID, entityType string, action event.Action) {
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: scenarioID,
		EntityType: entityType,
		Action:     action,
	})
}

func (s *CapTableService) publishWithChanges(ctx context.Context, tenantID, scenarioID uuid.UUID, entityType string, action event.Action, changes map[string]any) {
	s.emitter.Publish(event.Event{
		Type:       event.DataChanged,
		TenantID:   tenantID,
		UserID:     ctxutil.GetUserID(ctx),
		ScenarioID: scenarioID,
		EntityType: entityType,
		Action:     action,
		Changes:    marshalChanges(changes),
	})
}
