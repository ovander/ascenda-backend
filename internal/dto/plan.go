package dto

import (
	"kerplan/internal/model"
)

// PlanResponse is the API representation of a business plan.
type PlanResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedBy   string `json:"createdBy"`
	Timestamps
}

// PlanFromModel converts a model.BusinessPlan to a PlanResponse.
func PlanFromModel(p model.BusinessPlan) PlanResponse {
	return PlanResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		Status:      p.Status,
		CreatedBy:   p.CreatedBy.String(),
		Timestamps: Timestamps{
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		},
	}
}

// PlansFromModels converts a slice of model.BusinessPlan to PlanResponse DTOs.
func PlansFromModels(plans []model.BusinessPlan) []PlanResponse {
	out := make([]PlanResponse, len(plans))
	for i, p := range plans {
		out[i] = PlanFromModel(p)
	}
	return out
}

// ScenarioResponse is the API representation of a scenario.
type ScenarioResponse struct {
	ID        string `json:"id"`
	PlanID    string `json:"planId"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	Timestamps
}

// ScenarioFromModel converts a model.Scenario to a ScenarioResponse.
func ScenarioFromModel(s model.Scenario) ScenarioResponse {
	return ScenarioResponse{
		ID:        s.ID.String(),
		PlanID:    s.PlanID.String(),
		Name:      s.Name,
		IsDefault: s.IsDefault,
		Timestamps: Timestamps{
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		},
	}
}

// ScenariosFromModels converts a slice of model.Scenario to ScenarioResponse DTOs.
func ScenariosFromModels(scenarios []model.Scenario) []ScenarioResponse {
	out := make([]ScenarioResponse, len(scenarios))
	for i, s := range scenarios {
		out[i] = ScenarioFromModel(s)
	}
	return out
}
