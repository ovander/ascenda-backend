// Package service — AI usage policy enforcement.
// Ported from GPWA internal/service/ai_usage_policy_service.go and adapted for
// Ascenda (UUID IDs, Ascenda roles/features, no coach-player check).
package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"ascenda/internal/model"
	"ascenda/internal/repo"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ============================================================================
// AI Usage Policy Service
// ============================================================================

// AIUsagePolicyService enforces AI feature access based on user role and tier.
// It maintains an in-memory policy cache (TTL 5 min) to minimise DB round-trips.
type AIUsagePolicyService struct {
	policyRepo *repo.AIUsagePolicyRepo
	usageRepo  *repo.AIUsageRecordRepo
	log        *logrus.Entry

	// In-memory policy cache
	mu              sync.RWMutex
	policyCache     map[uuid.UUID]*model.AIUsagePolicyLookup
	policyCacheTime map[uuid.UUID]time.Time
	policyCacheTTL  time.Duration
}

// NewAIUsagePolicyService creates a new AIUsagePolicyService.
func NewAIUsagePolicyService(
	policyRepo *repo.AIUsagePolicyRepo,
	usageRepo *repo.AIUsageRecordRepo,
	log *logrus.Entry,
) *AIUsagePolicyService {
	return &AIUsagePolicyService{
		policyRepo:      policyRepo,
		usageRepo:       usageRepo,
		log:             log,
		policyCache:     make(map[uuid.UUID]*model.AIUsagePolicyLookup),
		policyCacheTime: make(map[uuid.UUID]time.Time),
		policyCacheTTL:  5 * time.Minute,
	}
}

// NewAIUsagePolicyServiceWithCache creates a service pre-loaded with a seeded
// policy lookup for a specific tenant.  The database repos are left nil so
// no real DB connection is required; the service will only work for the
// supplied tenantID and only for features whose policy has no quota limits
// (nil DailyLimit/WeeklyLimit/MonthlyLimit — the typical case for the
// default policy matrix).
//
// Intended for use in integration tests that exercise the middleware stack
// without spinning up a full database.
func NewAIUsagePolicyServiceWithCache(
	tenantID uuid.UUID,
	policies []model.AIUsagePolicy,
	log *logrus.Entry,
) *AIUsagePolicyService {
	svc := &AIUsagePolicyService{
		log:             log,
		policyCache:     make(map[uuid.UUID]*model.AIUsagePolicyLookup),
		policyCacheTime: make(map[uuid.UUID]time.Time),
		policyCacheTTL:  5 * time.Minute,
	}
	lookup := model.NewAIUsagePolicyLookup(policies)
	svc.policyCache[tenantID] = lookup
	svc.policyCacheTime[tenantID] = time.Now()
	return svc
}

// ============================================================================
// Access Control
// ============================================================================

// CheckAccess verifies whether a user can access an AI feature.
// Returns an AIAccessResult with the allowed/denied status and current quota usage.
func (s *AIUsagePolicyService) CheckAccess(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	role string,
	tier string,
	feature model.AIFeatureType,
) (*model.AIAccessResult, error) {
	result := &model.AIAccessResult{
		FeatureType: feature,
		UserRole:    role,
		Tier:        tier,
	}

	policy, err := s.getPolicy(ctx, tenantID, role, tier, feature)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Allowed = false
			result.DeniedReason = model.AIAccessDeniedTierNotAllowed
			result.DeniedMessage = model.GetDeniedMessage(result.DeniedReason, feature)
			return result, nil
		}
		return nil, err
	}

	if !policy.Allowed {
		result.Allowed = false
		result.DeniedReason = model.AIAccessDeniedTierNotAllowed
		result.DeniedMessage = model.GetDeniedMessage(result.DeniedReason, feature)
		return result, nil
	}

	// Short-circuit: if no limits are configured, skip the DB quota lookup.
	// This is the common case for admin users and unlimited tiers.
	if policy.DailyLimit == nil && policy.WeeklyLimit == nil && policy.MonthlyLimit == nil {
		result.Allowed = true
		return result, nil
	}

	// Get current quota usage
	daily, weekly, monthly, err := s.usageRepo.GetUsageCounts(ctx, tenantID, userID, feature)
	if err != nil {
		return nil, err
	}

	result.DailyUsed = daily
	result.DailyLimit = policy.DailyLimit
	result.WeeklyUsed = weekly
	result.WeeklyLimit = policy.WeeklyLimit
	result.MonthlyUsed = monthly
	result.MonthlyLimit = policy.MonthlyLimit

	if policy.DailyLimit != nil && daily >= *policy.DailyLimit {
		result.Allowed = false
		result.DeniedReason = model.AIAccessDeniedDailyLimitReached
		result.DeniedMessage = model.GetDeniedMessage(result.DeniedReason, feature)
		return result, nil
	}
	if policy.WeeklyLimit != nil && weekly >= *policy.WeeklyLimit {
		result.Allowed = false
		result.DeniedReason = model.AIAccessDeniedWeeklyLimitReached
		result.DeniedMessage = model.GetDeniedMessage(result.DeniedReason, feature)
		return result, nil
	}
	if policy.MonthlyLimit != nil && monthly >= *policy.MonthlyLimit {
		result.Allowed = false
		result.DeniedReason = model.AIAccessDeniedMonthlyLimitReached
		result.DeniedMessage = model.GetDeniedMessage(result.DeniedReason, feature)
		return result, nil
	}

	result.Allowed = true
	return result, nil
}

