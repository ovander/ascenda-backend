# Financial Planning SaaS — Project Structure

## 1. Domain Model Overview

```
Tenant (Organization)
 ├── Users (with roles)
 ├── Fiscal Years
 │    └── Periods (monthly/quarterly)
 ├── Chart of Accounts
 │    └── Accounts
 ├── Business Plans
 │    ├── Scenario (base, optimistic, pessimistic)
 │    ├── Sales Plan Lines
 │    ├── CAPEX Plan Lines
 │    ├── OPEX Plan Lines
 │    ├── Balance Sheet Entries
 │    └── Cash Flow (derived)
 └── Currencies
```

---

## 2. Data Models (Go structs / DB tables)

### 2.1 Multi-Tenancy & Users

```go
// tenant — the top-level isolation boundary
type Tenant struct {
    ID          uuid.UUID  `json:"id"          db:"id"`
    Name        string     `json:"name"        db:"name"`
    Slug        string     `json:"slug"        db:"slug"`         // subdomain or identifier
    LogoURL     string     `json:"logoUrl"     db:"logo_url"`
    IsActive    bool       `json:"isActive"    db:"is_active"`
    CreatedAt   time.Time  `json:"createdAt"   db:"created_at"`
    UpdatedAt   time.Time  `json:"updatedAt"   db:"updated_at"`
}

// user — linked to OAuth2 Socrate identity
type User struct {
    ID              uuid.UUID  `json:"id"              db:"id"`
    TenantID        uuid.UUID  `json:"tenantId"        db:"tenant_id"`
    ExternalID      string     `json:"-"               db:"external_id"`     // Socrate subject ID
    Email           string     `json:"email"           db:"email"`
    DisplayName     string     `json:"displayName"     db:"display_name"`
    AvatarURL       string     `json:"avatarUrl"       db:"avatar_url"`
    Role            UserRole   `json:"role"            db:"role"`            // owner | admin | editor | viewer
    IsActive        bool       `json:"isActive"        db:"is_active"`
    LastLoginAt     *time.Time `json:"lastLoginAt"     db:"last_login_at"`
    CreatedAt       time.Time  `json:"createdAt"       db:"created_at"`
    UpdatedAt       time.Time  `json:"updatedAt"       db:"updated_at"`
}

type UserRole string

const (
    RoleOwner  UserRole = "owner"
    RoleAdmin  UserRole = "admin"
    RoleEditor UserRole = "editor"
    RoleViewer UserRole = "viewer"
)
```

### 2.2 Fiscal Structure

```go
// fiscal_year — defines the company's financial year
type FiscalYear struct {
    ID         uuid.UUID  `json:"id"         db:"id"`
    TenantID   uuid.UUID  `json:"tenantId"   db:"tenant_id"`
    Label      string     `json:"label"      db:"label"`          // e.g. "FY2026"
    StartDate  time.Time  `json:"startDate"  db:"start_date"`
    EndDate    time.Time  `json:"endDate"    db:"end_date"`
    IsClosed   bool       `json:"isClosed"   db:"is_closed"`
    CreatedAt  time.Time  `json:"createdAt"  db:"created_at"`
    UpdatedAt  time.Time  `json:"updatedAt"  db:"updated_at"`
}

// period — monthly or quarterly breakdown within a fiscal year
type Period struct {
    ID           uuid.UUID   `json:"id"           db:"id"`
    TenantID     uuid.UUID   `json:"tenantId"     db:"tenant_id"`
    FiscalYearID uuid.UUID   `json:"fiscalYearId" db:"fiscal_year_id"`
    Label        string      `json:"label"        db:"label"`         // e.g. "Jan 2026", "Q1 2026"
    PeriodType   PeriodType  `json:"periodType"   db:"period_type"`   // monthly | quarterly
    StartDate    time.Time   `json:"startDate"    db:"start_date"`
    EndDate      time.Time   `json:"endDate"      db:"end_date"`
    SortOrder    int         `json:"sortOrder"    db:"sort_order"`
}

type PeriodType string

const (
    PeriodMonthly   PeriodType = "monthly"
    PeriodQuarterly PeriodType = "quarterly"
)
```

### 2.3 Chart of Accounts

