package service

import (
	"encoding/json"

	"github.com/google/uuid"
	"kerplan/internal/compute"
	"kerplan/internal/model"
	"kerplan/internal/repo"
)

// SnapshotSection handles capture and restore for one logical data group.
// Capture returns key→JSON pairs to include in the snapshot.
// Restore receives the full snapshot data map and writes its entities back.
type SnapshotSection struct {
	Name    string
	Capture func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error)
	Restore func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error
}

// registerSections builds the ordered list of snapshot sections.
func registerSections(repos *repo.RepoBundle) []SnapshotSection {
	return []SnapshotSection{
		settingsSection(repos),
		productsSection(repos),
		staffSection(repos),
		capexSection(repos),
		opexSection(repos),
		pnlSection(repos),
		fiplanSection(repos),
		pnlCashSection(repos),
		wcrSection(repos),
		cashSection(repos),
		budgetSection(repos),
	}
}

// ---------------------------------------------------------------------------
// Settings (config, opening balance, working capital config)
// ---------------------------------------------------------------------------

func settingsSection(repos *repo.RepoBundle) SnapshotSection {
	return SnapshotSection{
		Name: "settings",
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			if config, err := repos.Settings.GetConfig(tenantID, scenarioID); err == nil && config != nil {
				result["config"], _ = json.Marshal(config)
			}
			if balance, err := repos.Settings.GetOpeningBalance(tenantID, scenarioID); err == nil && balance != nil {
				result["openingBalance"], _ = json.Marshal(balance)
			}
			if wc, err := repos.Settings.GetWCConfig(tenantID, scenarioID); err == nil && wc != nil {
				result["wcConfig"], _ = json.Marshal(wc)
			}
			return result, nil
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			if raw, ok := data["config"]; ok && len(raw) > 0 {
				var config model.PlanConfig
				if err := json.Unmarshal(raw, &config); err == nil {
					config.ID = uuid.New()
					config.TenantID = tenantID
					config.ScenarioID = scenarioID
					_ = repos.Settings.UpsertConfig(&config)
				}
			}
			if raw, ok := data["openingBalance"]; ok && len(raw) > 0 {
				var ob model.OpeningBalance
				if err := json.Unmarshal(raw, &ob); err == nil {
					ob.ID = uuid.New()
					ob.TenantID = tenantID
					ob.ScenarioID = scenarioID
					_ = repos.Settings.UpsertOpeningBalance(&ob)
				}
			}
			if raw, ok := data["wcConfig"]; ok && len(raw) > 0 {
				var wc model.WorkingCapitalConfig
				if err := json.Unmarshal(raw, &wc); err == nil {
					wc.ID = uuid.New()
					wc.TenantID = tenantID
					wc.ScenarioID = scenarioID
					_ = repos.Settings.UpsertWCConfig(&wc)
				}
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Products (products + assumptions + volumes + margins with ID remapping)
// ---------------------------------------------------------------------------

func productsSection(repos *repo.RepoBundle) SnapshotSection {
	return SnapshotSection{
		Name: "products",
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			products, err := repos.Product.ListProductsByScenario(tenantID, scenarioID)
			if err != nil {
				return result, nil
			}
			result["products"], _ = json.Marshal(products)

			// Batch-fetch: 3 queries total instead of 3N
			if ptrA, err := repos.Product.GetAssumptionsByScenario(tenantID, scenarioID); err == nil {
				allAssumptions := make([]model.ProductAssumption, len(ptrA))
				for i, a := range ptrA {
					allAssumptions[i] = *a
				}
				result["productAssumptions"], _ = json.Marshal(allAssumptions)
			}
			if ptrV, err := repos.Product.GetVolumesByScenario(tenantID, scenarioID); err == nil {
				allVolumes := make([]model.ProductSalesVolume, len(ptrV))
				for i, v := range ptrV {
					allVolumes[i] = *v
				}
				result["productSalesVolumes"], _ = json.Marshal(allVolumes)
			}
			if ptrM, err := repos.Product.GetMarginsByScenario(tenantID, scenarioID); err == nil {
				allMargins := make([]model.ProductDistributorMargin, len(ptrM))
				for i, m := range ptrM {
					allMargins[i] = *m
				}
				result["productDistributorMargins"], _ = json.Marshal(allMargins)
			}
			return result, nil
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			oldToNewProduct := make(map[uuid.UUID]uuid.UUID)

			if raw, ok := data["products"]; ok && len(raw) > 0 {
				var products []model.Product
				if err := json.Unmarshal(raw, &products); err == nil {
					for i := range products {
						oldID := products[i].ID
						products[i].ID = uuid.New()
						products[i].TenantID = tenantID
						products[i].ScenarioID = scenarioID
						oldToNewProduct[oldID] = products[i].ID
						_ = repos.Product.CreateProduct(&products[i])
					}
				}
			}

			if raw, ok := data["productAssumptions"]; ok && len(raw) > 0 {
				var assumptions []model.ProductAssumption
				if err := json.Unmarshal(raw, &assumptions); err == nil {
					for i := range assumptions {
						assumptions[i].ID = uuid.New()
						assumptions[i].TenantID = tenantID
						if newID, ok := oldToNewProduct[assumptions[i].ProductID]; ok {
							assumptions[i].ProductID = newID
						}
					}
					_ = repos.Product.BatchUpsertAssumptions(tenantID, scenarioID, assumptions)
				}
			}

			if raw, ok := data["productSalesVolumes"]; ok && len(raw) > 0 {
				var volumes []model.ProductSalesVolume
				if err := json.Unmarshal(raw, &volumes); err == nil {
					for i := range volumes {
						volumes[i].ID = uuid.New()
						volumes[i].TenantID = tenantID
						if newID, ok := oldToNewProduct[volumes[i].ProductID]; ok {
							volumes[i].ProductID = newID
						}
					}
					_ = repos.Product.BatchUpsertVolumes(tenantID, scenarioID, volumes)
				}
			}

			if raw, ok := data["productDistributorMargins"]; ok && len(raw) > 0 {
				var margins []model.ProductDistributorMargin
				if err := json.Unmarshal(raw, &margins); err == nil {
					for i := range margins {
						margins[i].ID = uuid.New()
						margins[i].TenantID = tenantID
						if newID, ok := oldToNewProduct[margins[i].ProductID]; ok {
							margins[i].ProductID = newID
						}
					}
					_ = repos.Product.BatchUpsertMargins(tenantID, scenarioID, margins)
				}
			}

			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Staff (headcounts + salaries + incentives)
// ---------------------------------------------------------------------------

func staffSection(repos *repo.RepoBundle) SnapshotSection {
	return SnapshotSection{
		Name: "staff",
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			if hc, err := repos.Staff.ListHeadcountsByScenario(tenantID, scenarioID); err == nil {
				result["staffHeadcounts"], _ = json.Marshal(hc)
			}
			if sal, err := repos.Staff.ListSalariesByScenario(tenantID, scenarioID); err == nil {
				result["staffSalaries"], _ = json.Marshal(sal)
			}
			if inc, err := repos.Staff.ListIncentivesByScenario(tenantID, scenarioID); err == nil {
				result["staffIncentives"], _ = json.Marshal(inc)
			}
			return result, nil
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			if raw, ok := data["staffHeadcounts"]; ok && len(raw) > 0 {
				var hc []model.StaffHeadcount
				if err := json.Unmarshal(raw, &hc); err == nil {
					for i := range hc {
						hc[i].ID = uuid.New()
						hc[i].TenantID = tenantID
						hc[i].ScenarioID = scenarioID
					}
					_ = repos.Staff.BatchUpsertHeadcounts(tenantID, scenarioID, hc)
				}
			}
			if raw, ok := data["staffSalaries"]; ok && len(raw) > 0 {
				var sal []model.StaffSalary
				if err := json.Unmarshal(raw, &sal); err == nil {
					for i := range sal {
						sal[i].ID = uuid.New()
						sal[i].TenantID = tenantID
						sal[i].ScenarioID = scenarioID
					}
					_ = repos.Staff.BatchUpsertSalaries(tenantID, scenarioID, sal)
				}
			}
			if raw, ok := data["staffIncentives"]; ok && len(raw) > 0 {
				var inc []model.StaffIncentive
				if err := json.Unmarshal(raw, &inc); err == nil {
					for i := range inc {
						inc[i].ID = uuid.New()
						inc[i].TenantID = tenantID
						inc[i].ScenarioID = scenarioID
					}
					_ = repos.Staff.BatchUpsertIncentives(tenantID, scenarioID, inc)
				}
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Simple single-key sections (capex, opex, pnl, fiplan, pnlCash, wcr, cash)
// ---------------------------------------------------------------------------

func capexSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("capex", "capexEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.Capex.ListByScenario(tID, sID) },
		func(tID, sID uuid.UUID, raw json.RawMessage) error {
			var entries []model.CapexEntry
			if err := json.Unmarshal(raw, &entries); err != nil {
				return err
			}
			for i := range entries {
				entries[i].ID = uuid.New()
				entries[i].TenantID = tID
				entries[i].ScenarioID = sID
			}
			return repos.Capex.BatchUpsert(tID, sID, entries)
		},
	)
}

func opexSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("opex", "opexEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.Opex.ListByScenario(tID, sID) },
		func(tID, sID uuid.UUID, raw json.RawMessage) error {
			var entries []model.OpexManualEntry
			if err := json.Unmarshal(raw, &entries); err != nil {
				return err
			}
			for i := range entries {
				entries[i].ID = uuid.New()
				entries[i].TenantID = tID
				entries[i].ScenarioID = sID
			}
			return repos.Opex.BatchUpsert(tID, sID, entries)
		},
	)
}

func pnlSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("pnl", "pnlEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.PnL.ListByScenario(tID, sID) },
		func(tID, sID uuid.UUID, raw json.RawMessage) error {
			var entries []model.PnlManualEntry
			if err := json.Unmarshal(raw, &entries); err != nil {
				return err
			}
			for i := range entries {
				entries[i].ID = uuid.New()
				entries[i].TenantID = tID
				entries[i].ScenarioID = sID
			}
			return repos.PnL.BatchUpsert(tID, sID, entries)
		},
	)
}

func fiplanSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("fiplan", "fiplanEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.FiPlan.ListByScenario(tID, sID) },
		func(tID, sID uuid.UUID, raw json.RawMessage) error {
			var entries []model.FiplanEntry
			if err := json.Unmarshal(raw, &entries); err != nil {
				return err
			}
			for i := range entries {
				entries[i].ID = uuid.New()
				entries[i].TenantID = tID
				entries[i].ScenarioID = sID
			}
			return repos.FiPlan.BatchUpsert(tID, sID, entries)
		},
	)
}

func pnlCashSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("pnlCash", "pnlCashEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.PnlCash.ListByScenario(tID, sID) },
		func(tID, sID uuid.UUID, raw json.RawMessage) error {
			var entries []model.PnlCashEntry
			if err := json.Unmarshal(raw, &entries); err != nil {
				return err
			}
			for i := range entries {
				entries[i].ID = uuid.New()
				entries[i].TenantID = tID
				entries[i].ScenarioID = sID
			}
			return repos.PnlCash.BatchUpsert(tID, sID, entries)
		},
	)
}

func wcrSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("wcr", "wcrEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.WCR.ListByScenario(tID, sID) },
		func(tID, sID uuid.UUID, raw json.RawMessage) error {
			var entries []model.WCREntry
			if err := json.Unmarshal(raw, &entries); err != nil {
				return err
			}
			for i := range entries {
				entries[i].ID = uuid.New()
				entries[i].TenantID = tID
				entries[i].ScenarioID = sID
			}
			return repos.WCR.BatchUpsert(tID, sID, entries)
		},
	)
}

func cashSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("cash", "cashOverrides",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.Cash.ListByScenario(tID, sID) },
		func(tID, sID uuid.UUID, raw json.RawMessage) error {
			var entries []model.CashMonthlyOverride
			if err := json.Unmarshal(raw, &entries); err != nil {
				return err
			}
			for i := range entries {
				entries[i].ID = uuid.New()
				entries[i].TenantID = tID
				entries[i].ScenarioID = sID
			}
			return repos.Cash.BatchUpsert(tID, sID, entries)
		},
	)
}