// EnforceAccess checks access and returns an AIAccessError if denied.
// Convenience method for service-layer enforcement.
func (s *AIUsagePolicyService) EnforceAccess(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	role string,
	tier string,
	feature model.AIFeatureType,
) error {
	result, err := s.CheckAccess(ctx, tenantID, userID, role, tier, feature)
	if err != nil {
		return err
	}
	if !result.Allowed {
		return newAIAccessError(result)
	}
	return nil
}

// ============================================================================
// Usage Recording
// ============================================================================

// RecordUsage persists a successful AI feature usage record.
func (s *AIUsagePolicyService) RecordUsage(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	feature model.AIFeatureType,
	userRole string,
	subscriptionTier string,
) error {
	record := model.NewAIUsageRecord(tenantID, userID, feature, userRole, subscriptionTier)
	return s.usageRepo.Create(ctx, tenantID, record)
}

// RecordUsageWithDetails persists a usage record with additional telemetry.
func (s *AIUsagePolicyService) RecordUsageWithDetails(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	feature model.AIFeatureType,
	userRole string,
	subscriptionTier string,
	requestID string,
	durationMs int,
	tokensUsed int,
	estimatedCostCents float64,
	success bool,
	errorMsg string,
) error {
	record := model.NewAIUsageRecord(tenantID, userID, feature, userRole, subscriptionTier)
	record.RequestID = requestID
	record.DurationMs = durationMs
	record.TokensUsed = tokensUsed
	record.EstimatedCostCents = estimatedCostCents
	record.Success = success
	record.ErrorMsg = errorMsg
	return s.usageRepo.Create(ctx, tenantID, record)
}

// CheckAndRecord performs an access check and records usage if allowed.
func (s *AIUsagePolicyService) CheckAndRecord(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	role string,
	tier string,
	feature model.AIFeatureType,
) (*model.AIAccessResult, error) {
	result, err := s.CheckAccess(ctx, tenantID, userID, role, tier, feature)
	if err != nil {
		return nil, err
	}
	if result.Allowed {
		if err := s.RecordUsage(ctx, tenantID, userID, feature, role, tier); err != nil {
			// Log but don't fail the request when recording fails.
			s.log.WithError(err).Warn("failed to record AI usage — quota may be inaccurate")
		}
	}
	return result, nil
}

// ============================================================================
// Usage Statistics
// ============================================================================

// GetUsageStats returns aggregated usage statistics for a user over a period.
func (s *AIUsagePolicyService) GetUsageStats(
	ctx context.Context,
	tenantID uuid.UUID,
	userID uuid.UUID,
	period string, // "daily", "weekly", "monthly"
) (*model.AIUsageStats, error) {
	start, end := periodBounds(period)
	stats, err := s.usageRepo.GetUserStats(ctx, tenantID, userID, start, end)
	if err != nil {
		return nil, err
	}
	stats.Period = period
	return stats, nil
}

// ============================================================================
// Policy Management
// ============================================================================

// InitializeDefaultPolicies seeds the DB with the default access matrix.
func (s *AIUsagePolicyService) InitializeDefaultPolicies(ctx context.Context, tenantID uuid.UUID) error {
	policies := model.DefaultAIUsagePolicies()
	return s.policyRepo.BulkUpsertPolicies(ctx, tenantID, policies)
}

// ListPolicies returns all policies for a tenant.
func (s *AIUsagePolicyService) ListPolicies(ctx context.Context, tenantID uuid.UUID) ([]model.AIUsagePolicy, error) {
	return s.policyRepo.ListPolicies(ctx, tenantID)
}