```go
// account_category — top-level grouping
type AccountCategory string

const (
    CategoryRevenue        AccountCategory = "revenue"
    CategoryCOGS           AccountCategory = "cogs"
    CategoryOPEX           AccountCategory = "opex"
    CategoryCAPEX          AccountCategory = "capex"
    CategoryAsset          AccountCategory = "asset"
    CategoryLiability      AccountCategory = "liability"
    CategoryEquity         AccountCategory = "equity"
    CategoryCashFlow       AccountCategory = "cash_flow"
)

// account — a line in the chart of accounts
type Account struct {
    ID           uuid.UUID       `json:"id"           db:"id"`
    TenantID     uuid.UUID       `json:"tenantId"     db:"tenant_id"`
    ParentID     *uuid.UUID      `json:"parentId"     db:"parent_id"`      // for tree structure
    Code         string          `json:"code"         db:"code"`           // e.g. "6100"
    Name         string          `json:"name"         db:"name"`           // e.g. "Salaries & Wages"
    Category     AccountCategory `json:"category"     db:"category"`
    IsGroup      bool            `json:"isGroup"      db:"is_group"`       // parent node vs leaf
    SortOrder    int             `json:"sortOrder"    db:"sort_order"`
    IsActive     bool            `json:"isActive"     db:"is_active"`
    CreatedAt    time.Time       `json:"createdAt"    db:"created_at"`
    UpdatedAt    time.Time       `json:"updatedAt"    db:"updated_at"`
}
```

### 2.4 Business Plan & Scenarios

```go
// business_plan — a plan for a fiscal year
type BusinessPlan struct {
    ID           uuid.UUID        `json:"id"           db:"id"`
    TenantID     uuid.UUID        `json:"tenantId"     db:"tenant_id"`
    FiscalYearID uuid.UUID        `json:"fiscalYearId" db:"fiscal_year_id"`
    Name         string           `json:"name"         db:"name"`
    Description  string           `json:"description"  db:"description"`
    Status       PlanStatus       `json:"status"       db:"status"`
    CreatedBy    uuid.UUID        `json:"createdBy"    db:"created_by"`
    ApprovedBy   *uuid.UUID       `json:"approvedBy"   db:"approved_by"`
    ApprovedAt   *time.Time       `json:"approvedAt"   db:"approved_at"`
    CreatedAt    time.Time        `json:"createdAt"    db:"created_at"`
    UpdatedAt    time.Time        `json:"updatedAt"    db:"updated_at"`
}

type PlanStatus string

const (
    PlanDraft    PlanStatus = "draft"
    PlanReview   PlanStatus = "review"
    PlanApproved PlanStatus = "approved"
    PlanArchived PlanStatus = "archived"
)

// scenario — variants of a plan (base, optimistic, pessimistic)
type Scenario struct {
    ID             uuid.UUID    `json:"id"             db:"id"`
    TenantID       uuid.UUID    `json:"tenantId"       db:"tenant_id"`
    BusinessPlanID uuid.UUID    `json:"businessPlanId" db:"business_plan_id"`
    Name           string       `json:"name"           db:"name"`           // "Base", "Optimistic", "Pessimistic"
    IsDefault      bool         `json:"isDefault"      db:"is_default"`
    CreatedAt      time.Time    `json:"createdAt"      db:"created_at"`
    UpdatedAt      time.Time    `json:"updatedAt"      db:"updated_at"`
}
```

### 2.5 Financial Data Lines

All financial entries follow a common pattern: **an amount per account per period per scenario**.

```go
// plan_entry — the core financial data point
// One row = one account × one period × one scenario
type PlanEntry struct {
    ID           uuid.UUID  `json:"id"           db:"id"`
    TenantID     uuid.UUID  `json:"tenantId"     db:"tenant_id"`
    ScenarioID   uuid.UUID  `json:"scenarioId"   db:"scenario_id"`
    AccountID    uuid.UUID  `json:"accountId"    db:"account_id"`
    PeriodID     uuid.UUID  `json:"periodId"     db:"period_id"`
    Amount       float64    `json:"amount"       db:"amount"`          // use decimal in production
    Note         string     `json:"note"         db:"note"`
    UpdatedBy    uuid.UUID  `json:"updatedBy"    db:"updated_by"`
    CreatedAt    time.Time  `json:"createdAt"    db:"created_at"`
    UpdatedAt    time.Time  `json:"updatedAt"    db:"updated_at"`
}
```

### 2.6 Sales-Specific Detail (optional granularity)

