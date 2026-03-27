// Package repo — Break-Even Point repository.
package repo

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ascenda/internal/model"
)

// BEPRepo handles all BEP data operations.
// A single struct covers all eight BEP tables.
type BEPRepo struct {
	db *gorm.DB
}

// NewBEPRepo creates a new BEPRepo.
func NewBEPRepo(db *gorm.DB) *BEPRepo {
	return &BEPRepo{db: db}
}

// ─── BEPSnapshot ──────────────────────────────────────────────────────────────

func (r *BEPRepo) CreateSnapshot(s *model.BEPSnapshot) error {
	return r.db.Create(s).Error
}

func (r *BEPRepo) GetSnapshot(tenantID, id uuid.UUID) (*model.BEPSnapshot, error) {
	var s model.BEPSnapshot
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *BEPRepo) ListSnapshots(tenantID, scenarioID uuid.UUID) ([]*model.BEPSnapshot, error) {
	var rows []*model.BEPSnapshot
	err := r.db.
		Where("tenant_id = ? AND scenario_id = ?", tenantID, scenarioID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) UpdateSnapshot(s *model.BEPSnapshot) error {
	return r.db.
		Where("tenant_id = ? AND id = ?", s.TenantID, s.ID).
		Save(s).Error
}

func (r *BEPRepo) DeleteSnapshot(tenantID, id uuid.UUID) error {
	return r.db.
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.BEPSnapshot{}).Error
}

// ─── FixedCostLine ────────────────────────────────────────────────────────────

func (r *BEPRepo) ListFixedCostLines(tenantID, snapshotID uuid.UUID) ([]*model.FixedCostLine, error) {
	var rows []*model.FixedCostLine
	err := r.db.
		Where("tenant_id = ? AND snapshot_id = ?", tenantID, snapshotID).
		Order("sort_order, created_at").
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) BatchUpsertFixedCostLines(tenantID, snapshotID uuid.UUID, lines []model.FixedCostLine) error {
	// Delete orphaned lines (those no longer in the incoming batch).
	incomingIDs := make([]uuid.UUID, 0, len(lines))
	for _, l := range lines {
		if l.ID != uuid.Nil {
			incomingIDs = append(incomingIDs, l.ID)
		}
	}
	del := r.db.Where("tenant_id = ? AND snapshot_id = ?", tenantID, snapshotID)
	if len(incomingIDs) > 0 {
		del = del.Where("id NOT IN ?", incomingIDs)
	}
	if err := del.Delete(&model.FixedCostLine{}).Error; err != nil {
		return err
	}

	for i := range lines {
		lines[i].TenantID = tenantID
		lines[i].SnapshotID = snapshotID
		if lines[i].ID == uuid.Nil {
			lines[i].ID = uuid.New()
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"category", "label", "amount_annual", "is_custom_category", "sort_order", "updated_at",
		}),
	}).Create(&lines).Error
}

// ─── VariableCostLine ─────────────────────────────────────────────────────────

func (r *BEPRepo) ListVariableCostLines(tenantID, snapshotID uuid.UUID) ([]*model.VariableCostLine, error) {
	var rows []*model.VariableCostLine
	err := r.db.
		Where("tenant_id = ? AND snapshot_id = ?", tenantID, snapshotID).
		Order("sort_order, created_at").
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) BatchUpsertVariableCostLines(tenantID, snapshotID uuid.UUID, lines []model.VariableCostLine) error {
	incomingIDs := make([]uuid.UUID, 0, len(lines))
	for _, l := range lines {
		if l.ID != uuid.Nil {
			incomingIDs = append(incomingIDs, l.ID)
		}
	}
	del := r.db.Where("tenant_id = ? AND snapshot_id = ?", tenantID, snapshotID)
	if len(incomingIDs) > 0 {
		del = del.Where("id NOT IN ?", incomingIDs)
	}
	if err := del.Delete(&model.VariableCostLine{}).Error; err != nil {
		return err
	}

	for i := range lines {
		lines[i].TenantID = tenantID
		lines[i].SnapshotID = snapshotID
		if lines[i].ID == uuid.Nil {
			lines[i].ID = uuid.New()
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"category", "label", "amount_per_unit", "is_custom_category", "sort_order", "updated_at",
		}),
	}).Create(&lines).Error
}

// ─── SensitivityConfig ────────────────────────────────────────────────────────

func (r *BEPRepo) ListSensitivityConfigs(tenantID, snapshotID uuid.UUID) ([]*model.SensitivityConfig, error) {
	var rows []*model.SensitivityConfig
	err := r.db.
		Where("tenant_id = ? AND snapshot_id = ?", tenantID, snapshotID).
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) UpsertSensitivityConfig(cfg *model.SensitivityConfig) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "snapshot_id"}, {Name: "analysis_type"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"step_size_pct", "range_pct", "is_default", "updated_at",
		}),
	}).Create(cfg).Error
}