// UpdatePolicy updates a policy and invalidates the cache.
func (s *AIUsagePolicyService) UpdatePolicy(ctx context.Context, tenantID uuid.UUID, policy *model.AIUsagePolicy) error {
	s.invalidateCache(tenantID)
	return s.policyRepo.UpdatePolicy(ctx, tenantID, policy)
}

// ============================================================================
// Internal Cache
// ============================================================================

// getPolicy returns the cached policy for a (role, tier, feature) triple,
// refreshing from the DB when the cache is stale or missing.
func (s *AIUsagePolicyService) getPolicy(
	ctx context.Context,
	tenantID uuid.UUID,
	role, tier string,
	feature model.AIFeatureType,
) (*model.AIUsagePolicy, error) {
	s.mu.RLock()
	lookup, exists := s.policyCache[tenantID]
	cacheTime := s.policyCacheTime[tenantID]
	s.mu.RUnlock()

	if exists && time.Since(cacheTime) < s.policyCacheTTL {
		if p := lookup.GetPolicy(role, tier, feature); p != nil {
			return p, nil
		}
	}

	// Cache miss or stale — refresh from DB.
	dbPolicies, err := s.policyRepo.ListPolicies(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Fallback to built-in defaults when the DB has no policies yet.
	if len(dbPolicies) == 0 {
		dbPolicies = model.DefaultAIUsagePolicies()
	}

	lookup = model.NewAIUsagePolicyLookup(dbPolicies)
	s.mu.Lock()
	s.policyCache[tenantID] = lookup
	s.policyCacheTime[tenantID] = time.Now()
	s.mu.Unlock()

	p := lookup.GetPolicy(role, tier, feature)
	if p == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return p, nil
}

// invalidateCache removes the cached policies for a tenant.
func (s *AIUsagePolicyService) invalidateCache(tenantID uuid.UUID) {
	s.mu.Lock()
	delete(s.policyCache, tenantID)
	delete(s.policyCacheTime, tenantID)
	s.mu.Unlock()
}

// ============================================================================
// Period Helper
// ============================================================================

func periodBounds(period string) (start, end time.Time) {
	now := time.Now()
	switch period {
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 7)
	case "monthly":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0)
	default: // "daily"
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 1)
	}
	return
}

// ============================================================================
// Business Error
// ============================================================================

// AIAccessError represents an access-denied error returned by the policy service.
type AIAccessError struct {
	Feature      model.AIFeatureType
	Reason       model.AIAccessDeniedReason
	Message      string
	DailyUsed    int
	DailyLimit   *int
	WeeklyUsed   int
	WeeklyLimit  *int
	MonthlyUsed  int
	MonthlyLimit *int
}

func (e *AIAccessError) Error() string { return e.Message }

// HTTPStatusCode maps the denial reason to an appropriate HTTP status code.
//
//	402 — Payment Required (upgrade subscription)
//	403 — Forbidden (role has no access)
//	429 — Too Many Requests (quota exceeded)
//	503 — Service Unavailable (feature disabled / maintenance)
func (e *AIAccessError) HTTPStatusCode() int {
	switch e.Reason {
	case model.AIAccessDeniedTierNotAllowed:
		return 402
	case model.AIAccessDeniedRoleNotAllowed:
		return 403
	case model.AIAccessDeniedDailyLimitReached,
		model.AIAccessDeniedWeeklyLimitReached,
		model.AIAccessDeniedMonthlyLimitReached:
		return 429
	case model.AIAccessDeniedFeatureDisabled, model.AIAccessDeniedMaintenanceMode:
		return 503
	default:
		return 403
	}
}

// IsUpgradeRequired returns true when the denial requires a subscription upgrade.
func (e *AIAccessError) IsUpgradeRequired() bool {
	return e.Reason == model.AIAccessDeniedTierNotAllowed
}

// IsAIAccessError reports whether err is an AIAccessError.
func IsAIAccessError(err error) bool {
	var e *AIAccessError
	return errors.As(err, &e)
}

func newAIAccessError(result *model.AIAccessResult) *AIAccessError {
	return &AIAccessError{
		Feature:      result.FeatureType,
		Reason:       result.DeniedReason,
		Message:      result.DeniedMessage,
		DailyUsed:    result.DailyUsed,
		DailyLimit:   result.DailyLimit,
		WeeklyUsed:   result.WeeklyUsed,
		WeeklyLimit:  result.WeeklyLimit,
		MonthlyUsed:  result.MonthlyUsed,
		MonthlyLimit: result.MonthlyLimit,
	}
}