```go
// sales_line — detailed revenue breakdown (product/service level)
type SalesLine struct {
    ID           uuid.UUID  `json:"id"           db:"id"`
    TenantID     uuid.UUID  `json:"tenantId"     db:"tenant_id"`
    ScenarioID   uuid.UUID  `json:"scenarioId"   db:"scenario_id"`
    PeriodID     uuid.UUID  `json:"periodId"     db:"period_id"`
    AccountID    uuid.UUID  `json:"accountId"    db:"account_id"`      // links to revenue account
    ProductName  string     `json:"productName"  db:"product_name"`
    Quantity     float64    `json:"quantity"      db:"quantity"`
    UnitPrice    float64    `json:"unitPrice"    db:"unit_price"`
    Discount     float64    `json:"discount"     db:"discount"`        // percentage
    TotalAmount  float64    `json:"totalAmount"  db:"total_amount"`    // computed
    Note         string     `json:"note"         db:"note"`
    CreatedAt    time.Time  `json:"createdAt"    db:"created_at"`
    UpdatedAt    time.Time  `json:"updatedAt"    db:"updated_at"`
}
```

### 2.7 CAPEX-Specific Detail

```go
// capex_item — capital expenditure with depreciation
type CapexItem struct {
    ID                  uuid.UUID  `json:"id"                  db:"id"`
    TenantID            uuid.UUID  `json:"tenantId"            db:"tenant_id"`
    ScenarioID          uuid.UUID  `json:"scenarioId"          db:"scenario_id"`
    AccountID           uuid.UUID  `json:"accountId"           db:"account_id"`
    Name                string     `json:"name"                db:"name"`
    Description         string     `json:"description"         db:"description"`
    AcquisitionPeriodID uuid.UUID  `json:"acquisitionPeriodId" db:"acquisition_period_id"`
    AcquisitionCost     float64    `json:"acquisitionCost"     db:"acquisition_cost"`
    UsefulLifeMonths    int        `json:"usefulLifeMonths"    db:"useful_life_months"`
    ResidualValue       float64    `json:"residualValue"       db:"residual_value"`
    DepreciationMethod  string     `json:"depreciationMethod"  db:"depreciation_method"` // straight_line | declining
    CreatedAt           time.Time  `json:"createdAt"           db:"created_at"`
    UpdatedAt           time.Time  `json:"updatedAt"           db:"updated_at"`
}
```

### 2.8 Currency

```go
type Currency struct {
    ID        uuid.UUID `json:"id"        db:"id"`
    TenantID  uuid.UUID `json:"tenantId"  db:"tenant_id"`
    Code      string    `json:"code"      db:"code"`       // ISO 4217: EUR, USD
    Name      string    `json:"name"      db:"name"`
    Symbol    string    `json:"symbol"    db:"symbol"`
    IsDefault bool      `json:"isDefault" db:"is_default"`
}
```

### 2.9 Audit Trail

```go
type AuditLog struct {
    ID         uuid.UUID       `json:"id"         db:"id"`
    TenantID   uuid.UUID       `json:"tenantId"   db:"tenant_id"`
    UserID     uuid.UUID       `json:"userId"      db:"user_id"`
    EntityType string          `json:"entityType"  db:"entity_type"`   // "plan_entry", "capex_item", etc.
    EntityID   uuid.UUID       `json:"entityId"    db:"entity_id"`
    Action     string          `json:"action"      db:"action"`        // create | update | delete
    OldValue   json.RawMessage `json:"oldValue"    db:"old_value"`
    NewValue   json.RawMessage `json:"newValue"    db:"new_value"`
    CreatedAt  time.Time       `json:"createdAt"   db:"created_at"`
}
```

---

## 3. Entity Relationship Diagram

