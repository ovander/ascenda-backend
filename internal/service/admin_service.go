package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	"ascenda/internal/repo"
	"github.com/sirupsen/logrus"
)

// ─── DTO types ────────────────────────────────────────────────────────────────

// AdminStats is the full response payload for GET /api/v1/admin/stats.
// All figures are platform-wide (across all tenants).
type AdminStats struct {
	Tenants        int64              `json:"tenants"`
	Users          AdminUserStats     `json:"users"`
	Plans          AdminPlanStats     `json:"plans"`
	Scenarios      AdminScenarioStats `json:"scenarios"`
	RecentActivity []ActivityItem     `json:"recentActivity"`
	TopUsers       []TopUserItem      `json:"topUsers"`
}

// AdminUserStats holds user-level aggregate counts.
type AdminUserStats struct {
	Total    int64            `json:"total"`
	Active   int64            `json:"active"`
	Inactive int64            `json:"inactive"`
	ByRole   map[string]int64 `json:"byRole"`
	ByPlan   map[string]int64 `json:"byPlan"`
}

// AdminPlanStats holds plan-level aggregate counts.
type AdminPlanStats struct {
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"byStatus"`
}

// AdminScenarioStats holds scenario-level aggregate counts.
type AdminScenarioStats struct {
	Total int64 `json:"total"`
}

// ActivityItem is one entry in the recent-activity list.
type ActivityItem struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Action string `json:"action"`
	Actor  string `json:"actor"`
	At     string `json:"at"`
}

// TopUserItem is one row in the per-user stats table.
type TopUserItem struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	TenantID      string `json:"tenantId"`
	PlanCount     int64  `json:"planCount"`
	ScenarioCount int64  `json:"scenarioCount"`
}

// ─── AI Usage DTO types ───────────────────────────────────────────────────────

// AdminAIUsageStats is the full response payload for GET /api/v1/admin/ai-usage.
type AdminAIUsageStats struct {
	PeriodStart string               `json:"periodStart"`
	PeriodEnd   string               `json:"periodEnd"`
	CallsToday  int64                `json:"callsToday"`
	CallsWeek   int64                `json:"callsWeek"`
	CallsMonth  int64                `json:"callsMonth"`
	ByFeature   []AIUsageFeatureItem `json:"byFeature"`
	ByTenant    []AIUsageTenantItem  `json:"byTenant"`
}

// AIUsageFeatureItem is one row in the per-feature breakdown.
type AIUsageFeatureItem struct {
	Feature            string  `json:"feature"`
	TotalCalls         int64   `json:"totalCalls"`
	SuccessfulCalls    int64   `json:"successfulCalls"`
	TotalTokens        int64   `json:"totalTokens"`
	EstimatedCostCents float64 `json:"estimatedCostCents"`
}

// AIUsageTenantItem is one row in the per-tenant breakdown.
type AIUsageTenantItem struct {
	TenantID           string  `json:"tenantId"`
	TotalCalls         int64   `json:"totalCalls"`
	SuccessfulCalls    int64   `json:"successfulCalls"`
	TotalTokens        int64   `json:"totalTokens"`
	EstimatedCostCents float64 `json:"estimatedCostCents"`
}

// ─── Service ──────────────────────────────────────────────────────────────────

// AdminService assembles admin dashboard statistics.
type AdminService struct {
	statsRepo     *repo.AdminStatsRepo
	socrateClient SocrateUserManager // optional; nil if Socrate not configured
	logger        *logrus.Entry
}

// NewAdminService creates a new AdminService.
// socrateClient may be nil — when provided it is used to resolve display names
// for users whose name/email are missing from the local users table (e.g. dev
// environments where the JWT does not carry those claims).
func NewAdminService(statsRepo *repo.AdminStatsRepo, socrateClient SocrateUserManager, logger *logrus.Entry) *AdminService {
	return &AdminService{statsRepo: statsRepo, socrateClient: socrateClient, logger: logger}
}

// socrateDisplayName calls the Socrate API with the given externalID and returns
// the best available display name for the user.  Returns the original fallback
// unchanged when Socrate is not configured or the call fails.
func (s *AdminService) socrateDisplayName(ctx context.Context, externalID, fallback string) string {
	if s.socrateClient == nil || externalID == "" {
		return fallback
	}
	u, err := s.socrateClient.GetUser(ctx, externalID)
	if err != nil || u == nil {
		return fallback
	}
	name := strings.TrimSpace(u.Name)
	if name == "" {
		name = u.Email
	}
	if name == "" {
		return fallback
	}
	return name
}

// isNumericID returns true when s contains only ASCII digits — i.e. it looks
// like a raw Socrate user-ID rather than a real display name.
func isNumericID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// GetStats queries platform-wide aggregate data for the admin dashboard
// (sequential for simplicity — all are fast COUNT queries).
func (s *AdminService) GetStats(ctx context.Context) (*AdminStats, error) {
	// ── Tenants ───────────────────────────────────────────────────────────────
	totalTenants, err := s.statsRepo.CountTenants()
	if err != nil {
		return nil, err
	}

	// ── Users ────────────────────────────────────────────────────────────────
	roleCounts, err := s.statsRepo.CountUsersByRole()
	if err != nil {
		return nil, err
	}

	active, inactive, err := s.statsRepo.CountUsersByActivity()
	if err != nil {
		return nil, err
	}

	byRole := map[string]int64{"owner": 0, "admin": 0, "user": 0}
	var totalUsers int64
	for _, rc := range roleCounts {
		byRole[rc.Role] = rc.Count
		totalUsers += rc.Count
	}

	planCounts, err := s.statsRepo.CountUsersByPlan()
	if err != nil {
		return nil, err
	}
	byPlan := map[string]int64{"freemium": 0, "pro": 0, "enterprise": 0}
	for _, pc := range planCounts {
		byPlan[pc.Plan] = pc.Count
	}

	// ── Plans ─────────────────────────────────────────────────────────────────
	statusCounts, err := s.statsRepo.CountPlansByStatus()
	if err != nil {
		return nil, err
	}

	byStatus := map[string]int64{"draft": 0, "active": 0, "archived": 0}
	var totalPlans int64
	for _, sc := range statusCounts {
		byStatus[sc.Status] = sc.Count
		totalPlans += sc.Count
	}

	// ── Scenarios ─────────────────────────────────────────────────────────────
	totalScenarios, err := s.statsRepo.CountScenarios()
	if err != nil {
		return nil, err
	}

	// ── Recent activity ───────────────────────────────────────────────────────
	actRows, err := s.statsRepo.GetRecentActivity(10)
	if err != nil {
		return nil, err
	}

	recentActivity := make([]ActivityItem, 0, len(actRows))
	for _, row := range actRows {
		actor := row.UserEmail
		// When the SQL fell back to the numeric external_id, enrich via Socrate.
		if isNumericID(actor) {
			actor = s.socrateDisplayName(ctx, actor, actor)
		}
		recentActivity = append(recentActivity, ActivityItem{
			Type:   row.EntityType,
			Name:   row.EntityName,
			Action: row.Action,
			Actor:  actor,
			At:     row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	// ── Top users ─────────────────────────────────────────────────────────────
	topRows, err := s.statsRepo.GetTopUsers(10)
	if err != nil {
		return nil, err
	}

	topUsers := make([]TopUserItem, 0, len(topRows))
	for _, row := range topRows {
		name := row.Name
		// When the SQL fell back to the numeric external_id, enrich via Socrate.
		if isNumericID(name) {
			name = s.socrateDisplayName(ctx, name, name)
		}
		topUsers = append(topUsers, TopUserItem{
			Name:          name,
			Email:         row.Email,
			TenantID:      row.TenantID,
			PlanCount:     row.PlanCount,
			ScenarioCount: row.ScenarioCount,
		})
	}

	return &AdminStats{
		Tenants: totalTenants,
		Users: AdminUserStats{
			Total:    totalUsers,
			Active:   active,
			Inactive: inactive,
			ByRole:   byRole,
			ByPlan:   byPlan,
		},
		Plans: AdminPlanStats{
			Total:    totalPlans,
			ByStatus: byStatus,
		},
		Scenarios: AdminScenarioStats{
			Total: totalScenarios,
		},
		RecentActivity: recentActivity,
		TopUsers:       topUsers,
	}, nil
}

// GetAIUsageStats returns platform-wide AI usage statistics for the given time range.
// The caller supplies explicit start/end so the handler can support flexible windows
// (30-day default, weekly view, custom range, etc.).
func (s *AdminService) GetAIUsageStats(start, end time.Time) (*AdminAIUsageStats, error) {
	// ── Period call counts (today / this week / this month) ───────────────────
	now := time.Now().UTC()
	callsToday, callsWeek, callsMonth, err := s.statsRepo.CountPlatformAICalls(now)
	if err != nil {
		return nil, err
	}

	// ── By feature ────────────────────────────────────────────────────────────
	featureRows, err := s.statsRepo.GetPlatformAIUsageByFeature(start, end)
	if err != nil {
		return nil, err
	}

	byFeature := make([]AIUsageFeatureItem, 0, len(featureRows))
	for _, row := range featureRows {
		byFeature = append(byFeature, AIUsageFeatureItem{
			Feature:            row.FeatureType,
			TotalCalls:         row.TotalCalls,
			SuccessfulCalls:    row.SuccessfulCalls,
			TotalTokens:        row.TotalTokens,
			EstimatedCostCents: row.EstimatedCostCents,
		})
	}

	// ── By tenant ─────────────────────────────────────────────────────────────
	tenantRows, err := s.statsRepo.GetPlatformAIUsageByTenant(start, end)
	if err != nil {
		return nil, err
	}

	byTenant := make([]AIUsageTenantItem, 0, len(tenantRows))
	for _, row := range tenantRows {
		byTenant = append(byTenant, AIUsageTenantItem{
			TenantID:           row.TenantID,
			TotalCalls:         row.TotalCalls,
			SuccessfulCalls:    row.SuccessfulCalls,
			TotalTokens:        row.TotalTokens,
			EstimatedCostCents: row.EstimatedCostCents,
		})
	}

	return &AdminAIUsageStats{
		PeriodStart: start.UTC().Format(time.RFC3339),
		PeriodEnd:   end.UTC().Format(time.RFC3339),
		CallsToday:  callsToday,
		CallsWeek:   callsWeek,
		CallsMonth:  callsMonth,
		ByFeature:   byFeature,
		ByTenant:    byTenant,
	}, nil
}