// ---------------------------------------------------------------------------
// Budget (captures all years, uses compute.MaxYears)
// ---------------------------------------------------------------------------

func budgetSection(repos *repo.RepoBundle) SnapshotSection {
	return SnapshotSection{
		Name: "budget",
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			var allBudgets []model.BudgetMonthlyOverride
			for year := 1; year <= compute.MaxYears; year++ {
				if ptrEntries, err := repos.Budget.ListByScenario(tenantID, scenarioID, year); err == nil {
					for _, e := range ptrEntries {
						allBudgets = append(allBudgets, *e)
					}
				}
			}
			result["budgetOverrides"], _ = json.Marshal(allBudgets)
			return result, nil
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			if raw, ok := data["budgetOverrides"]; ok && len(raw) > 0 {
				var entries []model.BudgetMonthlyOverride
				if err := json.Unmarshal(raw, &entries); err == nil {
					for i := range entries {
						entries[i].ID = uuid.New()
						entries[i].TenantID = tenantID
						entries[i].ScenarioID = scenarioID
					}
					_ = repos.Budget.BatchUpsert(tenantID, scenarioID, entries)
				}
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// simpleSection is a helper for single-key capture/restore sections.
// ---------------------------------------------------------------------------

func simpleSection(
	name string,
	key string,
	listFn func(tenantID, scenarioID uuid.UUID) (interface{}, error),
	restoreFn func(tenantID, scenarioID uuid.UUID, raw json.RawMessage) error,
) SnapshotSection {
	return SnapshotSection{
		Name: name,
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			if entries, err := listFn(tenantID, scenarioID); err == nil {
				result[key], _ = json.Marshal(entries)
			}
			return result, nil
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			if raw, ok := data[key]; ok && len(raw) > 0 {
				return restoreFn(tenantID, scenarioID, raw)
			}
			return nil
		},
	}
}