```
┌──────────┐       ┌──────────┐       ┌──────────────┐
│  Tenant  │──1:N──│   User   │       │   Currency   │
└────┬─────┘       └──────────┘       └──────────────┘
     │
     ├──1:N──┌──────────────┐
     │       │  Fiscal Year │──1:N──┌──────────┐
     │       └──────────────┘       │  Period  │
     │                              └────┬─────┘
     ├──1:N──┌──────────────┐            │
     │       │   Account    │            │
     │       │ (tree/CoA)   │            │
     │       └──────┬───────┘            │
     │              │                    │
     ├──1:N──┌──────┴───────┐            │
     │       │BusinessPlan  │            │
     │       └──────┬───────┘            │
     │              │                    │
     │         1:N  │                    │
     │       ┌──────┴───────┐            │
     │       │   Scenario   │            │
     │       └──────┬───────┘            │
     │              │                    │
     │         ┌────┴────────────────────┤
     │         │                         │
     │    ┌────┴──────┐  ┌───────────┐  ┌┴──────────┐
     │    │ PlanEntry  │  │ SalesLine │  │ CapexItem │
     │    │(acct×per×  │  │(detailed  │  │(asset +   │
     │    │ scenario)  │  │ revenue)  │  │deprec.)   │
     │    └────────────┘  └───────────┘  └───────────┘
     │
     └──1:N──┌──────────────┐
             │  Audit Log   │
             └──────────────┘
```

---

## 4. Go Backend Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go                  # entry point, wire up dependencies
│
├── internal/
│   ├── config/
│   │   └── config.go                # env vars, DB DSN, OAuth2 settings
│   │
│   ├── middleware/
│   │   ├── auth.go                  # OAuth2/Socrate JWT validation
│   │   ├── tenant.go                # extract tenant from JWT/header, inject into ctx
│   │   ├── rbac.go                  # role-based access control
│   │   ├── logging.go               # structured request logging
│   │   └── cors.go                  # CORS for Vue.js frontend
│   │
│   ├── model/                       # pure data structs (no DB logic)
│   │   ├── tenant.go
│   │   ├── user.go
│   │   ├── fiscal_year.go
│   │   ├── period.go
│   │   ├── account.go
│   │   ├── business_plan.go
│   │   ├── scenario.go
│   │   ├── plan_entry.go
│   │   ├── sales_line.go
│   │   ├── capex_item.go
│   │   ├── currency.go
│   │   └── audit_log.go
│   │
│   ├── repo/                        # database access (SQL queries)
│   │   ├── tenant_repo.go
│   │   ├── user_repo.go
│   │   ├── fiscal_year_repo.go
│   │   ├── period_repo.go
│   │   ├── account_repo.go
│   │   ├── business_plan_repo.go
│   │   ├── scenario_repo.go
│   │   ├── plan_entry_repo.go
│   │   ├── sales_line_repo.go
│   │   ├── capex_item_repo.go
│   │   ├── currency_repo.go
│   │   └── audit_log_repo.go
│   │
│   ├── service/                     # business logic
│   │   ├── tenant_service.go
│   │   ├── user_service.go
│   │   ├── fiscal_year_service.go
│   │   ├── account_service.go
│   │   ├── business_plan_service.go
│   │   ├── scenario_service.go
│   │   ├── plan_entry_service.go    # includes aggregation logic
│   │   ├── sales_service.go
│   │   ├── capex_service.go         # depreciation calculations
│   │   ├── report_service.go        # P&L, balance sheet, cash flow generation
│   │   └── audit_service.go
│   │
│   ├── handler/                     # HTTP handlers (Chi routes)
│   │   ├── tenant_handler.go
│   │   ├── user_handler.go
│   │   ├── fiscal_year_handler.go
│   │   ├── account_handler.go
│   │   ├── business_plan_handler.go
│   │   ├── scenario_handler.go
│   │   ├── plan_entry_handler.go
│   │   ├── sales_handler.go
│   │   ├── capex_handler.go
│   │   ├── report_handler.go
│   │   └── auth_handler.go          # OAuth2 callback, token refresh
│   │
│   └── router/
│       └── router.go                # Chi router setup, mount all routes
│
├── pkg/
│   ├── database/
│   │   └── postgres.go              # connection pool, migrations runner
│   ├── auth/
│   │   └── socrate.go               # Socrate OAuth2 client, JWT parsing
│   └── response/
│       └── json.go                  # standard JSON response helpers
│
├── migrations/                      # SQL migration files
│   ├── 001_create_tenants.sql
│   ├── 002_create_users.sql
│   ├── 003_create_fiscal_years.sql
│   ├── 004_create_periods.sql
│   ├── 005_create_accounts.sql
│   ├── 006_create_business_plans.sql
│   ├── 007_create_scenarios.sql
│   ├── 008_create_plan_entries.sql
│   ├── 009_create_sales_lines.sql
│   ├── 010_create_capex_items.sql
│   ├── 011_create_currencies.sql
│   └── 012_create_audit_logs.sql
│
├── go.mod
├── go.sum
└── Makefile
```

---

## 5. API Routes

```go
// router.go — Chi route structure

