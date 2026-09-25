package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"ascenda/internal/compute"
	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SnapshotSection handles capture and restore for one logical data group.
//
// Capture returns key→JSON pairs to include in the snapshot. Clear empties the
// section's tables for a scenario and Restore writes the snapshot's entities
// back; ScenarioRestorer runs Clear for every section, then Restore for every
// section, inside one transaction, so both must return every error they meet
// (an error rolls the whole restore back) and must accept a missing key as
// "nothing to write".
type SnapshotSection struct {
	Name    string
	Capture func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error)
	Clear   func(tenantID, scenarioID uuid.UUID) error
	Restore func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error
}

// decodeSection unmarshals one snapshot key into a slice of T. A missing or
// empty key yields nil, nil; malformed JSON is an error so that a corrupt
// snapshot aborts the restore instead of silently restoring a subset.
func decodeSection[T any](data map[string]json.RawMessage, key string) ([]T, error) {
	raw, ok := data[key]
	if !ok || len(raw) == 0 {
		return nil, nil
	}
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	return out, nil
}

// decodeOne is decodeSection for a single-object key.
func decodeOne[T any](data map[string]json.RawMessage, key string) (*T, error) {
	raw, ok := data[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	return &out, nil
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

// capture marshals the rows returned by fetch under key. A fetch error aborts
// the capture: a snapshot missing a section would, on restore, wipe that
// section, so it must never be produced silently.
func capture(result map[string]json.RawMessage, key string, fetch func() (interface{}, error)) error {
	rows, err := fetch()
	if err != nil {
		return fmt.Errorf("read %s: %w", key, err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	result[key] = raw
	return nil
}

// captureOptional is capture for a single optional row: a not-found result
// leaves the key out instead of failing.
func captureOptional[T any](result map[string]json.RawMessage, key string, fetch func() (*T, error)) error {
	row, err := fetch()
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && row == nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", key, err)
	}
	raw, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	result[key] = raw
	return nil
}

// ---------------------------------------------------------------------------
// Settings (config, opening balance, working capital config)
// ---------------------------------------------------------------------------

func settingsSection(repos *repo.RepoBundle) SnapshotSection {
	return SnapshotSection{
		Name: "settings",
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			if err := captureOptional(result, "config", func() (*model.PlanConfig, error) {
				return repos.Settings.GetConfig(tenantID, scenarioID)
			}); err != nil {
				return nil, err
			}
			if err := captureOptional(result, "openingBalance", func() (*model.OpeningBalance, error) {
				return repos.Settings.GetOpeningBalance(tenantID, scenarioID)
			}); err != nil {
				return nil, err
			}
			if err := captureOptional(result, "wcConfig", func() (*model.WorkingCapitalConfig, error) {
				return repos.Settings.GetWCConfig(tenantID, scenarioID)
			}); err != nil {
				return nil, err
			}
			return result, nil
		},
		Clear: func(tenantID, scenarioID uuid.UUID) error {
			return repos.Settings.DeleteCoreByScenario(tenantID, scenarioID)
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			config, err := decodeOne[model.PlanConfig](data, "config")
			if err != nil {
				return err
			}
			if config != nil {
				config.ID = uuid.New()
				config.TenantID = tenantID
				config.ScenarioID = scenarioID
				if err := repos.Settings.UpsertConfig(config); err != nil {
					return fmt.Errorf("write config: %w", err)
				}
			}
			ob, err := decodeOne[model.OpeningBalance](data, "openingBalance")
			if err != nil {
				return err
			}
			if ob != nil {
				ob.ID = uuid.New()
				ob.TenantID = tenantID
				ob.ScenarioID = scenarioID
				if err := repos.Settings.UpsertOpeningBalance(ob); err != nil {
					return fmt.Errorf("write opening balance: %w", err)
				}
			}
			wc, err := decodeOne[model.WorkingCapitalConfig](data, "wcConfig")
			if err != nil {
				return err
			}
			if wc != nil {
				wc.ID = uuid.New()
				wc.TenantID = tenantID
				wc.ScenarioID = scenarioID
				if err := repos.Settings.UpsertWCConfig(wc); err != nil {
					return fmt.Errorf("write working capital config: %w", err)
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
			// Batch-fetch: 4 queries total instead of 1+3N.
			steps := []struct {
				key   string
				fetch func() (interface{}, error)
			}{
				{"products", func() (interface{}, error) { return repos.Product.ListProductsByScenario(tenantID, scenarioID) }},
				{"productAssumptions", func() (interface{}, error) { return repos.Product.GetAssumptionsByScenario(tenantID, scenarioID) }},
				{"productSalesVolumes", func() (interface{}, error) { return repos.Product.GetVolumesByScenario(tenantID, scenarioID) }},
				{"productDistributorMargins", func() (interface{}, error) { return repos.Product.GetMarginsByScenario(tenantID, scenarioID) }},
			}
			for _, step := range steps {
				if err := capture(result, step.key, step.fetch); err != nil {
					return nil, err
				}
			}
			return result, nil
		},
		Clear: func(tenantID, scenarioID uuid.UUID) error {
			return repos.Product.DeleteByScenario(tenantID, scenarioID)
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			products, err := decodeSection[model.Product](data, "products")
			if err != nil {
				return err
			}
			// Products get fresh IDs; child rows are re-pointed through this map.
			// A child whose product is not in the snapshot is dropped: it could
			// only reference a product the restore has just deleted.
			oldToNewProduct := make(map[uuid.UUID]uuid.UUID, len(products))
			for i := range products {
				oldID := products[i].ID
				products[i].ID = uuid.New()
				products[i].TenantID = tenantID
				products[i].ScenarioID = scenarioID
				oldToNewProduct[oldID] = products[i].ID
				if err := repos.Product.CreateProduct(&products[i]); err != nil {
					return fmt.Errorf("write product %q: %w", products[i].Name, err)
				}
			}

			assumptions, err := decodeSection[model.ProductAssumption](data, "productAssumptions")
			if err != nil {
				return err
			}
			for productID, rows := range groupByProduct(assumptions, oldToNewProduct,
				func(a *model.ProductAssumption) *uuid.UUID { return &a.ProductID }) {
				for i := range rows {
					rows[i].ID = uuid.New()
					rows[i].TenantID = tenantID
				}
				if err := repos.Product.BatchUpsertAssumptions(tenantID, scenarioID, rows); err != nil {
					return fmt.Errorf("write assumptions of product %s: %w", productID, err)
				}
			}

			volumes, err := decodeSection[model.ProductSalesVolume](data, "productSalesVolumes")
			if err != nil {
				return err
			}
			for productID, rows := range groupByProduct(volumes, oldToNewProduct,
				func(v *model.ProductSalesVolume) *uuid.UUID { return &v.ProductID }) {
				for i := range rows {
					rows[i].ID = uuid.New()
					rows[i].TenantID = tenantID
				}
				if err := repos.Product.BatchUpsertVolumes(tenantID, scenarioID, rows); err != nil {
					return fmt.Errorf("write sales volumes of product %s: %w", productID, err)
				}
			}

			margins, err := decodeSection[model.ProductDistributorMargin](data, "productDistributorMargins")
			if err != nil {
				return err
			}
			for productID, rows := range groupByProduct(margins, oldToNewProduct,
				func(m *model.ProductDistributorMargin) *uuid.UUID { return &m.ProductID }) {
				for i := range rows {
					rows[i].ID = uuid.New()
					rows[i].TenantID = tenantID
				}
				if err := repos.Product.BatchUpsertMargins(tenantID, scenarioID, rows); err != nil {
					return fmt.Errorf("write distributor margins of product %s: %w", productID, err)
				}
			}
			return nil
		},
	}
}

// groupByProduct re-points product child rows to their restored product and
// groups them by that product, because the product repository's batch upserts
// take the rows of one product at a time. Rows whose product is not in the
// map are dropped.
func groupByProduct[T any](rows []T, oldToNew map[uuid.UUID]uuid.UUID, productID func(*T) *uuid.UUID) map[uuid.UUID][]T {
	groups := make(map[uuid.UUID][]T)
	for i := range rows {
		id := productID(&rows[i])
		newID, ok := oldToNew[*id]
		if !ok {
			continue
		}
		*id = newID
		groups[newID] = append(groups[newID], rows[i])
	}
	return groups
}

// ---------------------------------------------------------------------------
// Staff (headcounts + salaries + incentives)
// ---------------------------------------------------------------------------

func staffSection(repos *repo.RepoBundle) SnapshotSection {
	return SnapshotSection{
		Name: "staff",
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			steps := []struct {
				key   string
				fetch func() (interface{}, error)
			}{
				{"staffHeadcounts", func() (interface{}, error) { return repos.Staff.ListHeadcountsByScenario(tenantID, scenarioID) }},
				{"staffSalaries", func() (interface{}, error) { return repos.Staff.ListSalariesByScenario(tenantID, scenarioID) }},
				{"staffIncentives", func() (interface{}, error) { return repos.Staff.ListIncentivesByScenario(tenantID, scenarioID) }},
			}
			for _, step := range steps {
				if err := capture(result, step.key, step.fetch); err != nil {
					return nil, err
				}
			}
			return result, nil
		},
		Clear: func(tenantID, scenarioID uuid.UUID) error {
			return repos.Staff.DeleteByScenario(tenantID, scenarioID)
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			hc, err := decodeSection[model.StaffHeadcount](data, "staffHeadcounts")
			if err != nil {
				return err
			}
			if len(hc) > 0 {
				for i := range hc {
					hc[i].ID = uuid.New()
				}
				if err := repos.Staff.BatchUpsertHeadcounts(tenantID, scenarioID, hc); err != nil {
					return fmt.Errorf("write headcounts: %w", err)
				}
			}
			sal, err := decodeSection[model.StaffSalary](data, "staffSalaries")
			if err != nil {
				return err
			}
			if len(sal) > 0 {
				for i := range sal {
					sal[i].ID = uuid.New()
				}
				if err := repos.Staff.BatchUpsertSalaries(tenantID, scenarioID, sal); err != nil {
					return fmt.Errorf("write salaries: %w", err)
				}
			}
			inc, err := decodeSection[model.StaffIncentive](data, "staffIncentives")
			if err != nil {
				return err
			}
			if len(inc) > 0 {
				for i := range inc {
					inc[i].ID = uuid.New()
				}
				if err := repos.Staff.BatchUpsertIncentives(tenantID, scenarioID, inc); err != nil {
					return fmt.Errorf("write incentives: %w", err)
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
		repos.Capex.DeleteByScenario,
		repos.Capex.BatchUpsert,
		func(e *model.CapexEntry, tID, sID uuid.UUID) { e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID },
	)
}

func opexSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("opex", "opexEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.Opex.ListByScenario(tID, sID) },
		repos.Opex.DeleteByScenario,
		repos.Opex.BatchUpsert,
		func(e *model.OpexManualEntry, tID, sID uuid.UUID) {
			e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID
		},
	)
}

func pnlSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("pnl", "pnlEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.PnL.ListByScenario(tID, sID) },
		repos.PnL.DeleteByScenario,
		repos.PnL.BatchUpsert,
		func(e *model.PnlManualEntry, tID, sID uuid.UUID) {
			e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID
		},
	)
}

func fiplanSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("fiplan", "fiplanEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.FiPlan.ListByScenario(tID, sID) },
		repos.FiPlan.DeleteByScenario,
		repos.FiPlan.BatchUpsert,
		func(e *model.FiplanEntry, tID, sID uuid.UUID) { e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID },
	)
}

func pnlCashSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("pnlCash", "pnlCashEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.PnlCash.ListByScenario(tID, sID) },
		repos.PnlCash.DeleteByScenario,
		repos.PnlCash.BatchUpsert,
		func(e *model.PnlCashEntry, tID, sID uuid.UUID) { e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID },
	)
}

func wcrSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("wcr", "wcrEntries",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.WCR.ListByScenario(tID, sID) },
		repos.WCR.DeleteByScenario,
		repos.WCR.BatchUpsert,
		func(e *model.WCREntry, tID, sID uuid.UUID) { e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID },
	)
}

