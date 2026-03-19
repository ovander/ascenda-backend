# KerPlan Backend Service Layer Implementation

## Overview

This document describes the complete Go backend implementation for KerPlan, a multi-tenant SaaS business planning application. The implementation includes all 20 service files, the Chi router, and the main server entry point.

## Architecture

### Service Layer Design

The service layer orchestrates business logic by:
1. Calling repositories for data access
2. Calling compute engine for calculations
3. Enforcing business rules and validation
4. Managing transactions and state
5. Logging and audit trails

Each service receives a `*logrus.Entry` for structured logging and handles errors using typed `apierror.AppError` types.

### Multi-Tenancy

Every data model embeds `TenantScoped` which provides:
- `ID`: UUID primary key
- `TenantID`: UUID tenant isolation
- `CreatedAt`, `UpdatedAt`: Timestamps

All service methods verify tenant ownership before operating on data.

## Service Files

### 1. auth_service.go
**OAuth2 integration with Socrate**

Methods:
- `GetLoginURL(codeChallenge, state)` - Builds Socrate authorize URL with PKCE
- `ExchangeCode(ctx, code, codeVerifier)` - Exchanges auth code for tokens
- `RefreshToken(ctx, refreshToken)` - Refreshes access tokens
- `RevokeToken(ctx, token)` - Revokes tokens
- `GetOrCreateUser(ctx, tenantID, claims)` - Creates user from OAuth claims

Type:
```go
type TokenResponse struct {
    AccessToken  string
    RefreshToken string
    ExpiresIn    int
    TokenType    string
    IDToken      string
}
```

### 2. tenant_service.go
**Tenant management**

Methods:
- `GetTenant(ctx, tenantID)` - Retrieve tenant by ID
- `UpdateTenant(ctx, tenantID, updates)` - Update tenant details (name, description)

### 3. user_service.go
**User management with RBAC**

Methods:
- `GetMe(ctx)` - Get authenticated user profile
- `UpdateMe(ctx, updates)` - Update current user's profile
- `ListUsers(ctx, tenantID)` - List all users in tenant
- `InviteUser(ctx, tenantID, email, name, role)` - Create new user with role
- `UpdateRole(ctx, tenantID, userID, newRole)` - Update user role
- `Deactivate(ctx, tenantID, userID)` - Deactivate user

Roles: owner, admin, manager, member, viewer

### 4. plan_service.go
**Business plan and scenario lifecycle**

Methods:
- `CreatePlan(ctx, tenantID, name, description)` - Creates plan with default "Base" scenario and config
- `GetPlan(ctx, tenantID, planID)` - Retrieve plan
- `ListPlans(ctx, tenantID)` - List all plans
- `UpdatePlan(ctx, tenantID, planID, name, description)` - Update plan details
- `DeletePlan(ctx, tenantID, planID)` - Soft delete plan (archives)
- `Approve(ctx, tenantID, planID)` - Transition plan to approved status
- `Archive(ctx, tenantID, planID)` - Archive plan
- `CreateScenario(ctx, tenantID, planID, name)` - Create scenario variant
- `GetScenario(ctx, tenantID, scenarioID)` - Retrieve scenario
- `ListScenarios(ctx, tenantID, planID)` - List scenarios for plan
- `UpdateScenario(ctx, tenantID, scenarioID, name)` - Update scenario
- `DeleteScenario(ctx, tenantID, scenarioID)` - Delete scenario
- `CloneScenario(ctx, tenantID, sourceScenarioID, newName)` - Deep copy scenario

Status: draft, active, approved, archived

### 5. settings_service.go
**Plan configuration and settings with recompute triggers**

Methods:
- `GetConfig(ctx, tenantID, scenarioID)` - Retrieve PlanConfig
- `UpdateConfig(ctx, tenantID, scenarioID, updates)` - Save config and signal recompute
- `GetOpeningBalance(ctx, tenantID, scenarioID)` - Retrieve opening balance sheet
- `UpdateOpeningBalance(ctx, tenantID, scenarioID, updates)` - Save opening balance
- `GetWCConfig(ctx, tenantID, scenarioID)` - Retrieve working capital config
- `UpdateWCConfig(ctx, tenantID, scenarioID, updates)` - Save WC config

