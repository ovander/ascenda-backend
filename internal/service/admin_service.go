package service

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"ascenda/internal/repo"
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

// ─── Service ──────────────────────────────────────────────────────────────────

// AdminService assembles admin dashboard statistics.
type AdminService struct {
	statsRepo *repo.AdminStatsRepo
	logger    *logrus.Entry
}

// NewAdminService creates a new AdminService.
func NewAdminService(statsRepo *repo.AdminStatsRepo, logger *logrus.Entry) *AdminService {
	return &AdminService{statsRepo: statsRepo, logger: logger}
}

// GetStats queries platform-wide aggregate data for the admin dashboard
// (sequential for simplicity — all are fast COUNT queries).
func (s *AdminService) GetStats(_ context.Context) (*AdminStats, error) {
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
		recentActivity = append(recentActivity, ActivityItem{
			Type:   row.EntityType,
			Name:   row.EntityName,
			Action: row.Action,
			Actor:  row.UserEmail,
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
		topUsers = append(topUsers, TopUserItem{
			Name:          row.Name,
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