func cashSection(repos *repo.RepoBundle) SnapshotSection {
	return simpleSection("cash", "cashOverrides",
		func(tID, sID uuid.UUID) (interface{}, error) { return repos.Cash.ListByScenario(tID, sID) },
		repos.Cash.DeleteByScenario,
		repos.Cash.BatchUpsert,
		func(e *model.CashMonthlyOverride, tID, sID uuid.UUID) {
			e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID
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
			allBudgets := []model.BudgetMonthlyOverride{}
			for year := 1; year <= compute.MaxYears; year++ {
				ptrEntries, err := repos.Budget.ListByScenario(tenantID, scenarioID, year)
				if err != nil {
					return nil, fmt.Errorf("read budgetOverrides year %d: %w", year, err)
				}
				for _, e := range ptrEntries {
					allBudgets = append(allBudgets, *e)
				}
			}
			return result, capture(result, "budgetOverrides", func() (interface{}, error) { return allBudgets, nil })
		},
		Clear: func(tenantID, scenarioID uuid.UUID) error {
			return repos.Budget.DeleteByScenario(tenantID, scenarioID)
		},
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			return restoreEntries(tenantID, scenarioID, data, "budgetOverrides", repos.Budget.BatchUpsert,
				func(e *model.BudgetMonthlyOverride, tID, sID uuid.UUID) {
					e.ID, e.TenantID, e.ScenarioID = uuid.New(), tID, sID
				})
		},
	}
}