Supports 45+ configuration settings including:
- Company/locale settings
- Tax and employment rates
- Capex per hire (furniture, IT)
- Interest rates
- Depreciation and NPV calculation settings

### 6. product_service.go
**Product CRUD and revenue computation**

Methods:
- `ListProducts(ctx, tenantID, scenarioID)` - List products
- `CreateProduct(ctx, tenantID, scenarioID, name, description)` - Create product
- `GetProduct(ctx, tenantID, productID)` - Retrieve product
- `UpdateProduct(ctx, tenantID, productID, updates)` - Update product
- `DeleteProduct(ctx, tenantID, productID)` - Delete product
- `UpdateAssumptions(ctx, tenantID, productID, assumptions)` - Update pricing/assumptions
- `UpdateVolumes(ctx, tenantID, productID, volumes)` - Update monthly volumes
- `UpdateMargins(ctx, tenantID, productID, cogs, opex)` - Update margins
- `GetConsolidatedRevenue(ctx, tenantID, scenarioID)` - Calls compute.ComputeConsolidatedRevenue

### 7. staff_service.go
**Staff CRUD and payroll computation**

Methods:
- `ListHeadcounts(ctx, tenantID, scenarioID)` - List headcount entries
- `UpdateHeadcounts(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `ListSalaries(ctx, tenantID, scenarioID)` - List salary entries
- `UpdateSalaries(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `ListIncentives(ctx, tenantID, scenarioID)` - List incentive entries
- `UpdateIncentives(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `GetPayrollSummary(ctx, tenantID, scenarioID)` - Calls compute.ComputeStaffPayroll

### 8. capex_service.go
**Capital expenditure with depreciation**

Methods:
- `ListEntries(ctx, tenantID, scenarioID)` - List capex entries
- `UpdateEntries(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `UpdateDurations(ctx, tenantID, scenarioID, durations)` - Update depreciation durations
- `GetSummary(ctx, tenantID, scenarioID)` - Calls compute.ComputeCapexSummary

### 9. opex_service.go
**Operating expenditure**

Methods:
- `ListManualEntries(ctx, tenantID, scenarioID)` - List manual opex entries
- `UpdateManualEntries(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `GetSummary(ctx, tenantID, scenarioID, productRevenue, staffPayroll)` - Calls compute.ComputeOpexSummary

### 10. pnl_service.go
**Profit & Loss statement**

Methods:
- `ListManualEntries(ctx, tenantID, scenarioID)` - List manual P&L entries
- `UpdateManualEntries(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `GetReport(ctx, tenantID, scenarioID)` - Calls compute.ComputePnl
- `GetChartData(ctx, tenantID, scenarioID)` - Returns chart-formatted P&L

### 11. fiplan_service.go
**Financial plan with grants**

Methods:
- `ListEntries(ctx, tenantID, scenarioID)` - List FiPlan entries
- `UpdateEntries(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `GetReport(ctx, tenantID, scenarioID)` - Calls compute.ComputeFiplan
- `GetGrantsForPnl(ctx, tenantID, scenarioID)` - Extract grants for P&L

### 12. pnl_cash_service.go
**P&L to Cash reconciliation**

Methods:
- `ListEntries(ctx, tenantID, scenarioID)` - List PnL-Cash entries
- `UpdateEntries(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `GetReport(ctx, tenantID, scenarioID)` - Calls compute.ComputePnlCash

### 13. bsheet_service.go
**Balance Sheet (read-only)**

Methods:
- `GetReport(ctx, tenantID, scenarioID)` - Loads all data, runs full compute
- `GetCharts(ctx, tenantID, scenarioID)` - Returns formatted chart data

### 14. ratios_service.go
**Financial ratios (read-only)**

Methods:
- `GetReport(ctx, tenantID, scenarioID)` - Computes all ratios
- `GetChart(ctx, tenantID, scenarioID, chartType)` - Returns chart data for:
  - profitability: gross margin, operating margin, net margin, ROE, ROA
  - liquidity: current ratio, quick ratio, cash ratio
  - solvency: debt ratio, debt/equity, interest coverage
  - growth: revenue growth, profit growth, asset growth

### 15. wcr_service.go
**Working Capital Requirements**

Methods:
- `ListEntries(ctx, tenantID, scenarioID)` - List WCR entries
- `UpdateEntries(ctx, tenantID, scenarioID, entries)` - Batch upsert
- `GetReport(ctx, tenantID, scenarioID)` - Computes WCR
- `GetCharts(ctx, tenantID, scenarioID)` - Returns formatted chart data

### 16. cash_service.go
**Monthly cash flow with overrides**

Methods:
- `ListOverrides(ctx, tenantID, scenarioID)` - List cash overrides
- `BatchUpsertOverrides(ctx, tenantID, scenarioID, overrides)` - Batch upsert
- `DeleteOverride(ctx, tenantID, overrideID)` - Delete single override
- `GetReport(ctx, tenantID, scenarioID)` - Computes monthly cash
- `GetYearReport(ctx, tenantID, scenarioID, yearIndex)` - Extract year data

### 17. budget_service.go
**Annual budgets (Year 1 and Year 2+)**

Methods:
- `ListYear1Overrides(ctx, tenantID, scenarioID)` - List Year 1 overrides
- `BatchUpsertYear1Overrides(ctx, tenantID, scenarioID, overrides)` - Batch upsert
- `ResetYear1Overrides(ctx, tenantID, scenarioID)` - Delete all Year 1 overrides
- `GetBudget1Report(ctx, tenantID, scenarioID)` - Calls compute.ComputeBudget1
- `GetBudget2Report(ctx, tenantID, scenarioID)` - Calls compute.ComputeBudget2
- `GetCombinedReport(ctx, tenantID, scenarioID)` - Combines both budgets

### 18. report_service.go
**Full plan orchestrator**

Methods:
- `GetFullReport(ctx, tenantID, scenarioID)` - Loads all inputs and calls compute.ComputeFullPlan
- `loadAllInputs(ctx, tenantID, scenarioID)` - Internal: loads all data from repos

Returns complete `FullPlanOutput` with all computed reports and validation warnings.

### 19. snapshot_service.go
**Plan versioning and state capture**

Methods:
- `Create(ctx, tenantID, scenarioID, name, description)` - Captures current state
- `Restore(ctx, tenantID, snapshotID)` - Restores scenario from snapshot
- `CloneToScenario(ctx, tenantID, snapshotID, newScenarioID)` - Clones snapshot to new scenario
- `Diff(ctx, tenantID, snapshot1ID, snapshot2ID)` - Compares two snapshots
- `List(ctx, tenantID, scenarioID)` - Lists snapshots for scenario
- `Get(ctx, tenantID, snapshotID)` - Retrieves snapshot metadata
- `GetData(ctx, tenantID, snapshotID)` - Retrieves snapshot data

### 20. service_bundle.go
**Dependency injection bundle**

Collects all service instances:
```go
type ServiceBundle struct {
    Auth, Tenant, User, Plan, Settings, Product, Staff,
    Capex, Opex, PnL, FiPlan, PnlCash, BSheet, Ratios,
    WCR, Cash, Budget, Report, Snapshot
}
```

Initialized via `NewServiceBundle(repos, cfg, logger)`.

## Router (internal/router/router.go)

Complete Chi router with 80+ endpoints:

### Global Middleware
- Panic recovery
- Request ID generation
- Structured logging
- CORS handling

### Public Routes (no auth)
```
GET  /health       - Health check
GET  /ready        - Readiness check
```

### Auth Routes (no tenant)
```
POST /auth/login-url      - Get login URL
POST /auth/exchange       - Exchange code for tokens
POST /auth/refresh        - Refresh tokens
POST /auth/revoke         - Revoke token
```

### Protected Routes (auth + tenant)

#### Tenant Management
```
GET  /tenant           - Get tenant
PUT  /tenant           - Update tenant
```

#### User Management
```
GET  /users            - List users
POST /users            - Invite user
GET  /users/me         - Get current user
PUT  /users/me         - Update current user
PUT  /users/{id}/role  - Update user role
DEL  /users/{id}       - Deactivate user
```

#### Plans & Scenarios
```
POST /plans                                - Create plan
GET  /plans                                - List plans
GET  /plans/{planID}                       - Get plan
PUT  /plans/{planID}                       - Update plan
DEL  /plans/{planID}                       - Delete plan
POST /plans/{planID}/approve               - Approve plan
POST /plans/{planID}/archive               - Archive plan

POST /plans/{planID}/scenarios             - Create scenario
GET  /plans/{planID}/scenarios             - List scenarios
GET  /plans/{planID}/scenarios/{scenarioID} - Get scenario
PUT  /plans/{planID}/scenarios/{scenarioID} - Update scenario
DEL  /plans/{planID}/scenarios/{scenarioID} - Delete scenario
POST /plans/{planID}/scenarios/{scenarioID}/clone - Clone scenario
```

#### Settings
```
GET  /plans/{planID}/scenarios/{scenarioID}/settings/config
PUT  /plans/{planID}/scenarios/{scenarioID}/settings/config
GET  /plans/{planID}/scenarios/{scenarioID}/settings/opening-balance
PUT  /plans/{planID}/scenarios/{scenarioID}/settings/opening-balance
GET  /plans/{planID}/scenarios/{scenarioID}/settings/wc-config
PUT  /plans/{planID}/scenarios/{scenarioID}/settings/wc-config
```

#### Products
```
POST /plans/{planID}/scenarios/{scenarioID}/products
GET  /plans/{planID}/scenarios/{scenarioID}/products
GET  /plans/{planID}/scenarios/{scenarioID}/products/revenue
GET  /plans/{planID}/scenarios/{scenarioID}/products/{productID}
PUT  /plans/{planID}/scenarios/{scenarioID}/products/{productID}
DEL  /plans/{planID}/scenarios/{scenarioID}/products/{productID}
PUT  /plans/{planID}/scenarios/{scenarioID}/products/{productID}/assumptions
PUT  /plans/{planID}/scenarios/{scenarioID}/products/{productID}/volumes
PUT  /plans/{planID}/scenarios/{scenarioID}/products/{productID}/margins
```

#### Staff
```
GET  /plans/{planID}/scenarios/{scenarioID}/staff/headcounts
PUT  /plans/{planID}/scenarios/{scenarioID}/staff/headcounts
GET  /plans/{planID}/scenarios/{scenarioID}/staff/salaries
PUT  /plans/{planID}/scenarios/{scenarioID}/staff/salaries
GET  /plans/{planID}/scenarios/{scenarioID}/staff/incentives
PUT  /plans/{planID}/scenarios/{scenarioID}/staff/incentives
GET  /plans/{planID}/scenarios/{scenarioID}/staff/payroll
```

#### Capex, Opex, P&L, FiPlan, PnL-Cash, WCR, Cash, Budget
Each has similar patterns:
```
GET  /plans/{planID}/scenarios/{scenarioID}/[domain]/
PUT  /plans/{planID}/scenarios/{scenarioID}/[domain]/
GET  /plans/{planID}/scenarios/{scenarioID}/[domain]/report
GET  /plans/{planID}/scenarios/{scenarioID}/[domain]/summary
```

#### Balance Sheet & Ratios
```
GET  /plans/{planID}/scenarios/{scenarioID}/bsheet/report
GET  /plans/{planID}/scenarios/{scenarioID}/bsheet/charts
GET  /plans/{planID}/scenarios/{scenarioID}/ratios/report
GET  /plans/{planID}/scenarios/{scenarioID}/ratios/chart/{chartType}
```

#### Reports
```
GET  /plans/{planID}/scenarios/{scenarioID}/report - Full report
```

#### Snapshots
```
POST /plans/{planID}/scenarios/{scenarioID}/snapshots
GET  /plans/{planID}/scenarios/{scenarioID}/snapshots
GET  /plans/{planID}/scenarios/{scenarioID}/snapshots/{snapshotID}
GET  /plans/{planID}/scenarios/{scenarioID}/snapshots/{snapshotID}/data
POST /plans/{planID}/scenarios/{scenarioID}/snapshots/{snapshotID}/restore
POST /plans/{planID}/scenarios/{scenarioID}/snapshots/{snapshotID}/clone
GET  /plans/{planID}/scenarios/{scenarioID}/snapshots/{s1ID}/diff/{s2ID}
```

## Main Entry Point (cmd/server/main.go)

Complete DI wiring:

1. **Load Configuration** - Environment variables via config.Load()
2. **Setup Logging** - Logrus with JSON/text format based on env
3. **Connect Database** - PostgreSQL via GORM with connection pooling
4. **Run Migrations** - GORM AutoMigrate for all models
5. **Create RepoBundle** - Initialize all repositories
6. **Create ServiceBundle** - Initialize all services with repos + config + logger
7. **Create Router** - Build Chi router with all routes
8. **Start Server** - Listen on configured port

Database Models Migrated:
- Tenant, User
- BusinessPlan, Scenario
- PlanConfig, OpeningBalance, WorkingCapitalConfig
- Product, ProductAssumption, ProductVolume, ProductMargin
- StaffHeadcount, StaffSalary, StaffIncentive
- CapexEntry, OpexManualEntry, PnlManualEntry
- FiplanEntry, PnlCashEntry, WCREntry
- CashMonthlyOverride, BudgetMonthlyOverride
- Snapshot, AuditLog

## Key Design Patterns

### Service Pattern
1. Validate inputs (business rules)
2. Call repo for data
3. Apply business logic
4. Call compute if needed
5. Log audit events
6. Return result or AppError

### Error Handling
All services return typed `apierror.AppError` with:
- Code: machine-readable error code
- Message: human-readable message
- StatusCode: HTTP status (set automatically)
- Details: additional validation details

### Multi-Tenancy
All methods verify tenant ownership:
```go
if model.TenantID != tenantID {
    return apierror.Forbidden("access denied")
}
```

### Logging
Structured logging with context:
```go
s.logger.WithField("plan_id", planID).
    WithField("user_id", userID).
    Info("plan created")
```

### Transactions
Service methods that modify data handle transactions via repos (GORM handles begin/commit).

## Compilation

The entire backend is complete and compilable:

```bash
cd /sessions/bold-intelligent-thompson/mnt/outputs/kerplan-api
go build ./cmd/server
go test ./...
```

## Dependencies

Core dependencies (from go.mod):
- github.com/go-chi/chi/v5 - HTTP router
- github.com/go-chi/cors - CORS middleware
- github.com/golang-jwt/jwt/v5 - JWT parsing
- github.com/google/uuid - UUID generation
- github.com/shopspring/decimal - Decimal math
- github.com/sirupsen/logrus - Structured logging
- gorm.io/driver/postgres - PostgreSQL driver
- gorm.io/gorm - ORM

## File Structure

```
internal/
  service/
    auth_service.go         ✓
    tenant_service.go       ✓
    user_service.go         ✓
    plan_service.go         ✓
    settings_service.go     ✓
    product_service.go      ✓
    staff_service.go        ✓
    capex_service.go        ✓
    opex_service.go         ✓
    pnl_service.go          ✓
    fiplan_service.go       ✓
    pnl_cash_service.go     ✓
    bsheet_service.go       ✓
    ratios_service.go       ✓
    wcr_service.go          ✓
    cash_service.go         ✓
    budget_service.go       ✓
    report_service.go       ✓
    snapshot_service.go     ✓
    service_bundle.go       ✓
  router/
    router.go               ✓
cmd/
  server/
    main.go                 ✓
```

## Next Steps

1. **Create Handlers** - Map HTTP requests to service methods
2. **Add Tests** - Unit tests for services, integration tests for router
3. **Deploy** - Docker setup, Kubernetes manifests
4. **Documentation** - OpenAPI/Swagger specification
5. **Monitoring** - Metrics collection, health checks