r := chi.NewRouter()

// Global middleware
r.Use(middleware.Logger)
r.Use(middleware.CORS)
r.Use(middleware.Auth)         // validates Socrate JWT
r.Use(middleware.TenantCtx)    // extracts tenant_id from JWT claims

// Auth (public)
r.Group(func(r chi.Router) {
    r.Get("/auth/login",    authHandler.Login)       // redirect to Socrate
    r.Get("/auth/callback", authHandler.Callback)     // OAuth2 callback
    r.Post("/auth/refresh", authHandler.Refresh)
    r.Post("/auth/logout",  authHandler.Logout)
})

// Tenant admin
r.Route("/api/v1/tenant", func(r chi.Router) {
    r.Use(middleware.RequireRole(RoleOwner, RoleAdmin))
    r.Get("/",     tenantHandler.Get)
    r.Put("/",     tenantHandler.Update)
    r.Get("/users", userHandler.List)
    r.Post("/users", userHandler.Invite)
    r.Put("/users/{userID}", userHandler.UpdateRole)
    r.Delete("/users/{userID}", userHandler.Deactivate)
})

// Fiscal years & periods
r.Route("/api/v1/fiscal-years", func(r chi.Router) {
    r.Get("/",               fiscalYearHandler.List)
    r.Post("/",              fiscalYearHandler.Create)
    r.Get("/{fyID}",         fiscalYearHandler.Get)
    r.Put("/{fyID}",         fiscalYearHandler.Update)
    r.Get("/{fyID}/periods", periodHandler.List)
})

// Chart of accounts
r.Route("/api/v1/accounts", func(r chi.Router) {
    r.Get("/",            accountHandler.List)     // returns tree
    r.Post("/",           accountHandler.Create)
    r.Get("/{accountID}", accountHandler.Get)
    r.Put("/{accountID}", accountHandler.Update)
    r.Delete("/{accountID}", accountHandler.Delete)
})

// Business plans
r.Route("/api/v1/plans", func(r chi.Router) {
    r.Get("/",           planHandler.List)
    r.Post("/",          planHandler.Create)
    r.Get("/{planID}",   planHandler.Get)
    r.Put("/{planID}",   planHandler.Update)
    r.Delete("/{planID}", planHandler.Delete)

    // Scenarios within a plan
    r.Route("/{planID}/scenarios", func(r chi.Router) {
        r.Get("/",              scenarioHandler.List)
        r.Post("/",             scenarioHandler.Create)
        r.Get("/{scenarioID}",  scenarioHandler.Get)
        r.Put("/{scenarioID}",  scenarioHandler.Update)
        r.Delete("/{scenarioID}", scenarioHandler.Delete)

        // Plan entries within a scenario
        r.Route("/{scenarioID}/entries", func(r chi.Router) {
            r.Get("/",               entryHandler.List)    // filter by account, period
            r.Post("/bulk",          entryHandler.BulkUpsert)  // spreadsheet-style save
            r.Put("/{entryID}",      entryHandler.Update)
        })

        // Sales detail
        r.Route("/{scenarioID}/sales", func(r chi.Router) {
            r.Get("/",               salesHandler.List)
            r.Post("/",              salesHandler.Create)
            r.Put("/{lineID}",       salesHandler.Update)
            r.Delete("/{lineID}",    salesHandler.Delete)
        })

        // CAPEX detail
        r.Route("/{scenarioID}/capex", func(r chi.Router) {
            r.Get("/",               capexHandler.List)
            r.Post("/",              capexHandler.Create)
            r.Put("/{itemID}",       capexHandler.Update)
            r.Delete("/{itemID}",    capexHandler.Delete)
            r.Get("/{itemID}/depreciation", capexHandler.DepreciationSchedule)
        })
    })
})

// Reports (read-only, computed)
r.Route("/api/v1/reports", func(r chi.Router) {
    r.Get("/pnl",          reportHandler.ProfitAndLoss)     // ?planID=&scenarioID=
    r.Get("/balance-sheet", reportHandler.BalanceSheet)
    r.Get("/cash-flow",    reportHandler.CashFlow)
    r.Get("/comparison",   reportHandler.ScenarioComparison) // side-by-side
})