// ─── OptimisationPlan ─────────────────────────────────────────────────────────

func (r *BEPRepo) CreateOptimisationPlan(p *model.OptimisationPlan) error {
	return r.db.Create(p).Error
}

func (r *BEPRepo) GetOptimisationPlan(tenantID, id uuid.UUID) (*model.OptimisationPlan, error) {
	var p model.OptimisationPlan
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *BEPRepo) ListOptimisationPlans(tenantID, snapshotID uuid.UUID) ([]*model.OptimisationPlan, error) {
	var rows []*model.OptimisationPlan
	err := r.db.
		Where("tenant_id = ? AND snapshot_id = ?", tenantID, snapshotID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) UpdateOptimisationPlan(p *model.OptimisationPlan) error {
	return r.db.
		Where("tenant_id = ? AND id = ?", p.TenantID, p.ID).
		Save(p).Error
}

func (r *BEPRepo) DeleteOptimisationPlan(tenantID, id uuid.UUID) error {
	return r.db.
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.OptimisationPlan{}).Error
}

// ─── FixedCostSaving ─────────────────────────────────────────────────────────

func (r *BEPRepo) ListFixedCostSavings(tenantID, planID uuid.UUID) ([]*model.FixedCostSaving, error) {
	var rows []*model.FixedCostSaving
	err := r.db.
		Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Order("created_at").
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) BatchUpsertFixedCostSavings(tenantID, planID uuid.UUID, savings []model.FixedCostSaving) error {
	incomingIDs := make([]uuid.UUID, 0, len(savings))
	for _, s := range savings {
		if s.ID != uuid.Nil {
			incomingIDs = append(incomingIDs, s.ID)
		}
	}
	del := r.db.Where("tenant_id = ? AND plan_id = ?", tenantID, planID)
	if len(incomingIDs) > 0 {
		del = del.Where("id NOT IN ?", incomingIDs)
	}
	if err := del.Delete(&model.FixedCostSaving{}).Error; err != nil {
		return err
	}

	for i := range savings {
		savings[i].TenantID = tenantID
		savings[i].PlanID = planID
		if savings[i].ID == uuid.Nil {
			savings[i].ID = uuid.New()
		}
	}
	if len(savings) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"fixed_cost_line_id", "saving_amount", "new_amount", "comment", "pcg_account_refs", "updated_at",
		}),
	}).Create(&savings).Error
}

// ─── VariableCostSaving ───────────────────────────────────────────────────────

func (r *BEPRepo) ListVariableCostSavings(tenantID, planID uuid.UUID) ([]*model.VariableCostSaving, error) {
	var rows []*model.VariableCostSaving
	err := r.db.
		Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Order("created_at").
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) BatchUpsertVariableCostSavings(tenantID, planID uuid.UUID, savings []model.VariableCostSaving) error {
	incomingIDs := make([]uuid.UUID, 0, len(savings))
	for _, s := range savings {
		if s.ID != uuid.Nil {
			incomingIDs = append(incomingIDs, s.ID)
		}
	}
	del := r.db.Where("tenant_id = ? AND plan_id = ?", tenantID, planID)
	if len(incomingIDs) > 0 {
		del = del.Where("id NOT IN ?", incomingIDs)
	}
	if err := del.Delete(&model.VariableCostSaving{}).Error; err != nil {
		return err
	}

	for i := range savings {
		savings[i].TenantID = tenantID
		savings[i].PlanID = planID
		if savings[i].ID == uuid.Nil {
			savings[i].ID = uuid.New()
		}
	}
	if len(savings) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"variable_cost_line_id", "saving_amount", "new_amount", "comment", "updated_at",
		}),
	}).Create(&savings).Error
}

// ─── PCGReviewItem ────────────────────────────────────────────────────────────

func (r *BEPRepo) ListPCGReviewItems(tenantID, planID uuid.UUID) ([]*model.PCGReviewItem, error) {
	var rows []*model.PCGReviewItem
	err := r.db.
		Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Order("pcg_code").
		Find(&rows).Error
	return rows, err
}

func (r *BEPRepo) BatchUpsertPCGReviewItems(tenantID, planID uuid.UUID, items []model.PCGReviewItem) error {
	for i := range items {
		items[i].TenantID = tenantID
		items[i].PlanID = planID
		if items[i].ID == uuid.Nil {
			items[i].ID = uuid.New()
		}
	}
	if len(items) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "plan_id"}, {Name: "pcg_code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"pcg_label", "checked", "comment", "updated_at",
		}),
	}).Create(&items).Error
}
