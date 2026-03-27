package repo

import (
	"time"

	"gorm.io/gorm"
	"ascenda/internal/model"
)

// AdminStatsRepo runs platform-wide aggregate queries for the admin dashboard.
// All queries are intentionally unscoped by tenant_id — the platform admin sees
// data across all tenants.
type AdminStatsRepo struct {
	db *gorm.DB
}

// NewAdminStatsRepo creates a new AdminStatsRepo.
func NewAdminStatsRepo(db *gorm.DB) *AdminStatsRepo {
	return &AdminStatsRepo{db: db}
}

// ─── Result types ─────────────────────────────────────────────────────────────

// UserRoleCount holds a role and its user count.
type UserRoleCount struct {
	Role  string
	Count int64
}

// PlanStatusCount holds a plan status and its count.
type PlanStatusCount struct {
	Status string
	Count  int64
}

// ActivityRow holds one recent-activity record.
type ActivityRow struct {
	EntityType string    `gorm:"column:entity_type"`
	EntityName string    `gorm:"column:entity_name"`
	Action     string    `gorm:"column:action"`
	UserEmail  string    `gorm:"column:user_email"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

// TopUserRow holds one per-user activity stats record.
type TopUserRow struct {
	Name          string `gorm:"column:name"`
	Email         string `gorm:"column:email"`
	TenantID      string `gorm:"column:tenant_id"`
	PlanCount     int64  `gorm:"column:plan_count"`
	ScenarioCount int64  `gorm:"column:scenario_count"`
}

// ─── Queries ──────────────────────────────────────────────────────────────────

// CountUsersByRole returns the platform-wide number of users per role.
func (r *AdminStatsRepo) CountUsersByRole() ([]UserRoleCount, error) {
	var results []UserRoleCount
	err := r.db.Model(&model.User{}).
		Select("role, COUNT(*) as count").
		Group("role").
		Scan(&results).Error
	return results, err
}

// CountUsersByActivity returns the platform-wide active and inactive user counts.
func (r *AdminStatsRepo) CountUsersByActivity() (active, inactive int64, err error) {
	if e := r.db.Model(&model.User{}).
		Where("is_active = ?", true).
		Count(&active).Error; e != nil {
		err = e
		return
	}
	err = r.db.Model(&model.User{}).
		Where("is_active = ?", false).
		Count(&inactive).Error
	return
}

// CountPlansByStatus returns the platform-wide number of plans per status.
func (r *AdminStatsRepo) CountPlansByStatus() ([]PlanStatusCount, error) {
	var results []PlanStatusCount
	err := r.db.Model(&model.BusinessPlan{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&results).Error
	return results, err
}

// CountScenarios returns the platform-wide total number of scenarios.
func (r *AdminStatsRepo) CountScenarios() (int64, error) {
	var count int64
	err := r.db.Model(&model.Scenario{}).
		Count(&count).Error
	return count, err
}

// CountTenants returns the total number of tenants on the platform.
func (r *AdminStatsRepo) CountTenants() (int64, error) {
	var count int64
	err := r.db.Model(&model.Tenant{}).
		Count(&count).Error
	return count, err
}

// GetRecentActivity returns the latest plan/scenario audit-log events across all tenants.
func (r *AdminStatsRepo) GetRecentActivity(limit int) ([]ActivityRow, error) {
	var rows []ActivityRow
	err := r.db.Raw(`
		SELECT
			a.entity_type,
			COALESCE(bp.name, s.name, '') AS entity_name,
			a.action,
			u.email AS user_email,
			a.created_at
		FROM audit_logs a
		LEFT JOIN business_plans bp ON bp.id = a.entity_id AND a.entity_type = 'plan'
		LEFT JOIN scenarios      s  ON s.id  = a.entity_id AND a.entity_type = 'scenario'
		JOIN  users u ON u.id = a.user_id
		WHERE a.entity_type IN ('plan', 'scenario')
		ORDER BY a.created_at DESC
		LIMIT ?
	`, limit).Scan(&rows).Error
	return rows, err
}

// GetTopUsers returns users ranked by the number of plans they created, across all tenants.
func (r *AdminStatsRepo) GetTopUsers(limit int) ([]TopUserRow, error) {
	var rows []TopUserRow
	err := r.db.Raw(`
		SELECT
			u.name,
			u.email,
			u.tenant_id::text AS tenant_id,
			COUNT(DISTINCT bp.id) AS plan_count,
			COUNT(DISTINCT s.id)  AS scenario_count
		FROM users u
		LEFT JOIN business_plans bp ON bp.created_by = u.id
		LEFT JOIN scenarios      s  ON s.plan_id    = bp.id
		GROUP BY u.id, u.name, u.email, u.tenant_id
		ORDER BY plan_count DESC, scenario_count DESC
		LIMIT ?
	`, limit).Scan(&rows).Error
	return rows, err
}