// Currencies
r.Route("/api/v1/currencies", func(r chi.Router) {
    r.Get("/",  currencyHandler.List)
    r.Post("/", currencyHandler.Create)
})
```

---

## 6. Vue.js Frontend Structure

```
frontend/
├── public/
├── src/
│   ├── main.ts
│   ├── App.vue
│   │
│   ├── api/                           # Axios/fetch wrappers
│   │   ├── client.ts                  # base client with auth interceptor
│   │   ├── tenantApi.ts
│   │   ├── userApi.ts
│   │   ├── fiscalYearApi.ts
│   │   ├── accountApi.ts
│   │   ├── planApi.ts
│   │   ├── scenarioApi.ts
│   │   ├── entryApi.ts
│   │   ├── salesApi.ts
│   │   ├── capexApi.ts
│   │   └── reportApi.ts
│   │
│   ├── composables/                   # Vue 3 composables
│   │   ├── useAuth.ts                 # Socrate OAuth2 flow, token storage
│   │   ├── useTenant.ts               # current tenant context
│   │   ├── useCurrentUser.ts
│   │   └── useNotification.ts
│   │
│   ├── stores/                        # Pinia stores
│   │   ├── authStore.ts
│   │   ├── tenantStore.ts
│   │   ├── fiscalYearStore.ts
│   │   ├── accountStore.ts
│   │   ├── planStore.ts
│   │   ├── scenarioStore.ts
│   │   └── entryStore.ts
│   │
│   ├── router/
│   │   └── index.ts                   # Vue Router with auth guards
│   │
│   ├── layouts/
│   │   ├── MainLayout.vue             # sidebar + topbar + content
│   │   └── AuthLayout.vue             # login/callback pages
│   │
│   ├── views/
│   │   ├── auth/
│   │   │   ├── LoginView.vue
│   │   │   └── CallbackView.vue
│   │   ├── dashboard/
│   │   │   └── DashboardView.vue      # overview KPIs
│   │   ├── plans/
│   │   │   ├── PlanListView.vue
│   │   │   ├── PlanDetailView.vue     # tabs for scenario data
│   │   │   └── PlanCompareView.vue    # scenario side-by-side
│   │   ├── sales/
│   │   │   └── SalesView.vue          # spreadsheet-style grid
│   │   ├── opex/
│   │   │   └── OpexView.vue           # spreadsheet-style grid
│   │   ├── capex/
│   │   │   ├── CapexView.vue          # asset list + depreciation
│   │   │   └── DepreciationView.vue
│   │   ├── balance-sheet/
│   │   │   └── BalanceSheetView.vue
│   │   ├── reports/
│   │   │   ├── PnlView.vue
│   │   │   ├── CashFlowView.vue
│   │   │   └── ComparisonView.vue
│   │   ├── accounts/
│   │   │   └── ChartOfAccountsView.vue  # PrimeVue TreeTable
│   │   ├── settings/
│   │   │   ├── TenantSettingsView.vue
│   │   │   ├── UserManagementView.vue
│   │   │   ├── FiscalYearView.vue
│   │   │   └── CurrencyView.vue
│   │   └── NotFoundView.vue
│   │
│   ├── components/
│   │   ├── common/
│   │   │   ├── AppSidebar.vue
│   │   │   ├── AppTopbar.vue
│   │   │   ├── ConfirmDialog.vue
│   │   │   └── LoadingSpinner.vue
│   │   ├── plan/
│   │   │   ├── PlanCard.vue
│   │   │   ├── ScenarioTabs.vue
│   │   │   └── StatusBadge.vue
│   │   ├── entries/
│   │   │   ├── EntryGrid.vue          # PrimeVue DataTable (editable)
│   │   │   ├── EntryCell.vue
│   │   │   └── PeriodColumns.vue
│   │   ├── sales/
│   │   │   └── SalesLineForm.vue
│   │   ├── capex/
│   │   │   ├── CapexForm.vue
│   │   │   └── DepreciationChart.vue
│   │   ├── reports/
│   │   │   ├── PnlTable.vue
│   │   │   ├── BalanceSheetTable.vue
│   │   │   └── ChartWidget.vue        # PrimeVue Chart
│   │   └── accounts/
│   │       └── AccountTree.vue         # PrimeVue TreeTable
│   │
│   └── types/                          # TypeScript interfaces (mirror Go models)
│       ├── tenant.ts
│       ├── user.ts
│       ├── fiscalYear.ts
│       ├── account.ts
│       ├── plan.ts
│       ├── scenario.ts
│       ├── planEntry.ts
│       ├── salesLine.ts
│       ├── capexItem.ts
│       └── report.ts
│
├── index.html
├── vite.config.ts
├── tailwind.config.js
├── tsconfig.json
└── package.json
```

---

## 7. Key Architectural Decisions

### 7.1 Tenant Isolation

**Strategy: shared database, `tenant_id` on every table.**

Every repo method receives the tenant ID from context (extracted by middleware from the JWT):

```go
// In every repo query:
func (r *PlanEntryRepo) List(ctx context.Context, scenarioID uuid.UUID) ([]model.PlanEntry, error) {
    tenantID := middleware.TenantFromCtx(ctx)
    query := `SELECT * FROM plan_entries WHERE tenant_id = $1 AND scenario_id = $2`
    // ...
}
```

A PostgreSQL Row-Level Security (RLS) policy provides a safety net:

```sql
ALTER TABLE plan_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON plan_entries
    USING (tenant_id = current_setting('app.current_tenant')::uuid);
