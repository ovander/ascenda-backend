package repo

import (
	"time"

	"ascenda/internal/model"
	"gorm.io/gorm"
)

// ─── AI Usage result types ────────────────────────────────────────────────────

// PlatformAIFeatureRow holds aggregated AI usage data for one feature across all tenants.
type PlatformAIFeatureRow struct {
	FeatureType        string  `gorm:"column:feature_type"`
	TotalCalls         int64   `gorm:"column:total_calls"`
	SuccessfulCalls    int64   `gorm:"column:successful_calls"`
	TotalTokens        int64   `gorm:"column:total_tokens"`
	EstimatedCostCents float64 `gorm:"column:estimated_cost_cents"`
}

// PlatformAITenantRow holds aggregated AI usage data per tenant.
type PlatformAITenantRow struct {
	TenantID           string  `gorm:"column:tenant_id"`
	TotalCalls         int64   `gorm:"column:total_calls"`
	SuccessfulCalls    int64   `gorm:"column:successful_calls"`
	TotalTokens        int64   `gorm:"column:total_tokens"`
	EstimatedCostCents float64 `gorm:"column:estimated_cost_cents"`
}

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

// PlanCount holds a plan value and its user count.
type PlanCount struct {
	Plan  string
	Count int64
}

// CountUsersByPlan returns the platform-wide number of users per commercial plan.
func (r *AdminStatsRepo) CountUsersByPlan() ([]PlanCount, error) {
	var results []PlanCount
	err := r.db.Model(&model.User{}).
		Select("plan, COUNT(*) as count").
		Group("plan").
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
			COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(u.email), ''), NULLIF(TRIM(u.external_id), ''), 'User #' || LEFT(a.user_id::text, 8)) AS user_email,
			a.created_at
		FROM audit_logs a
		LEFT JOIN business_plans bp ON bp.id = a.entity_id AND a.entity_type = 'plan'
		LEFT JOIN scenarios      s  ON s.id  = a.entity_id AND a.entity_type = 'scenario'
		LEFT JOIN users          u  ON u.id  = a.user_id
		WHERE a.entity_type IN ('plan', 'scenario')
		ORDER BY a.created_at DESC
		LIMIT ?
	`, limit).Scan(&rows).Error
	return rows, err
}

// GetPlatformAIUsageByFeature returns aggregated AI usage grouped by feature type
// for the given time range, across all tenants.
func (r *AdminStatsRepo) GetPlatformAIUsageByFeature(start, end time.Time) ([]PlatformAIFeatureRow, error) {
	var rows []PlatformAIFeatureRow
	err := r.db.Raw(`
		SELECT
			feature_type,
			COUNT(*)                              AS total_calls,
			SUM(CASE WHEN success THEN 1 ELSE 0 END) AS successful_calls,
			COALESCE(SUM(tokens_used), 0)         AS total_tokens,
			COALESCE(SUM(estimated_cost_cents), 0) AS estimated_cost_cents
		FROM ai_usage_records
		WHERE used_at >= ? AND used_at < ?
		GROUP BY feature_type
		ORDER BY total_calls DESC
	`, start, end).Scan(&rows).Error
	return rows, err
}

// GetPlatformAIUsageByTenant returns aggregated AI usage grouped by tenant
// for the given time range.
func (r *AdminStatsRepo) GetPlatformAIUsageByTenant(start, end time.Time) ([]PlatformAITenantRow, error) {
	var rows []PlatformAITenantRow
	err := r.db.Raw(`
		SELECT
			tenant_id::text                       AS tenant_id,
			COUNT(*)                              AS total_calls,
			SUM(CASE WHEN success THEN 1 ELSE 0 END) AS successful_calls,
			COALESCE(SUM(tokens_used), 0)         AS total_tokens,
			COALESCE(SUM(estimated_cost_cents), 0) AS estimated_cost_cents
		FROM ai_usage_records
		WHERE used_at >= ? AND used_at < ?
		GROUP BY tenant_id
		ORDER BY total_calls DESC
		LIMIT 50
	`, start, end).Scan(&rows).Error
	return rows, err
}

// CountPlatformAICalls returns total AI call counts for today, this week, and this month.
func (r *AdminStatsRepo) CountPlatformAICalls(now time.Time) (today, week, month int64, err error) {
	dailyKey := now.Format("2006-01-02")
	year, w := now.ISOWeek()
	weeklyKey := aiRecordFormatWeekKey(year, w)
	monthlyKey := now.Format("2006-01")

	type result struct {
		Today int64
		Week  int64
		Month int64
	}
	var res result
	err = r.db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM ai_usage_records WHERE daily_key   = ?) AS today,
			(SELECT COUNT(*) FROM ai_usage_records WHERE weekly_key  = ?) AS week,
			(SELECT COUNT(*) FROM ai_usage_records WHERE monthly_key = ?) AS month
	`, dailyKey, weeklyKey, monthlyKey).Scan(&res).Error
	return res.Today, res.Week, res.Month, err
}

// GetTopUsers returns users ranked by the number of plans they created, across all tenants.
func (r *AdminStatsRepo) GetTopUsers(limit int) ([]TopUserRow, error) {
	var rows []TopUserRow
	err := r.db.Raw(`
		SELECT
			COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(u.email), ''), NULLIF(TRIM(u.external_id), ''), 'User #' || LEFT(u.id::text, 8)) AS name,
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
