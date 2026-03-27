// Package repo — AI usage policy and record repositories.
// Ported from GPWA internal/repository/ai_usage_policy_repo.go and adapted for Ascenda
// (UUID tenant/user IDs, Ascenda feature types).
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"ascenda/internal/model"
)

// ============================================================================
// AI Usage Policy Repository
// ============================================================================

// AIUsagePolicyRepo handles persistence of AI usage policies.
type AIUsagePolicyRepo struct {
	db *gorm.DB
}

// NewAIUsagePolicyRepo creates a new AIUsagePolicyRepo.
func NewAIUsagePolicyRepo(db *gorm.DB) *AIUsagePolicyRepo {
	return &AIUsagePolicyRepo{db: db}
}

// GetPolicy retrieves a specific policy by role, tier, and feature.
func (r *AIUsagePolicyRepo) GetPolicy(ctx context.Context, tenantID uuid.UUID, role, tier string, feature model.AIFeatureType) (*model.AIUsagePolicy, error) {
	var policy model.AIUsagePolicy
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_role = ? AND subscription_tier = ? AND feature_type = ?",
			tenantID, role, tier, feature).
		First(&policy).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// ListPolicies retrieves all policies for a tenant.
func (r *AIUsagePolicyRepo) ListPolicies(ctx context.Context, tenantID uuid.UUID) ([]model.AIUsagePolicy, error) {
	var policies []model.AIUsagePolicy
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("user_role, subscription_tier, feature_type").
		Find(&policies).Error
	return policies, err
}

// UpdatePolicy updates an existing policy.
func (r *AIUsagePolicyRepo) UpdatePolicy(ctx context.Context, tenantID uuid.UUID, policy *model.AIUsagePolicy) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Save(policy).Error
}

// BulkUpsertPolicies creates or updates multiple policies in a single transaction.
func (r *AIUsagePolicyRepo) BulkUpsertPolicies(ctx context.Context, tenantID uuid.UUID, policies []model.AIUsagePolicy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range policies {
			policies[i].TenantID = tenantID
			err := tx.
				Where("tenant_id = ? AND user_role = ? AND subscription_tier = ? AND feature_type = ?",
					tenantID, policies[i].UserRole, policies[i].SubscriptionTier, policies[i].FeatureType).
				Assign(&policies[i]).
				FirstOrCreate(&policies[i]).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// ============================================================================
// AI Usage Record Repository
// ============================================================================

// AIUsageRecordRepo handles persistence of AI usage records.
type AIUsageRecordRepo struct {
	db *gorm.DB
}

// NewAIUsageRecordRepo creates a new AIUsageRecordRepo.
func NewAIUsageRecordRepo(db *gorm.DB) *AIUsageRecordRepo {
	return &AIUsageRecordRepo{db: db}
}

// Create persists a new usage record.
func (r *AIUsageRecordRepo) Create(ctx context.Context, tenantID uuid.UUID, record *model.AIUsageRecord) error {
	record.TenantID = tenantID
	return r.db.WithContext(ctx).Create(record).Error
}

// GetUsageCounts returns daily, weekly, and monthly usage counts in a single query.
func (r *AIUsageRecordRepo) GetUsageCounts(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, feature model.AIFeatureType) (daily, weekly, monthly int, err error) {
	now := time.Now()
	dailyKey := now.Format("2006-01-02")
	year, week := now.ISOWeek()
	weeklyKey := aiRecordFormatWeekKey(year, week)
	monthlyKey := now.Format("2006-01")

	type countResult struct {
		DailyCount   int64
		WeeklyCount  int64
		MonthlyCount int64
	}

	var result countResult
	err = r.db.WithContext(ctx).Raw(`
		SELECT
			(SELECT COUNT(*) FROM ai_usage_records WHERE tenant_id = ? AND user_id = ? AND feature_type = ? AND daily_key   = ? AND success = true) AS daily_count,
			(SELECT COUNT(*) FROM ai_usage_records WHERE tenant_id = ? AND user_id = ? AND feature_type = ? AND weekly_key  = ? AND success = true) AS weekly_count,
			(SELECT COUNT(*) FROM ai_usage_records WHERE tenant_id = ? AND user_id = ? AND feature_type = ? AND monthly_key = ? AND success = true) AS monthly_count
	`,
		tenantID, userID, feature, dailyKey,
		tenantID, userID, feature, weeklyKey,
		tenantID, userID, feature, monthlyKey,
	).Scan(&result).Error

	return int(result.DailyCount), int(result.WeeklyCount), int(result.MonthlyCount), err
}

// GetUserStats returns aggregated usage statistics for a user within a time range.
func (r *AIUsageRecordRepo) GetUserStats(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, start, end time.Time) (*model.AIUsageStats, error) {
	stats := &model.AIUsageStats{
		TenantID:       tenantID,
		UserID:         userID,
		PeriodStart:    start,
		PeriodEnd:      end,
		UsageByFeature: make(map[model.AIFeatureType]int),
	}

	type featureCount struct {
		FeatureType model.AIFeatureType
		Count       int64
		Success     int64
		Tokens      int64
	}

	var counts []featureCount
	err := r.db.WithContext(ctx).Model(&model.AIUsageRecord{}).
		Select("feature_type, COUNT(*) as count, SUM(CASE WHEN success THEN 1 ELSE 0 END) as success, COALESCE(SUM(tokens_used), 0) as tokens").
		Where("tenant_id = ? AND user_id = ? AND used_at >= ? AND used_at < ?", tenantID, userID, start, end).
		Group("feature_type").
		Scan(&counts).Error
	if err != nil {
		return nil, err
	}

	for _, c := range counts {
		stats.UsageByFeature[c.FeatureType] = int(c.Count)
		stats.TotalCalls += int(c.Count)
		stats.SuccessfulCalls += int(c.Success)
		stats.TotalTokens += int(c.Tokens)
	}
	stats.FailedCalls = stats.TotalCalls - stats.SuccessfulCalls

	return stats, nil
}

// DeleteOlderThan removes usage records older than the given time (for data retention).
func (r *AIUsageRecordRepo) DeleteOlderThan(ctx context.Context, tenantID uuid.UUID, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND used_at < ?", tenantID, before).
		Delete(&model.AIUsageRecord{})
	return result.RowsAffected, result.Error
}

// aiRecordFormatWeekKey formats year and week number into a key (e.g. "2026-W05").
func aiRecordFormatWeekKey(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}