// ---------------------------------------------------------------------------
// simpleSection is a helper for single-key capture/restore sections whose
// rows are written with one batch upsert.
// ---------------------------------------------------------------------------

func simpleSection[T any](
	name string,
	key string,
	listFn func(tenantID, scenarioID uuid.UUID) (interface{}, error),
	clearFn func(tenantID, scenarioID uuid.UUID) error,
	upsertFn func(tenantID, scenarioID uuid.UUID, entries []T) error,
	retarget func(entry *T, tenantID, scenarioID uuid.UUID),
) SnapshotSection {
	return SnapshotSection{
		Name: name,
		Capture: func(tenantID, scenarioID uuid.UUID) (map[string]json.RawMessage, error) {
			result := make(map[string]json.RawMessage)
			if err := capture(result, key, func() (interface{}, error) { return listFn(tenantID, scenarioID) }); err != nil {
				return nil, err
			}
			return result, nil
		},
		Clear: clearFn,
		Restore: func(tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage) error {
			return restoreEntries(tenantID, scenarioID, data, key, upsertFn, retarget)
		},
	}
}

// restoreEntries decodes data[key], re-targets every row (fresh ID, target
// tenant and scenario) and writes them in one batch. No rows → nothing written.
func restoreEntries[T any](tenantID, scenarioID uuid.UUID, data map[string]json.RawMessage, key string,
	upsertFn func(tenantID, scenarioID uuid.UUID, entries []T) error,
	retarget func(entry *T, tenantID, scenarioID uuid.UUID)) error {
	entries, err := decodeSection[T](data, key)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	for i := range entries {
		retarget(&entries[i], tenantID, scenarioID)
	}
	if err := upsertFn(tenantID, scenarioID, entries); err != nil {
		return fmt.Errorf("write %s: %w", key, err)
	}
	return nil
}