```

### 7.2 Auth Flow (Socrate OAuth2)

```
Vue App                     Go Backend              Socrate
  │                            │                       │
  ├── GET /auth/login ────────►│                       │
  │                            ├── 302 redirect ──────►│
  │◄──────────────── redirect to Socrate login ────────┤
  │── user authenticates ─────────────────────────────►│
  │◄───────────── redirect to /auth/callback?code=X ───┤
  ├── GET /auth/callback ─────►│                       │
  │                            ├── POST /token ───────►│
  │                            │◄── {access, refresh} ─┤
  │                            ├── upsert user in DB   │
  │◄── Set HttpOnly cookies ───┤                       │
  │                            │                       │
  ├── GET /api/v1/plans ──────►│                       │
  │   (cookie: access_token)   ├── validate JWT        │
  │                            ├── extract tenant_id   │
  │◄── JSON response ──────────┤                       │
```

### 7.3 The PlanEntry Pattern

The `plan_entry` table is the **core data table** — it stores all numeric values in a normalized way. One row = one number at the intersection of:

- **Scenario** (which variant of the plan)
- **Account** (which line item — revenue, salary, rent, etc.)
- **Period** (which month/quarter)

This makes it straightforward to:
- Render spreadsheet-style grids (accounts as rows, periods as columns)
- Aggregate totals by category (sum all OPEX accounts)
- Compare scenarios side-by-side
- Generate P&L, balance sheet, and cash flow reports

### 7.4 Computed Reports

Reports (P&L, Balance Sheet, Cash Flow) are **not stored** — they are computed on the fly by `report_service.go` by aggregating `plan_entry` rows grouped by `account.category`. This keeps the data normalized and avoids sync issues.

---

## 8. Database Indexes (Critical for Performance)

```sql
-- Every table gets a tenant_id index
CREATE INDEX idx_plan_entries_tenant ON plan_entries(tenant_id);

-- The hot query path: loading a grid of entries
CREATE UNIQUE INDEX idx_plan_entries_unique
    ON plan_entries(tenant_id, scenario_id, account_id, period_id);

-- Scenario lookups
CREATE INDEX idx_plan_entries_scenario
    ON plan_entries(tenant_id, scenario_id);

-- Account tree queries
CREATE INDEX idx_accounts_parent
    ON accounts(tenant_id, parent_id);

-- Audit trail lookups
CREATE INDEX idx_audit_entity
    ON audit_logs(tenant_id, entity_type, entity_id);
```

---

## 9. Suggested Go Dependencies

| Purpose                | Package                              |
|------------------------|--------------------------------------|
| Router                 | `github.com/go-chi/chi/v5`          |
| Database               | `github.com/jackc/pgx/v5`           |
| Migrations             | `github.com/golang-migrate/migrate`  |
| UUID                   | `github.com/google/uuid`             |
| JWT validation         | `github.com/golang-jwt/jwt/v5`       |
| Structured logging     | `log/slog` (stdlib)                  |
| Env config             | `github.com/caarlos0/env/v11`        |
| Validation             | `github.com/go-playground/validator`  |
| Decimal (for money)    | `github.com/shopspring/decimal`       |
| Testing                | `github.com/stretchr/testify`         |
