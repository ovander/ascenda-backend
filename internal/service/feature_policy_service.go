package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"ascenda/internal/model"
	"github.com/ovander/backendkit/apierror"
	"ascenda/internal/repo"
)

// FeaturePolicyService manages feature-to-tier access rules.
// Policies are stored in the database and cached in-process for 5 minutes so
// that the hot read-path (every request that checks a limit) never hits the DB.
// The cache is invalidated immediately on every admin write.
type FeaturePolicyService struct {
	repo   repo.FeaturePolicyRepository
	logger *logrus.Entry

	mu          sync.RWMutex
	cached      []*model.FeaturePolicy
	cacheExpiry time.Time
	cacheTTL    time.Duration
}

const defaultCacheTTL = 5 * time.Minute

func NewFeaturePolicyService(r repo.FeaturePolicyRepository, logger *logrus.Entry) *FeaturePolicyService {
	return &FeaturePolicyService{
		repo:     r,
		logger:   logger,
		cacheTTL: defaultCacheTTL,
	}
}

// SeedDefaults inserts the platform's baseline rules on first boot.
// Called once during service-bundle initialisation.
func (s *FeaturePolicyService) SeedDefaults() error {
	defaults := model.DefaultFeaturePolicies()
	if err := s.repo.SeedDefaults(defaults); err != nil {
		return fmt.Errorf("feature policy seed: %w", err)
	}
	s.bust() // invalidate cache so the next read re-fetches the seeded rows
	s.logger.WithField("count", len(defaults)).Info("feature policies seeded")
	return nil
}

// List returns all feature policies (from cache if fresh).
func (s *FeaturePolicyService) List() ([]*model.FeaturePolicy, error) {
	return s.all()
}

// UpdatePolicy lets an admin change a single policy row.
// The in-process cache is invalidated immediately after the write.
func (s *FeaturePolicyService) UpdatePolicy(req UpdateFeaturePolicyRequest) (*model.FeaturePolicy, error) {
	existing, err := s.repo.GetByFeature(req.Feature)
	if err != nil {
		return nil, apierror.NotFound("feature_policy", req.Feature)
	}

	// Validate the incoming tier rules match the expected feature type.
	if err := validateTierRules(existing.FeatureType, req.Freemium, req.Pro, req.Enterprise); err != nil {
		return nil, apierror.BadRequest(err.Error())
	}

	existing.Freemium   = req.Freemium
	existing.Pro        = req.Pro
	existing.Enterprise = req.Enterprise

	if err := s.repo.Upsert(existing); err != nil {
		return nil, apierror.Internal("failed to save feature policy")
	}

	s.bust() // invalidate cache so the change takes effect immediately
	s.logger.WithFields(logrus.Fields{
		"feature":  req.Feature,
	}).Info("feature policy updated")

	return existing, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Access helpers — called by PlanService, middleware, handlers
// ──────────────────────────────────────────────────────────────────────────────

// IsAllowed returns true when the given commercial plan has access to the named
// feature.  Falls back to false (deny) on any error.
func (s *FeaturePolicyService) IsAllowed(feature, userPlan string) bool {
	rule, err := s.ruleFor(feature, userPlan)
	if err != nil || rule.Allowed == nil {
		return false
	}
	return *rule.Allowed
}

// NumericLimit returns the maximum count allowed for the named feature under
// the given commercial plan.  Returns -1 for unlimited, 0 when not found.
func (s *FeaturePolicyService) NumericLimit(feature, userPlan string) int {
	rule, err := s.ruleFor(feature, userPlan)
	if err != nil || rule.Limit == nil {
		return 0
	}
	return *rule.Limit
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

// ruleFor resolves the TierRule for (feature, plan) from the cached policies.
func (s *FeaturePolicyService) ruleFor(feature, userPlan string) (model.TierRule, error) {
	policies, err := s.all()
	if err != nil {
		return model.TierRule{}, err
	}

	for _, p := range policies {
		if p.Feature != feature {
			continue
		}
		var raw json.RawMessage
		switch normalisePlan(userPlan) {
		case "pro":
			raw = p.Pro
		case "enterprise":
			raw = p.Enterprise
		default: // freemium / unknown
			raw = p.Freemium
		}
		var rule model.TierRule
		if err := json.Unmarshal(raw, &rule); err != nil {
			return model.TierRule{}, fmt.Errorf("unmarshal rule: %w", err)
		}
		return rule, nil
	}
	return model.TierRule{}, fmt.Errorf("feature policy %q not found", feature)
}

// all returns cached policies, refreshing from DB when the cache is stale.
func (s *FeaturePolicyService) all() ([]*model.FeaturePolicy, error) {
	s.mu.RLock()
	if time.Now().Before(s.cacheExpiry) && s.cached != nil {
		defer s.mu.RUnlock()
		return s.cached, nil
	}
	s.mu.RUnlock()

	// Cache miss — refresh under write-lock.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write-lock.
	if time.Now().Before(s.cacheExpiry) && s.cached != nil {
		return s.cached, nil
	}

	policies, err := s.repo.List()
	if err != nil {
		s.logger.WithError(err).Error("failed to refresh feature policy cache")
		// If we have a stale cache, return it rather than failing hard.
		if s.cached != nil {
			return s.cached, nil
		}
		return nil, apierror.Internal("feature policies unavailable")
	}

	s.cached = policies
	s.cacheExpiry = time.Now().Add(s.cacheTTL)
	return policies, nil
}

// bust clears the cache so the next call re-fetches from the DB.
func (s *FeaturePolicyService) bust() {
	s.mu.Lock()
	s.cached = nil
	s.cacheExpiry = time.Time{}
	s.mu.Unlock()
}

// normalisePlan maps raw user plan strings to the three canonical tiers.
func normalisePlan(plan string) string {
	switch plan {
	case "pro":
		return "pro"
	case "enterprise":
		return "enterprise"
	default:
		return "freemium"
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Request / validation types
// ──────────────────────────────────────────────────────────────────────────────

// UpdateFeaturePolicyRequest is the body for PUT /admin/feature-policies/:feature.
type UpdateFeaturePolicyRequest struct {
	Feature    string          `json:"feature"`
	Freemium   json.RawMessage `json:"freemium"`
	Pro        json.RawMessage `json:"pro"`
	Enterprise json.RawMessage `json:"enterprise"`
}

// validateTierRules checks that each provided rule contains the correct fields
// for the given feature type.
func validateTierRules(ft model.FeatureType, tiers ...json.RawMessage) error {
	for _, raw := range tiers {
		var rule model.TierRule
		if err := json.Unmarshal(raw, &rule); err != nil {
			return fmt.Errorf("invalid tier rule JSON: %w", err)
		}
		switch ft {
		case model.FeatureTypeAccess:
			if rule.Allowed == nil {
				return fmt.Errorf("access feature requires {\"allowed\": bool}")
			}
		case model.FeatureTypeNumericLimit:
			if rule.Limit == nil {
				return fmt.Errorf("numeric_limit feature requires {\"limit\": int}")
			}
		}
	}
	return nil
}
