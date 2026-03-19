package model

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/google/uuid"
)

// PlanSnapshot represents a versioned snapshot of a scenario's complete data state.
type PlanSnapshot struct {
	TenantScoped
	ScenarioID  uuid.UUID       `gorm:"type:uuid;not null;index" json:"scenarioId"`
	Version     int             `gorm:"not null" json:"version"`
	Label       string          `gorm:"type:varchar(255)" json:"label"`
	Description string          `gorm:"type:text" json:"description"`
	CreatedBy   uuid.UUID       `gorm:"type:uuid;not null" json:"createdBy"`
	Reason      string          `gorm:"type:text" json:"reason"`
	Data        json.RawMessage `gorm:"type:jsonb;not null" json:"data"`
	ReportHash  string          `gorm:"type:varchar(64)" json:"reportHash"`
}

// TableName specifies the table name for PlanSnapshot.
func (PlanSnapshot) TableName() string {
	return "plan_snapshots"
}

// SnapshotData contains all serialized scenario data for a snapshot.
type SnapshotData struct {
	Config                    json.RawMessage `json:"config"`
	OpeningBalance            json.RawMessage `json:"openingBalance"`
	WCConfig                  json.RawMessage `json:"wcConfig"`
	Products                  json.RawMessage `json:"products"`
	ProductAssumptions        json.RawMessage `json:"productAssumptions"`
	ProductSalesVolumes       json.RawMessage `json:"productSalesVolumes"`
	ProductDistributorMargins json.RawMessage `json:"productDistributorMargins"`
	StaffHeadcounts           json.RawMessage `json:"staffHeadcounts"`
	StaffSalaries             json.RawMessage `json:"staffSalaries"`
	StaffIncentives           json.RawMessage `json:"staffIncentives"`
	CapexEntries              json.RawMessage `json:"capexEntries"`
	OpexEntries               json.RawMessage `json:"opexEntries"`
	PnlEntries                json.RawMessage `json:"pnlEntries"`
	FiplanEntries             json.RawMessage `json:"fiplanEntries"`
	PnlCashEntries            json.RawMessage `json:"pnlCashEntries"`
	WCREntries                json.RawMessage `json:"wcrEntries"`
	CashOverrides             json.RawMessage `json:"cashOverrides"`
	BudgetOverrides           json.RawMessage `json:"budgetOverrides"`
}

// Scan implements the sql.Scanner interface for SnapshotData.
func (sd *SnapshotData) Scan(value interface{}) error {
	bytes := value.([]byte)
	return json.Unmarshal(bytes, &sd)
}

// Value implements the driver.Valuer interface for SnapshotData.
func (sd SnapshotData) Value() (driver.Value, error) {
	return json.Marshal(sd)
}
