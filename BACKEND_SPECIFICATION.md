# KerPlan — Backend Go Specification

## High-Level Architecture

KerPlan is a **multi-tenant, multi-user SaaS business plan application** built as a Go backend
that serves as the **source of truth and financial computation engine**. The backend replaces
Excel's cell-reference engine with server-side recalculation, storing only user inputs and
computing all derived values on the fly.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              CLIENTS                                         │
│  Vue.js SPA (PrimeVue)  ·  Mobile (future)  ·  PDF Export (future)          │
└────────────────────────────────┬─────────────────────────────────────────────┘
                                 │ HTTPS / JSON
                                 ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                           GO BACKEND (Chi)                                   │
│                                                                              │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐  ┌─────────────┐  │
│  │  Router   │→│ Middleware│→│  Handler   │→│  Service  │→│  Repository  │  │
│  │  (Chi)    │  │ (Auth,   │  │ (HTTP I/O) │  │ (Business │  │  (GORM)     │  │
│  │          │  │  Tenant,  │  │            │  │  Logic +  │  │             │  │
│  │          │  │  Logger)  │  │            │  │  Compute) │  │             │  │
│  └──────────┘  └──────────┘  └───────────┘  └──────────┘  └──────┬──────┘  │
│                                                                    │         │
│  ┌─────────────────────────────────────────────────────────────────┤         │
│  │              COMPUTE ENGINE (internal/compute/)                 │         │
│  │  Pure functions: no DB access, no side effects                  │         │
│  │  Input: model structs → Output: report structs                  │         │
│  └─────────────────────────────────────────────────────────────────┘         │
│                                                                              │
└──────────────────────────────────────────┬───────────────────────────────────┘
                                           │
                                           ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                        POSTGRESQL (+ RLS)                                    │
│  Shared database · tenant_id on every table · Row-Level Security policies   │
└──────────────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                       SOCRATE OAuth2 Server                                  │
│  JWT tokens · tenant_id claim · user identity · refresh flow                │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 1. Technology Stack

| Layer | Technology | Notes |
|---|---|---|
| Language | Go 1.22+ | Standard library + minimal deps |
| Router | `go-chi/chi/v5` | Lightweight, idiomatic, middleware-friendly |
| ORM | `gorm.io/gorm` + `gorm.io/driver/postgres` | Struct-first, hooks, scopes for multi-tenancy |
| Decimal | `shopspring/decimal` | All monetary values — no float64 for money |
| UUID | `google/uuid` | Primary keys |
| Auth | Custom OAuth2 client → Socrate server | JWT validation, JWKS key rotation |
| Logger | `uber-go/zap` (structured) | Request-scoped via context |
| Config | `spf13/viper` or `caarlos0/env` | Env-based, 12-factor |
| Validation | `go-playground/validator/v10` | Struct tag validation |
| Migration | `golang-migrate/migrate` | SQL migration files |
| Testing | `stretchr/testify` + `DATA-DOG/go-sqlmock` | Unit + integration |

---

## 2. Project Structure

```
kerplan-api/
├── cmd/
│   └── server/
│       └── main.go                     # Entrypoint: config, DI, server start
│
├── internal/
│   ├── config/
│   │   └── config.go                   # Env-based configuration struct
│   │
│   ├── middleware/
│   │   ├── auth.go                     # JWT validation, extract claims
│   │   ├── tenant.go                   # Set tenant_id scope on DB + context
│   │   ├── logger.go                   # Request-scoped structured logging
│   │   ├── recover.go                  # Panic recovery
│   │   ├── cors.go                     # CORS headers
│   │   └── rbac.go                     # Role-based access (owner/admin/editor/viewer)
│   │
│   ├── model/                          # Domain structs — NO business logic
│   │   ├── tenant.go                   # Tenant, User, UserRole
│   │   ├── plan.go                     # BusinessPlan, Scenario, PlanStatus
│   │   ├── settings.go                 # PlanConfig, OpeningBalance, WorkingCapitalConfig
│   │   ├── product.go                  # Product, GeographicZone, ProductAssumption, etc.
│   │   ├── staff.go                    # StaffHeadcount, StaffSalary, StaffIncentive
│   │   ├── capex.go                    # CapexEntry, CapexCategory, AssetDuration
│   │   ├── opex.go                     # OpexManualEntry, OpexLineID, CostDriver
│   │   ├── pnl.go                      # PnlManualEntry, PnlReport, PnlAggregate
│   │   ├── fiplan.go                   # FiplanEntry, FiplanReport, FiplanPlan
│   │   ├── pnl_cash.go                # PnlCashEntry, PnlCashReport
│   │   ├── bsheet.go                   # BSheetReport (no input model — pure computed)
│   │   ├── ratios.go                   # RatiosReport (pure computed)
│   │   ├── wcr.go                      # WCREntry, WCRReport
│   │   ├── cash.go                     # CashMonthlyOverride, CashReport, CashYear
│   │   ├── budget.go                   # BudgetMonthlyOverride, Budget1Report, Budget2Report
│   │   ├── graph.go                    # Chart data structs (pure computed)
│   │   ├── audit.go                    # AuditLog
│   │   ├── snapshot.go                 # PlanSnapshot (versioning)
│   │   └── validation.go              # ValidationWarning, cross-check types
│   │
│   ├── repo/                           # Data access — GORM scoped queries
│   │   ├── base.go                     # BaseRepo with tenant scope helper
│   │   ├── tenant_repo.go
│   │   ├── user_repo.go
│   │   ├── plan_repo.go               # BusinessPlan + Scenario CRUD
│   │   ├── settings_repo.go           # PlanConfig, OpeningBalance, WCConfig
│   │   ├── product_repo.go            # Product + assumptions + volumes + margins
│   │   ├── staff_repo.go              # Headcount + salaries + incentives
│   │   ├── capex_repo.go
│   │   ├── opex_repo.go
│   │   ├── pnl_repo.go
│   │   ├── fiplan_repo.go
│   │   ├── pnl_cash_repo.go
│   │   ├── wcr_repo.go
│   │   ├── cash_repo.go
│   │   ├── budget_repo.go             # Shared table for Budget1 + Budget2 overrides
│   │   ├── snapshot_repo.go
│   │   └── audit_repo.go
│   │
│   ├── service/                        # Business logic — orchestrates repo + compute
│   │   ├── auth_service.go             # OAuth2 token exchange, Socrate integration
│   │   ├── tenant_service.go
│   │   ├── user_service.go
│   │   ├── plan_service.go             # CRUD + clone scenario + snapshot
│   │   ├── settings_service.go         # Config save + dependent recalc trigger
│   │   ├── product_service.go
│   │   ├── staff_service.go
│   │   ├── capex_service.go
│   │   ├── opex_service.go
│   │   ├── pnl_service.go
│   │   ├── fiplan_service.go
│   │   ├── pnl_cash_service.go
│   │   ├── bsheet_service.go
│   │   ├── ratios_service.go
│   │   ├── wcr_service.go
│   │   ├── cash_service.go
│   │   ├── budget_service.go           # Budget1 + Budget2
│   │   ├── report_service.go           # Orchestrator: full plan report (all sheets)
│   │   └── snapshot_service.go         # Version management
│   │
│   ├── compute/                        # PURE COMPUTATION — the financial engine
│   │   ├── engine.go                   # ComputeFullPlan orchestrator
│   │   ├── product.go                  # ComputeProductRevenue, ComputeConsolidatedRevenue
│   │   ├── staff.go                    # ComputeStaffPayroll
│   │   ├── capex.go                    # ComputeCapexSummary, ComputeSLNDepreciation
│   │   ├── opex.go                     # ComputeOpexSummary
│   │   ├── pnl.go                      # ComputePnl
│   │   ├── fiplan.go                   # ComputeFiplan, GetGrantsForPnl
│   │   ├── pnl_cash.go                # ComputePnlCash
│   │   ├── bsheet.go                   # ComputeBSheet
│   │   ├── ratios.go                   # ComputeRatios, computeNPV, computeIRR
│   │   ├── wcr.go                      # ComputeWCR, GetWCRForBSheet, GetWCRChangeForFiPlan
│   │   ├── cash.go                     # ComputeCash, distributeMonthly
│   │   ├── budget.go                   # ComputeBudget1, ComputeBudget2
│   │   ├── calibration.go             # Treasury convergence loop
│   │   └── helpers.go                  # sumYears, pctSafe, fiscal year adjustment
│   │
│   ├── handler/                        # HTTP handlers — thin, delegates to service
│   │   ├── auth_handler.go
│   │   ├── tenant_handler.go
│   │   ├── user_handler.go
│   │   ├── plan_handler.go
│   │   ├── scenario_handler.go
│   │   ├── settings_handler.go
│   │   ├── product_handler.go
│   │   ├── staff_handler.go
│   │   ├── capex_handler.go
│   │   ├── opex_handler.go
│   │   ├── pnl_handler.go
│   │   ├── fiplan_handler.go
│   │   ├── pnl_cash_handler.go
│   │   ├── bsheet_handler.go
│   │   ├── ratios_handler.go
│   │   ├── wcr_handler.go
│   │   ├── cash_handler.go
│   │   ├── budget_handler.go
│   │   ├── report_handler.go          # Full plan report
│   │   ├── snapshot_handler.go
│   │   └── response.go                # JSON response helpers, error formatting
│   │
│   ├── router/
│   │   └── router.go                  # Chi router setup, all route registration
│   │
│   └── pkg/                            # Shared utilities
│       ├── apierror/                   # Structured API error types
│       ├── ctxutil/                    # Context getters (tenant_id, user_id, logger)
│       ├── pagination/                 # Cursor/offset pagination helpers
│       └── decimal_util/              # Decimal helpers (sum, pctSafe, roundTo)
│
├── migrations/
│   ├── 000001_create_tenants.up.sql
│   ├── 000001_create_tenants.down.sql
│   ├── 000002_create_users.up.sql
│   ├── ...                             # Sequential migration files
│   └── 000020_create_budget_overrides.up.sql
│
├── docs/
│   └── api/
│       └── openapi.yaml                # OpenAPI 3.0 specification
│
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── .env.example
```

---

## 3. Multi-Tenancy Architecture

### 3.1 Strategy: Shared Database + GORM Scopes

All tables carry a `tenant_id` column. GORM scopes automatically filter every query.

```go
// internal/repo/base.go

package repo

import (
    "github.com/google/uuid"
    "gorm.io/gorm"
)

// TenantScope returns a GORM scope that filters by tenant_id.
// Applied automatically to ALL queries via middleware.
func TenantScope(tenantID uuid.UUID) func(db *gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("tenant_id = ?", tenantID)
    }
}

// BaseRepo provides tenant-scoped DB access.
type BaseRepo struct {
    db *gorm.DB
}

func NewBaseRepo(db *gorm.DB) BaseRepo {
    return BaseRepo{db: db}
}

// DB returns a tenant-scoped GORM session.
func (r *BaseRepo) DB(tenantID uuid.UUID) *gorm.DB {
    return r.db.Scopes(TenantScope(tenantID))
}
```

### 3.2 PostgreSQL Row-Level Security (Defense in Depth)

GORM scopes are the primary filter. RLS is a safety net ensuring no cross-tenant leakage
even if a query bypasses the scope.

```sql
-- Applied to every data table (example: plan_configs)
ALTER TABLE plan_configs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON plan_configs
    USING (tenant_id = current_setting('app.tenant_id')::UUID);

-- The tenant middleware sets this per-connection:
-- SET LOCAL app.tenant_id = '<uuid>';
```

### 3.3 Tenant Middleware

```go
// internal/middleware/tenant.go

package middleware

import (
    "context"
    "net/http"

    "github.com/google/uuid"
    "gorm.io/gorm"
    "go.uber.org/zap"

    "kerplan/internal/pkg/ctxutil"
    "kerplan/internal/pkg/apierror"
)

type TenantMiddleware struct {
    db     *gorm.DB
    logger *zap.Logger
}

func NewTenantMiddleware(db *gorm.DB, logger *zap.Logger) *TenantMiddleware {
    return &TenantMiddleware{db: db, logger: logger}
}

// Handler extracts tenant_id from JWT claims (set by auth middleware)
// and injects it into the request context + sets the PostgreSQL session variable.
func (m *TenantMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := ctxutil.GetClaims(r.Context())
        if claims == nil {
            apierror.Unauthorized(w, "missing auth claims")
            return
        }

        tenantID, err := uuid.Parse(claims.TenantID)
        if err != nil {
            apierror.Unauthorized(w, "invalid tenant_id in token")
            return
        }

        // Set PostgreSQL session variable for RLS
        m.db.Exec("SET LOCAL app.tenant_id = ?", tenantID.String())

        // Inject into context
        ctx := context.WithValue(r.Context(), ctxutil.TenantIDKey, tenantID)
        ctx = context.WithValue(ctx, ctxutil.UserIDKey, claims.UserID)
        ctx = context.WithValue(ctx, ctxutil.UserRoleKey, claims.Role)

        m.logger.Debug("tenant context set",
            zap.String("tenant_id", tenantID.String()),
            zap.String("user_id", claims.UserID),
        )

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## 4. Authentication: Socrate OAuth2 Integration

### 4.1 Flow

```
┌────────┐        ┌─────────────┐        ┌──────────────┐
│ Browser │  ───►  │ KerPlan API │  ───►  │ Socrate OAuth │
│ (Vue)   │  ◄───  │ (Go/Chi)    │  ◄───  │ Server       │
└────────┘        └─────────────┘        └──────────────┘

1. Vue redirects to Socrate /authorize (PKCE flow)
2. User authenticates on Socrate
3. Socrate redirects back with authorization code
4. Vue sends code to KerPlan API /auth/callback
5. KerPlan exchanges code for tokens with Socrate
6. KerPlan validates JWT, extracts claims (tenant_id, user_id, role)
7. KerPlan returns access_token + refresh_token to Vue
8. Vue stores tokens, sends access_token on every API request
```

### 4.2 JWT Claims Structure

```go
// internal/middleware/auth.go

// SocrateClaims represents the JWT claims from the Socrate OAuth2 server.
type SocrateClaims struct {
    jwt.RegisteredClaims

    TenantID string `json:"tenant_id"`  // UUID of the tenant
    UserID   string `json:"user_id"`    // UUID of the user
    Email    string `json:"email"`
    Name     string `json:"name"`
    Role     string `json:"role"`       // "owner" | "admin" | "editor" | "viewer"
}
```

### 4.3 Auth Middleware

```go
// internal/middleware/auth.go

type AuthMiddleware struct {
    jwksURL    string          // Socrate JWKS endpoint
    keySet     jwk.Set         // Cached JWKS keys
    logger     *zap.Logger
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract Bearer token
        tokenStr := extractBearerToken(r)
        if tokenStr == "" {
            apierror.Unauthorized(w, "missing bearer token")
            return
        }

        // Validate JWT signature against Socrate JWKS
        claims, err := m.validateToken(tokenStr)
        if err != nil {
            apierror.Unauthorized(w, "invalid token: "+err.Error())
            return
        }

        // Inject claims into context
        ctx := context.WithValue(r.Context(), ctxutil.ClaimsKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 4.4 Auth Endpoints

```go
// internal/handler/auth_handler.go

type AuthHandler struct {
    authService service.AuthService
    logger      *zap.Logger
}

// POST /auth/login — initiate OAuth2 PKCE flow, return redirect URL
// POST /auth/callback — exchange authorization code for tokens
// POST /auth/refresh — refresh access token using refresh token
// POST /auth/logout — revoke tokens on Socrate
```

### 4.5 Role-Based Access Control (RBAC)

```go
// internal/middleware/rbac.go

type Permission string

const (
    PermViewPlan   Permission = "view_plan"
    PermEditPlan   Permission = "edit_plan"
    PermManagePlan Permission = "manage_plan"   // create/delete/approve
    PermManageUsers Permission = "manage_users"  // invite/remove users
    PermManageTenant Permission = "manage_tenant" // billing, settings
)

// RolePermissions defines what each role can do.
var RolePermissions = map[model.UserRole][]Permission{
    model.RoleViewer: {PermViewPlan},
    model.RoleEditor: {PermViewPlan, PermEditPlan},
    model.RoleAdmin:  {PermViewPlan, PermEditPlan, PermManagePlan, PermManageUsers},
    model.RoleOwner:  {PermViewPlan, PermEditPlan, PermManagePlan, PermManageUsers, PermManageTenant},
}

// RequirePermission returns middleware that checks the user's role.
func RequirePermission(perm Permission) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            role := ctxutil.GetUserRole(r.Context())
            if !hasPermission(role, perm) {
                apierror.Forbidden(w, "insufficient permissions")
                return
            }
            next.ServeHTTP(w, r.WithContext(r.Context()))
        })
    }
}
```

---

## 5. Layered Architecture: The Request Lifecycle

```
HTTP Request
    │
    ▼
┌─ ROUTER (Chi) ─────────────────────────────────────────────────────────┐
│  Route matching, URL parameter extraction                               │
│  chi.URLParam(r, "planID"), chi.URLParam(r, "scenID")                  │
└────┬────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─ MIDDLEWARE CHAIN ──────────────────────────────────────────────────────┐
│  1. RecoverMiddleware    — panic recovery                               │
│  2. CORSMiddleware       — CORS headers                                 │
│  3. RequestIDMiddleware  — X-Request-ID                                  │
│  4. LoggerMiddleware     — structured request/response logging           │
│  5. AuthMiddleware       — JWT validation, extract claims                │
│  6. TenantMiddleware     — set tenant scope on DB + context              │
│  7. RBACMiddleware       — check role permissions (per route group)      │
└────┬────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─ HANDLER ───────────────────────────────────────────────────────────────┐
│  Responsibilities:                                                       │
│  • Parse and validate HTTP request (path params, query, JSON body)       │
│  • Call service method                                                   │
│  • Format and write HTTP response (JSON + status code)                  │
│  • NO business logic, NO DB access                                      │
│                                                                          │
│  Pattern:                                                                │
│    func (h *StaffHandler) UpdateHeadcounts(w http.ResponseWriter,       │
│        r *http.Request) {                                               │
│        tenantID := ctxutil.GetTenantID(r.Context())                     │
│        scenID := parseUUID(chi.URLParam(r, "scenID"))                   │
│        var req UpdateHeadcountsRequest                                  │
│        if err := decodeAndValidate(r, &req); err != nil { ... }         │
│        result, err := h.staffService.UpdateHeadcounts(                  │
│            r.Context(), tenantID, scenID, req)                           │
│        if err != nil { handleError(w, err); return }                    │
│        respondJSON(w, http.StatusOK, result)                            │
│    }                                                                     │
└────┬────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─ SERVICE ───────────────────────────────────────────────────────────────┐
│  Responsibilities:                                                       │
│  • Orchestrate business logic                                            │
│  • Call repo for data access                                             │
│  • Call compute engine for calculations                                  │
│  • Enforce business rules and validation                                 │
│  • Manage transactions (when multiple repos involved)                   │
│  • Emit audit log entries                                                │
│  • NO HTTP awareness (doesn't know about requests/responses)            │
│                                                                          │
│  Pattern:                                                                │
│    func (s *StaffService) UpdateHeadcounts(ctx context.Context,         │
│        tenantID, scenID uuid.UUID, req UpdateHeadcountsRequest,         │
│    ) (*model.StaffPayrollSummary, error) {                              │
│        // 1. Validate scenario belongs to tenant                         │
│        // 2. Repo: save headcounts                                       │
│        // 3. Repo: fetch all staff data for this scenario                │
│        // 4. Compute: recompute payroll summary                          │
│        // 5. Audit: log the change                                       │
│        // 6. Return computed summary                                     │
│    }                                                                     │
└────┬────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─ REPOSITORY ────────────────────────────────────────────────────────────┐
│  Responsibilities:                                                       │
│  • GORM database operations (CRUD)                                       │
│  • Tenant-scoped via BaseRepo.DB(tenantID)                              │
│  • Return model structs or errors                                        │
│  • NO business logic, NO computation                                    │
│                                                                          │
│  Pattern:                                                                │
│    func (r *StaffRepo) UpdateHeadcounts(tenantID, scenID uuid.UUID,     │
│        entries []model.StaffHeadcount) error {                           │
│        return r.DB(tenantID).Transaction(func(tx *gorm.DB) error {      │
│            for _, e := range entries {                                    │
│                e.TenantID = tenantID                                     │
│                e.ScenarioID = scenID                                     │
│                if err := tx.Save(&e).Error; err != nil { return err }   │
│            }                                                             │
│            return nil                                                    │
│        })                                                                │
│    }                                                                     │
└────┬────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─ MODEL ─────────────────────────────────────────────────────────────────┐
│  • Pure data structs with GORM tags + JSON tags                          │
│  • No methods beyond trivial helpers (TableName, validation)            │
│  • Input models: stored in DB (have tenant_id, scenario_id)             │
│  • Report models: computed on the fly (never stored)                    │
│  • Decimal fields for all monetary values                                │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 6. The Computation Engine (internal/compute/)

The compute package is the **heart of the application** — it replaces Excel's formula engine.
All functions are **pure**: they take model structs as input and return report structs as
output, with no database access or side effects. This makes them trivially testable.

### 6.1 Computation Dependency Graph

```
                    ┌──────────┐
                    │ Settings │ (PlanConfig, OpeningBalance, WCConfig)
                    └────┬─────┘
                         │
          ┌──────────────┼──────────────┬──────────────┐
          ▼              ▼              ▼              ▼
    ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
    │ Products │  │  Staff   │  │  Capex   │  │  Opex    │
    │ (P1–P5)  │  │ Payroll  │  │ Deprec.  │  │ Summary  │
    └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘
         │              │              │              │
         ▼              │              │              │
    ┌──────────┐        │              │              │
    │ Revenue  │        │              │              │
    │ Consol.  │        │              │              │
    └────┬─────┘        │              │              │
         │              │              │              │
         └──────────────┴──────┬───────┴──────────────┘
                               │
                    ┌──────────┤ (+ FiPlan grants as user inputs)
                    ▼          │
              ┌──────────┐    │
              │   P&L    │    │ ◄── French format, 7 aggregates
              │ (ComputePnl)  │
              └────┬─────┘    │
                   │          │
         ┌─────────┼──────────┼─────────┐
         ▼         ▼          ▼         ▼
    ┌─────────┐ ┌──────┐ ┌────────┐ ┌──────────┐
    │  WCR    │ │FiPlan│ │P&L+Cash│ │ Budget1  │
    │         │ │      │ │(Anglo) │ │ (Yr1 Mo) │
    └────┬────┘ └──┬───┘ └────────┘ └────┬─────┘
         │         │                      │
         └────┬────┘                      ▼
              │                     ┌──────────┐
              ▼                     │ Budget2  │
         ┌──────────┐              │ (Aggr.)  │
         │  BSheet  │              └──────────┘
         └────┬─────┘
              │
              ▼
         ┌──────────┐
         │  Ratios  │ ◄── NPV, IRR, KPIs
         └────┬─────┘
              │
         ┌────┴──────────────┐
         ▼                   ▼
    ┌──────────┐       ┌──────────┐
    │  Cash    │       │ Graph1   │ (annual charts)
    │ (36 Mo)  │       └──────────┘
    └────┬─────┘
         │
         ▼
    ┌──────────┐
    │ Graph2   │ (monthly charts)
    └──────────┘
```

### 6.2 Full Plan Computation Orchestrator

```go
// internal/compute/engine.go

package compute

// FullPlanInput collects all user inputs needed for a full recomputation.
type FullPlanInput struct {
    Config         model.PlanConfig
    OpeningBalance model.OpeningBalance
    WCConfig       model.WorkingCapitalConfig
    Products       []model.Product
    ProductData    []ProductInputBundle      // assumptions, volumes, margins per product
    Headcounts     []model.StaffHeadcount
    Salaries       []model.StaffSalary
    Incentives     []model.StaffIncentive
    CapexEntries   []model.CapexEntry
    OpexEntries    []model.OpexManualEntry
    PnlEntries     []model.PnlManualEntry
    FiplanEntries  []model.FiplanEntry
    PnlCashEntries []model.PnlCashEntry
    WCREntries     []model.WCREntry
    CashOverrides  []model.CashMonthlyOverride
    BudgetOverrides []model.BudgetMonthlyOverride
}

// ProductInputBundle groups all input data for one product.
type ProductInputBundle struct {
    Assumptions []model.ProductAssumption
    Volumes     []model.ProductSalesVolume
    Margins     []model.ProductDistributorMargin
}

// FullPlanOutput holds every computed report for a scenario.
type FullPlanOutput struct {
    ProductRevenues []model.ProductRevenueReport
    Revenue         model.ConsolidatedRevenue
    Staff           model.StaffPayrollSummary
    Capex           model.CapexSummary
    Opex            model.OpexSummary
    PnL             model.PnlReport
    FiPlan          model.FiplanReport
    PnlCash         model.PnlCashReport
    WCR             model.WCRReport
    BSheet          model.BSheetReport
    Ratios          model.RatiosReport
    Cash            model.CashReport
    Budget1         model.Budget1Report
    Budget2         model.Budget2Report
    Warnings        []model.ValidationWarning
}

// ComputeFullPlan runs the entire computation graph in dependency order.
// This is a PURE FUNCTION: no DB access, no side effects.
func ComputeFullPlan(input FullPlanInput) FullPlanOutput {
    var out FullPlanOutput

    // ── Layer 1: Independent sheet computations ──
    for i, p := range input.Products {
        out.ProductRevenues = append(out.ProductRevenues,
            ComputeProductRevenue(p, input.ProductData[i], input.Config))
    }
    out.Revenue = ComputeConsolidatedRevenue(out.ProductRevenues, input.Config)
    out.Staff = ComputeStaffPayroll(input.Headcounts, input.Salaries,
        input.Incentives, input.Config)
    out.Capex = ComputeCapexSummary(input.CapexEntries, input.Config)
    out.Opex = ComputeOpexSummary(input.OpexEntries, out.Revenue,
        out.Staff, input.Config)

    // ── Layer 2: P&L (depends on Layer 1 + FiPlan grants) ──
    grants := GetGrantsForPnl(input.FiplanEntries)
    out.PnL = ComputePnl(input.PnlEntries, out.Revenue, out.Staff,
        out.Capex, out.Opex, grants, input.Config)

    // ── Layer 3: Sheets depending on P&L ──
    out.WCR = ComputeWCR(input.WCREntries, out.Revenue, out.Opex,
        out.Staff, input.WCConfig, input.Config)
    out.FiPlan = ComputeFiplan(input.FiplanEntries, out.PnL, out.Capex,
        out.WCR, input.Config)
    out.PnlCash = ComputePnlCash(input.PnlCashEntries, out.Revenue,
        out.Staff, out.Capex, out.Opex, out.PnL, input.Config)

    // ── Layer 4: Balance Sheet (depends on P&L, WCR, FiPlan, Capex) ──
    out.BSheet = ComputeBSheet(out.PnL, out.WCR, out.FiPlan, out.Capex,
        input.OpeningBalance, input.Config)

    // ── Layer 5: Ratios (reads everything) ──
    out.Ratios = ComputeRatios(out.PnL, out.BSheet, out.WCR, out.FiPlan,
        out.Revenue, out.Staff, out.Capex, input.Config)

    // ── Layer 6: Monthly Cash (depends on all upstream annual) ──
    out.Cash = ComputeCash(input.Config, input.CashOverrides, out.Revenue,
        input.Products, input.ProductData, out.Staff, out.Opex,
        out.Capex, out.FiPlan, out.PnL, out.BSheet)

    // ── Layer 7: Budgets (depends on Budget1, which reads upstream annuals) ──
    out.Budget1 = ComputeBudget1(input.Config, input.BudgetOverrides,
        out.Revenue, input.Products, out.Opex, out.Staff,
        out.PnL, out.Capex, out.FiPlan)
    out.Budget2 = ComputeBudget2(out.Budget1)

    // ── Cross-checks ──
    out.Warnings = RunCrossChecks(out)

    return out
}
```

### 6.3 Incremental vs Full Computation

Not every edit requires full recomputation. The service layer decides based on what changed:

```go
// internal/service/report_service.go

type ReportService struct {
    repos   *RepoBundle      // all repos
    logger  *zap.Logger
}

// GetFullReport loads all inputs and runs full computation.
func (s *ReportService) GetFullReport(ctx context.Context,
    tenantID, scenID uuid.UUID,
) (*compute.FullPlanOutput, error) {
    input, err := s.loadAllInputs(ctx, tenantID, scenID)
    if err != nil {
        return nil, err
    }
    result := compute.ComputeFullPlan(input)
    return &result, nil
}

// Incremental: when only staff changes, recompute from staff down.
// This is an OPTIMIZATION — the full recompute is always correct.
var dependencyMap = map[string][]string{
    "settings":  {"products", "staff", "capex", "opex", "pnl", "fiplan",
                  "pnl_cash", "wcr", "bsheet", "ratios", "cash", "budget"},
    "products":  {"revenue", "pnl", "wcr", "cash", "budget"},
    "staff":     {"opex", "pnl", "pnl_cash", "wcr", "cash", "budget"},
    "capex":     {"pnl", "pnl_cash", "bsheet", "cash", "budget"},
    "opex":      {"pnl", "pnl_cash", "wcr", "cash", "budget"},
    "pnl":       {"fiplan", "pnl_cash", "bsheet", "ratios", "budget"},
    "fiplan":    {"bsheet", "cash"},
    "wcr":       {"fiplan", "bsheet", "ratios"},
    "cash_over": {"cash"},       // only monthly cash recalc
    "budget_over": {"budget1", "budget2"},  // only budget recalc
}
```

### 6.4 Treasury Calibration (Circular Reference Resolution)

The Excel model has a circular reference between P&L financial income/expenses and the cash
position (you need cash to compute interest, but interest affects cash). The spreadsheet
resolves this with iterative recalculation (F9 key). We implement a convergence loop:

```go
// internal/compute/calibration.go

const (
    maxCalibrationIterations = 50
    convergenceThreshold     = 0.01 // k€
)

// CalibrateFinancials runs the iterative convergence loop.
// Starting from initial P&L/FiPlan outputs, it adjusts financial income
// and expenses until the cash position stabilizes.
func CalibrateFinancials(
    pnl     *model.PnlReport,
    fiplan  *model.FiplanReport,
    bsheet  *model.BSheetReport,
    config  model.PlanConfig,
) (converged bool, iterations int) {
    prevCash := [5]decimal.Decimal{}

    for i := 0; i < maxCalibrationIterations; i++ {
        // 1. Compute financial income from positive cash (interest earned)
        // 2. Compute overdraft expense from negative cash
        // 3. Re-run P&L with adjusted financial lines
        // 4. Re-run FiPlan cash flow
        // 5. Re-run BSheet to get new cash position
        // 6. Check convergence

        newCash := bsheet.Detailed.Cash
        maxDiff := decimal.Zero
        for y := 0; y < 5; y++ {
            diff := newCash[y].Sub(prevCash[y]).Abs()
            if diff.GreaterThan(maxDiff) {
                maxDiff = diff
            }
        }

        if maxDiff.LessThan(decimal.NewFromFloat(convergenceThreshold)) {
            return true, i + 1
        }
        prevCash = newCash
    }
    return false, maxCalibrationIterations
}
```

---

## 7. GORM Model Conventions

### 7.1 Base Model and Hooks

```go
// internal/model/base.go

package model

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

// TenantScoped is embedded in every data model for multi-tenancy.
type TenantScoped struct {
    ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    TenantID  uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
    CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// BeforeCreate hook — ensure tenant_id and UUID are set.
func (b *TenantScoped) BeforeCreate(tx *gorm.DB) error {
    if b.ID == uuid.Nil {
        b.ID = uuid.New()
    }
    return nil
}
```

### 7.2 GORM Tags Convention

```go
// Example: StaffHeadcount model with GORM tags
type StaffHeadcount struct {
    TenantScoped

    ScenarioID uuid.UUID       `gorm:"type:uuid;not null;index:idx_staff_hc_scen" json:"scenarioId"`
    RoleIndex  int             `gorm:"not null;check:role_index >= 0 AND role_index < 8" json:"roleIndex"`
    Year       int             `gorm:"not null;check:year >= 1 AND year <= 5" json:"year"`
    Count      decimal.Decimal `gorm:"type:numeric(10,2);not null" json:"count"`

    // GORM unique constraint
    // gorm:"uniqueIndex:idx_staff_hc_unique,composite:tenant_id,scenario_id,role_index,year"
}

func (StaffHeadcount) TableName() string {
    return "staff_headcounts"
}
```

### 7.3 Decimal Handling with GORM

```go
// GORM natively supports shopspring/decimal via the numeric PostgreSQL type.
// No custom serializer needed — just use gorm:"type:numeric(20,4)"
//
// For JSON serialization, decimal.Decimal marshals to string by default.
// We use a custom JSON encoder to serialize as number:

import "github.com/shopspring/decimal"

func init() {
    // Configure shopspring/decimal to marshal as JSON number (not string)
    decimal.MarshalJSONWithoutQuotes = true
}
```

---

## 8. Versioning: Scenarios + Snapshots

### 8.1 Scenario Model (Already Designed)

Each BusinessPlan has N Scenarios. A Scenario is a **complete copy** of all settings and data,
enabling side-by-side comparison (base / optimistic / pessimistic).

```
BusinessPlan
├── Scenario: "Base"      ← default, created with plan
├── Scenario: "Optimistic"← cloned from Base
└── Scenario: "Pessimistic"← cloned from Base
```

### 8.2 Snapshot Model (Version History)

Snapshots capture a **point-in-time freeze** of a scenario's complete state. Used for:
audit trail, undo, comparison, approval workflows.

```go
// internal/model/snapshot.go

// PlanSnapshot captures a complete frozen state of a scenario.
type PlanSnapshot struct {
    TenantScoped

    ScenarioID  uuid.UUID `gorm:"type:uuid;not null;index" json:"scenarioId"`
    Version     int       `gorm:"not null" json:"version"`            // Auto-increment per scenario
    Label       string    `gorm:"size:255" json:"label"`              // "v1", "Pre-board review", etc.
    CreatedBy   uuid.UUID `gorm:"type:uuid;not null" json:"createdBy"`
    Reason      string    `gorm:"size:500" json:"reason"`             // Optional comment

    // Complete state serialized as JSONB
    Data        SnapshotData `gorm:"type:jsonb;not null" json:"-"`    // Large — excluded from list API

    // Computed report at snapshot time (optional, for quick preview)
    ReportHash  string    `gorm:"size:64" json:"reportHash"`          // SHA-256 of the full report
}

func (PlanSnapshot) TableName() string {
    return "plan_snapshots"
}

// SnapshotData holds the complete serialized state of a scenario.
type SnapshotData struct {
    Config         PlanConfig               `json:"config"`
    OpeningBalance OpeningBalance           `json:"openingBalance"`
    WCConfig       WorkingCapitalConfig     `json:"wcConfig"`
    Products       []Product                `json:"products"`
    // ... all input tables serialized as JSON ...
    StaffHeadcounts []StaffHeadcount        `json:"staffHeadcounts"`
    StaffSalaries   []StaffSalary           `json:"staffSalaries"`
    StaffIncentives []StaffIncentive        `json:"staffIncentives"`
    CapexEntries    []CapexEntry            `json:"capexEntries"`
    OpexEntries     []OpexManualEntry       `json:"opexEntries"`
    PnlEntries      []PnlManualEntry        `json:"pnlEntries"`
    FiplanEntries   []FiplanEntry           `json:"fiplanEntries"`
    PnlCashEntries  []PnlCashEntry          `json:"pnlCashEntries"`
    WCREntries      []WCREntry              `json:"wcrEntries"`
    CashOverrides   []CashMonthlyOverride   `json:"cashOverrides"`
    BudgetOverrides []BudgetMonthlyOverride  `json:"budgetOverrides"`
}
```

### 8.3 Snapshot Service

```go
// internal/service/snapshot_service.go

type SnapshotService struct {
    snapshotRepo repo.SnapshotRepo
    repos        *RepoBundle
    logger       *zap.Logger
}

// Create creates a new snapshot of the scenario's current state.
func (s *SnapshotService) Create(ctx context.Context,
    tenantID, scenID, userID uuid.UUID, label, reason string,
) (*model.PlanSnapshot, error) {
    // 1. Load all inputs for this scenario
    data, err := s.loadSnapshotData(ctx, tenantID, scenID)
    if err != nil {
        return nil, err
    }

    // 2. Get next version number
    lastVersion, err := s.snapshotRepo.GetMaxVersion(tenantID, scenID)
    if err != nil {
        return nil, err
    }

    // 3. Save snapshot
    snapshot := &model.PlanSnapshot{
        TenantScoped: model.TenantScoped{TenantID: tenantID},
        ScenarioID:   scenID,
        Version:      lastVersion + 1,
        Label:        label,
        CreatedBy:    userID,
        Reason:       reason,
        Data:         *data,
    }

    return snapshot, s.snapshotRepo.Create(snapshot)
}

// Restore rolls back a scenario to a snapshot state.
func (s *SnapshotService) Restore(ctx context.Context,
    tenantID, scenID, snapshotID uuid.UUID,
) error {
    // 1. Load snapshot
    // 2. Create a new snapshot of current state (before-restore backup)
    // 3. Delete all current data for this scenario
    // 4. Insert snapshot data as new current state
    // 5. Audit log
}

// Clone creates a new scenario from a snapshot.
func (s *SnapshotService) CloneToScenario(ctx context.Context,
    tenantID, snapshotID uuid.UUID, newName string,
) (*model.Scenario, error) { ... }

// Diff compares two snapshots (or snapshot vs current).
func (s *SnapshotService) Diff(ctx context.Context,
    tenantID uuid.UUID, snapshotA, snapshotB uuid.UUID,
) (*model.SnapshotDiff, error) { ... }
```

### 8.4 Snapshot API

```go
r.Route("/plans/{planID}/scenarios/{scenID}/snapshots", func(r chi.Router) {
    r.Get("/",           snapshotHandler.List)          // List versions
    r.Post("/",          snapshotHandler.Create)         // Create snapshot
    r.Get("/{snapID}",   snapshotHandler.Get)            // Get snapshot metadata
    r.Get("/{snapID}/data", snapshotHandler.GetData)     // Get full data (large)
    r.Post("/{snapID}/restore", snapshotHandler.Restore) // Restore to this version
    r.Post("/{snapID}/clone", snapshotHandler.Clone)     // Clone to new scenario
    r.Get("/diff",       snapshotHandler.Diff)            // Compare two versions
})
```

### 8.5 Auto-Snapshot Triggers

```go
// Automatic snapshots are created before destructive operations:
// 1. Before scenario restore (backup current state)
// 2. Before plan approval (freeze approved state)
// 3. Before bulk import (backup before overwrite)
// Manual snapshots: user explicitly saves a version ("Save checkpoint")
```

---

## 9. Complete API Route Map

```go
// internal/router/router.go

func NewRouter(
    cfg        *config.Config,
    db         *gorm.DB,
    logger     *zap.Logger,
    handlers   *HandlerBundle,
    middleware *MiddlewareBundle,
) *chi.Mux {
    r := chi.NewRouter()

    // ── Global Middleware ──
    r.Use(middleware.Recover)
    r.Use(middleware.CORS)
    r.Use(middleware.RequestID)
    r.Use(middleware.Logger)

    // ── Health Check (no auth) ──
    r.Get("/health", handlers.Health.Check)
    r.Get("/ready", handlers.Health.Ready)

    // ── Auth (no tenant scope) ──
    r.Route("/auth", func(r chi.Router) {
        r.Post("/login", handlers.Auth.Login)
        r.Post("/callback", handlers.Auth.Callback)
        r.Post("/refresh", handlers.Auth.Refresh)
        r.Post("/logout", handlers.Auth.Logout)
    })

    // ── Authenticated Routes ──
    r.Group(func(r chi.Router) {
        r.Use(middleware.Auth)
        r.Use(middleware.Tenant)

        // ── Tenant Management ──
        r.Route("/tenant", func(r chi.Router) {
            r.Use(RequirePermission(PermManageTenant))
            r.Get("/", handlers.Tenant.Get)
            r.Put("/", handlers.Tenant.Update)
        })

        // ── User Management ──
        r.Route("/users", func(r chi.Router) {
            r.Get("/me", handlers.User.GetMe)
            r.Put("/me", handlers.User.UpdateMe)

            r.Group(func(r chi.Router) {
                r.Use(RequirePermission(PermManageUsers))
                r.Get("/", handlers.User.List)
                r.Post("/invite", handlers.User.Invite)
                r.Put("/{userID}/role", handlers.User.UpdateRole)
                r.Delete("/{userID}", handlers.User.Deactivate)
            })
        })

        // ── Business Plans ──
        r.Route("/plans", func(r chi.Router) {
            r.Get("/", handlers.Plan.List)

            r.Group(func(r chi.Router) {
                r.Use(RequirePermission(PermManagePlan))
                r.Post("/", handlers.Plan.Create)
            })

            r.Route("/{planID}", func(r chi.Router) {
                r.Use(RequirePermission(PermViewPlan))
                r.Get("/", handlers.Plan.Get)

                r.Group(func(r chi.Router) {
                    r.Use(RequirePermission(PermEditPlan))
                    r.Put("/", handlers.Plan.Update)
                })
                r.Group(func(r chi.Router) {
                    r.Use(RequirePermission(PermManagePlan))
                    r.Delete("/", handlers.Plan.Delete)
                    r.Post("/approve", handlers.Plan.Approve)
                    r.Post("/archive", handlers.Plan.Archive)
                })

                // ── Scenarios ──
                r.Route("/scenarios", func(r chi.Router) {
                    r.Get("/", handlers.Scenario.List)

                    r.Group(func(r chi.Router) {
                        r.Use(RequirePermission(PermEditPlan))
                        r.Post("/", handlers.Scenario.Create)
                        r.Post("/clone/{srcScenID}", handlers.Scenario.Clone)
                    })

                    r.Route("/{scenID}", func(r chi.Router) {
                        r.Get("/", handlers.Scenario.Get)

                        r.Group(func(r chi.Router) {
                            r.Use(RequirePermission(PermEditPlan))
                            r.Put("/", handlers.Scenario.Update)
                            r.Delete("/", handlers.Scenario.Delete)
                        })

                        // ── Settings ──
                        r.Route("/settings", func(r chi.Router) {
                            r.Get("/config", handlers.Settings.GetConfig)
                            r.Get("/opening-balance", handlers.Settings.GetOpeningBalance)
                            r.Get("/wc-config", handlers.Settings.GetWCConfig)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/config", handlers.Settings.UpdateConfig)
                                r.Put("/opening-balance", handlers.Settings.UpdateOpeningBalance)
                                r.Put("/wc-config", handlers.Settings.UpdateWCConfig)
                            })
                        })

                        // ── Products ──
                        r.Route("/products", func(r chi.Router) {
                            r.Get("/", handlers.Product.List)
                            r.Get("/{productID}", handlers.Product.Get)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Post("/", handlers.Product.Create)
                                r.Put("/{productID}", handlers.Product.Update)
                                r.Delete("/{productID}", handlers.Product.Delete)
                                r.Put("/{productID}/assumptions", handlers.Product.UpdateAssumptions)
                                r.Put("/{productID}/volumes", handlers.Product.UpdateVolumes)
                                r.Put("/{productID}/margins", handlers.Product.UpdateMargins)
                            })

                            r.Get("/revenue-summary", handlers.Product.GetConsolidatedRevenue)
                        })

                        // ── Staff ──
                        r.Route("/staff", func(r chi.Router) {
                            r.Get("/headcount", handlers.Staff.ListHeadcounts)
                            r.Get("/salaries", handlers.Staff.ListSalaries)
                            r.Get("/incentives", handlers.Staff.ListIncentives)
                            r.Get("/summary", handlers.Staff.GetPayrollSummary)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/headcount", handlers.Staff.UpdateHeadcounts)
                                r.Put("/salaries", handlers.Staff.UpdateSalaries)
                                r.Put("/incentives", handlers.Staff.UpdateIncentives)
                            })
                        })

                        // ── Capex ──
                        r.Route("/capex", func(r chi.Router) {
                            r.Get("/", handlers.Capex.ListEntries)
                            r.Get("/summary", handlers.Capex.GetSummary)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/", handlers.Capex.UpdateEntries)
                                r.Put("/durations", handlers.Capex.UpdateDurations)
                            })
                        })

                        // ── Opex ──
                        r.Route("/opex", func(r chi.Router) {
                            r.Get("/entries", handlers.Opex.ListManualEntries)
                            r.Get("/summary", handlers.Opex.GetSummary)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/entries", handlers.Opex.UpdateManualEntries)
                            })
                        })

                        // ── P&L (French) ──
                        r.Route("/pnl", func(r chi.Router) {
                            r.Get("/entries", handlers.PnL.ListManualEntries)
                            r.Get("/report", handlers.PnL.GetReport)
                            r.Get("/chart", handlers.PnL.GetChartData)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/entries", handlers.PnL.UpdateManualEntries)
                            })
                        })

                        // ── Financing Plan ──
                        r.Route("/fiplan", func(r chi.Router) {
                            r.Get("/entries", handlers.FiPlan.ListEntries)
                            r.Get("/report", handlers.FiPlan.GetReport)
                            r.Get("/grants", handlers.FiPlan.GetGrantsForPnl)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/entries", handlers.FiPlan.UpdateEntries)
                            })
                        })

                        // ── P&L + Cash (Anglo-Saxon) ──
                        r.Route("/pnl-cash", func(r chi.Router) {
                            r.Get("/entries", handlers.PnlCash.ListEntries)
                            r.Get("/report", handlers.PnlCash.GetReport)
                            r.Get("/chart", handlers.PnlCash.GetChartData)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/entries", handlers.PnlCash.UpdateEntries)
                            })
                        })

                        // ── Balance Sheet (read-only) ──
                        r.Route("/bsheet", func(r chi.Router) {
                            r.Get("/report", handlers.BSheet.GetReport)
                            r.Get("/charts", handlers.BSheet.GetCharts)
                        })

                        // ── Ratios / KPIs (read-only) ──
                        r.Route("/ratios", func(r chi.Router) {
                            r.Get("/report", handlers.Ratios.GetReport)
                            r.Get("/charts/{chartType}", handlers.Ratios.GetChart)
                        })

                        // ── Working Capital Requirements ──
                        r.Route("/wcr", func(r chi.Router) {
                            r.Get("/entries", handlers.WCR.ListEntries)
                            r.Get("/report", handlers.WCR.GetReport)
                            r.Get("/charts", handlers.WCR.GetCharts)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/entries", handlers.WCR.UpdateEntries)
                            })
                        })

                        // ── Monthly Cash Flow (36 months) ──
                        r.Route("/cash", func(r chi.Router) {
                            r.Get("/overrides", handlers.Cash.ListOverrides)
                            r.Get("/report", handlers.Cash.GetReport)
                            r.Get("/report/{yearIndex}", handlers.Cash.GetYearReport)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Put("/overrides", handlers.Cash.BatchUpsertOverrides)
                                r.Delete("/overrides/{lineID}/{month}", handlers.Cash.DeleteOverride)
                            })
                        })

                        // ── Budgets ──
                        r.Route("/budgets", func(r chi.Router) {
                            r.Get("/", handlers.Budget.GetCombinedReport)

                            r.Route("/year1", func(r chi.Router) {
                                r.Get("/overrides", handlers.Budget.ListYear1Overrides)
                                r.Get("/report", handlers.Budget.GetBudget1Report)

                                r.Group(func(r chi.Router) {
                                    r.Use(RequirePermission(PermEditPlan))
                                    r.Put("/overrides", handlers.Budget.BatchUpsertYear1Overrides)
                                    r.Post("/overrides/reset", handlers.Budget.ResetYear1Overrides)
                                })
                            })

                            r.Get("/year2/report", handlers.Budget.GetBudget2Report)
                        })

                        // ── Full Plan Report (all sheets) ──
                        r.Get("/report", handlers.Report.GetFullReport)

                        // ── Snapshots (versioning) ──
                        r.Route("/snapshots", func(r chi.Router) {
                            r.Get("/", handlers.Snapshot.List)
                            r.Get("/{snapID}", handlers.Snapshot.Get)

                            r.Group(func(r chi.Router) {
                                r.Use(RequirePermission(PermEditPlan))
                                r.Post("/", handlers.Snapshot.Create)
                                r.Post("/{snapID}/restore", handlers.Snapshot.Restore)
                                r.Post("/{snapID}/clone", handlers.Snapshot.Clone)
                            })

                            r.Get("/diff", handlers.Snapshot.Diff)
                        })
                    }) // /{scenID}
                }) // /scenarios
            }) // /{planID}
        }) // /plans
    }) // Authenticated group

    return r
}
```

---

## 10. Database Schema Overview

### 10.1 All Tables

```
┌─ PLATFORM ──────────────────────────────────────────────────┐
│  tenants                  │ Multi-tenant root               │
│  users                    │ Per-tenant users                │
│  audit_logs               │ Change tracking                 │
└─────────────────────────────────────────────────────────────┘

┌─ PLAN MANAGEMENT ───────────────────────────────────────────┐
│  business_plans           │ Root container per forecast     │
│  scenarios                │ Variants (base/optimistic/etc.) │
│  plan_snapshots           │ Version history (JSONB data)    │
└─────────────────────────────────────────────────────────────┘

┌─ SETTINGS (per scenario) ───────────────────────────────────┐
│  plan_configs             │ General config, rates, payroll  │
│  opening_balances         │ Initial balance sheet           │
│  working_capital_configs  │ Payment terms, inventory days   │
│  geographic_zones         │ 3 zone labels                   │
│  multi_year_adjustments   │ Per-year rate adjustments       │
└─────────────────────────────────────────────────────────────┘

┌─ OPERATIONAL DATA (per scenario) ───────────────────────────┐
│  products                 │ Up to 5 products per scenario   │
│  product_assumptions      │ Prices, costs per product/year  │
│  product_sales_volumes    │ Units per product/zone/year     │
│  product_distributor_margins │ Margin % per product/year    │
│  staff_headcounts         │ FTE per role (8) per year (5)   │
│  staff_salaries           │ Gross salary per role/year      │
│  staff_incentives         │ Bonus per role/year             │
│  capex_entries            │ Investment per category (14)/yr │
│  opex_manual_entries      │ Manual opex overrides           │
│  pnl_manual_entries       │ P&L manual overrides            │
│  fiplan_entries           │ Financing plan inputs           │
│  pnl_cash_entries         │ Anglo-Saxon P&L overrides       │
│  wcr_entries              │ WCR adjustment entries          │
│  cash_monthly_overrides   │ Monthly cash timing (36 mo)    │
│  budget_monthly_overrides │ Monthly budget timing (Budget1) │
└─────────────────────────────────────────────────────────────┘

Total: ~25 tables
```

### 10.2 Migration Strategy

```go
// Sequential SQL migrations using golang-migrate/migrate
// Each migration is idempotent with up/down pair.

// migrations/000001_create_tenants.up.sql
// migrations/000001_create_tenants.down.sql
// migrations/000002_create_users.up.sql
// ...

// GORM AutoMigrate is NOT used in production.
// GORM is used only for queries, not schema management.
// This ensures explicit control over schema changes.
```

### 10.3 Common Column Patterns

```sql
-- Every data table follows this pattern:
CREATE TABLE example_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    scenario_id     UUID NOT NULL REFERENCES scenarios(id),
    -- domain-specific columns --
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Standard indexes
CREATE INDEX idx_example_tenant ON example_entries(tenant_id);
CREATE INDEX idx_example_scenario ON example_entries(scenario_id);

-- RLS policy
ALTER TABLE example_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY example_tenant_policy ON example_entries
    USING (tenant_id = current_setting('app.tenant_id')::UUID);
```

---

## 11. Error Handling

```go
// internal/pkg/apierror/errors.go

package apierror

import (
    "net/http"
)

// AppError is the standard error type returned by services.
type AppError struct {
    Code       string `json:"code"`       // Machine-readable: "not_found", "validation_error"
    Message    string `json:"message"`    // Human-readable
    StatusCode int    `json:"-"`          // HTTP status code
    Details    any    `json:"details,omitempty"` // Optional structured details
}

func (e *AppError) Error() string { return e.Message }

// Standard constructors
func NotFound(entity, id string) *AppError {
    return &AppError{
        Code: "not_found", StatusCode: http.StatusNotFound,
        Message: entity + " not found: " + id,
    }
}
func ValidationError(msg string, details any) *AppError {
    return &AppError{
        Code: "validation_error", StatusCode: http.StatusUnprocessableEntity,
        Message: msg, Details: details,
    }
}
func Forbidden(msg string) *AppError {
    return &AppError{Code: "forbidden", StatusCode: http.StatusForbidden, Message: msg}
}
func Unauthorized(msg string) *AppError {
    return &AppError{Code: "unauthorized", StatusCode: http.StatusUnauthorized, Message: msg}
}
func Internal(msg string) *AppError {
    return &AppError{Code: "internal_error", StatusCode: http.StatusInternalServerError, Message: msg}
}
func Conflict(msg string) *AppError {
    return &AppError{Code: "conflict", StatusCode: http.StatusConflict, Message: msg}
}

// JSON API error response
type ErrorResponse struct {
    Error AppError `json:"error"`
}
```

### 11.1 Handler Error Pattern

```go
// internal/handler/response.go

func handleError(w http.ResponseWriter, err error) {
    var appErr *apierror.AppError
    if errors.As(err, &appErr) {
        respondJSON(w, appErr.StatusCode, apierror.ErrorResponse{Error: *appErr})
        return
    }
    // Unknown error — log and return 500
    logger.Error("unexpected error", zap.Error(err))
    respondJSON(w, 500, apierror.ErrorResponse{
        Error: *apierror.Internal("an unexpected error occurred"),
    })
}

func respondJSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}
```

---

## 12. Logging

```go
// internal/middleware/logger.go

// Structured request logging with zap.
// Every request logs: method, path, status, duration, tenant_id, user_id, request_id.
// The logger is injected into context for service/repo layers.

func LoggerMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            reqID := r.Header.Get("X-Request-ID")

            // Request-scoped logger
            reqLogger := logger.With(
                zap.String("request_id", reqID),
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
            )

            // Inject into context
            ctx := context.WithValue(r.Context(), ctxutil.LoggerKey, reqLogger)

            // Wrap response writer to capture status
            ww := &statusWriter{ResponseWriter: w, status: 200}
            next.ServeHTTP(ww, r.WithContext(ctx))

            // Log completion
            reqLogger.Info("request completed",
                zap.Int("status", ww.status),
                zap.Duration("duration", time.Since(start)),
                zap.String("tenant_id", ctxutil.GetTenantIDStr(ctx)),
            )
        })
    }
}
```

---

## 13. Audit Trail

```go
// internal/model/audit.go

type AuditAction string

const (
    AuditCreate  AuditAction = "create"
    AuditUpdate  AuditAction = "update"
    AuditDelete  AuditAction = "delete"
    AuditApprove AuditAction = "approve"
    AuditRestore AuditAction = "restore"
    AuditClone   AuditAction = "clone"
)

type AuditLog struct {
    TenantScoped

    UserID      uuid.UUID   `gorm:"type:uuid;not null" json:"userId"`
    ScenarioID  *uuid.UUID  `gorm:"type:uuid" json:"scenarioId"`
    EntityType  string      `gorm:"size:50;not null" json:"entityType"`  // "staff", "capex", etc.
    EntityID    *uuid.UUID  `gorm:"type:uuid" json:"entityId"`
    Action      AuditAction `gorm:"size:20;not null" json:"action"`
    Changes     any         `gorm:"type:jsonb" json:"changes"`           // Before/after diff
    IPAddress   string      `gorm:"size:45" json:"ipAddress"`
}

func (AuditLog) TableName() string {
    return "audit_logs"
}
```

---

## 14. Dependency Injection (main.go)

```go
// cmd/server/main.go

package main

func main() {
    // ── Load Config ──
    cfg := config.Load()

    // ── Logger ──
    logger, _ := zap.NewProduction()
    if cfg.Env == "development" {
        logger, _ = zap.NewDevelopment()
    }
    defer logger.Sync()

    // ── Database ──
    db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
        Logger: gormlogger.New(logger),
    })
    if err != nil {
        logger.Fatal("failed to connect to database", zap.Error(err))
    }

    // ── Run Migrations ──
    runMigrations(cfg.DatabaseURL)

    // ── Repositories ──
    repos := &repo.RepoBundle{
        Tenant:   repo.NewTenantRepo(db),
        User:     repo.NewUserRepo(db),
        Plan:     repo.NewPlanRepo(db),
        Settings: repo.NewSettingsRepo(db),
        Product:  repo.NewProductRepo(db),
        Staff:    repo.NewStaffRepo(db),
        Capex:    repo.NewCapexRepo(db),
        Opex:     repo.NewOpexRepo(db),
        PnL:      repo.NewPnlRepo(db),
        FiPlan:   repo.NewFiplanRepo(db),
        PnlCash:  repo.NewPnlCashRepo(db),
        WCR:      repo.NewWCRRepo(db),
        Cash:     repo.NewCashRepo(db),
        Budget:   repo.NewBudgetRepo(db),
        Snapshot: repo.NewSnapshotRepo(db),
        Audit:    repo.NewAuditRepo(db),
    }

    // ── Services ──
    services := &service.ServiceBundle{
        Auth:     service.NewAuthService(cfg.Socrate, logger),
        Tenant:   service.NewTenantService(repos.Tenant, logger),
        User:     service.NewUserService(repos.User, logger),
        Plan:     service.NewPlanService(repos.Plan, repos.Snapshot, logger),
        Settings: service.NewSettingsService(repos.Settings, logger),
        Product:  service.NewProductService(repos.Product, logger),
        Staff:    service.NewStaffService(repos.Staff, logger),
        Capex:    service.NewCapexService(repos.Capex, logger),
        Opex:     service.NewOpexService(repos.Opex, logger),
        PnL:      service.NewPnlService(repos.PnL, logger),
        FiPlan:   service.NewFiplanService(repos.FiPlan, logger),
        PnlCash:  service.NewPnlCashService(repos.PnlCash, logger),
        BSheet:   service.NewBSheetService(logger),
        Ratios:   service.NewRatiosService(logger),
        WCR:      service.NewWCRService(repos.WCR, logger),
        Cash:     service.NewCashService(repos.Cash, logger),
        Budget:   service.NewBudgetService(repos.Budget, logger),
        Report:   service.NewReportService(repos, logger),
        Snapshot: service.NewSnapshotService(repos.Snapshot, repos, logger),
    }

    // ── Handlers ──
    handlers := &handler.HandlerBundle{
        Auth:     handler.NewAuthHandler(services.Auth, logger),
        Tenant:   handler.NewTenantHandler(services.Tenant, logger),
        // ... all handlers ...
        Report:   handler.NewReportHandler(services.Report, logger),
        Snapshot: handler.NewSnapshotHandler(services.Snapshot, logger),
    }

    // ── Middleware ──
    mw := &middleware.MiddlewareBundle{
        Auth:    middleware.NewAuthMiddleware(cfg.Socrate.JWKSURL, logger),
        Tenant:  middleware.NewTenantMiddleware(db, logger),
        Recover: middleware.RecoverMiddleware(logger),
        CORS:    middleware.CORSMiddleware(cfg.AllowedOrigins),
        Logger:  middleware.LoggerMiddleware(logger),
    }

    // ── Router ──
    r := router.NewRouter(cfg, db, logger, handlers, mw)

    // ── Start Server ──
    addr := fmt.Sprintf(":%d", cfg.Port)
    logger.Info("starting server", zap.String("addr", addr))
    if err := http.ListenAndServe(addr, r); err != nil {
        logger.Fatal("server failed", zap.Error(err))
    }
}
```

---

## 15. Configuration

```go
// internal/config/config.go

type Config struct {
    Env            string   `env:"APP_ENV" envDefault:"development"`
    Port           int      `env:"PORT" envDefault:"8080"`
    DatabaseURL    string   `env:"DATABASE_URL,required"`
    AllowedOrigins []string `env:"CORS_ORIGINS" envSeparator:","`

    Socrate SocrateConfig
}

type SocrateConfig struct {
    BaseURL      string `env:"SOCRATE_BASE_URL,required"`   // e.g., https://auth.kerplan.com
    ClientID     string `env:"SOCRATE_CLIENT_ID,required"`
    ClientSecret string `env:"SOCRATE_CLIENT_SECRET,required"`
    JWKSURL      string `env:"SOCRATE_JWKS_URL,required"`   // e.g., https://auth.kerplan.com/.well-known/jwks.json
    RedirectURL  string `env:"SOCRATE_REDIRECT_URL,required"`
}
```

---

## 16. Cross-Cutting Concerns Summary

| Concern | Implementation |
|---|---|
| **Multi-tenancy** | `tenant_id` on every table + GORM scope + PostgreSQL RLS |
| **Multi-user** | OAuth2 via Socrate, JWT claims, RBAC middleware |
| **Multi-plan** | `BusinessPlan` → N `Scenario` hierarchy |
| **Versioning** | `PlanSnapshot` with JSONB state + version numbering |
| **Authentication** | Socrate OAuth2 PKCE flow, JWT validation, JWKS rotation |
| **Authorization** | Role-based: Owner > Admin > Editor > Viewer |
| **Logging** | Structured (zap), request-scoped, tenant-aware |
| **Audit trail** | `AuditLog` table with before/after JSONB diffs |
| **Validation** | `go-playground/validator` on request DTOs + compute cross-checks |
| **Error handling** | Typed `AppError` with code/message/status/details |
| **Monetary precision** | `shopspring/decimal` everywhere, `numeric(20,4)` in DB |
| **Computation** | Pure functions in `internal/compute/`, no side effects |
| **Transactions** | GORM `Transaction()` for multi-table writes |
| **Migrations** | `golang-migrate`, sequential SQL files, no AutoMigrate |
| **Testing** | Unit (compute), integration (repo+DB), E2E (handler+httptest) |

---

## 17. Input vs Computed Data Summary

| Domain | DB Tables (Input) | Computed Reports | Compute Function |
|---|---|---|---|
| Settings | `plan_configs`, `opening_balances`, `working_capital_configs`, `geographic_zones`, `multi_year_adjustments` | `PlanConfigComputed` | — |
| Products | `products`, `product_assumptions`, `product_sales_volumes`, `product_distributor_margins` | `ProductRevenueReport`, `ConsolidatedRevenue` | `ComputeProductRevenue`, `ComputeConsolidatedRevenue` |
| Staff | `staff_headcounts`, `staff_salaries`, `staff_incentives` | `StaffPayrollSummary` | `ComputeStaffPayroll` |
| Capex | `capex_entries` | `CapexSummary` | `ComputeCapexSummary`, `ComputeSLNDepreciation` |
| Opex | `opex_manual_entries` | `OpexSummary` | `ComputeOpexSummary` |
| P&L | `pnl_manual_entries` | `PnlReport` | `ComputePnl` |
| FiPlan | `fiplan_entries` | `FiplanReport` | `ComputeFiplan` |
| P&L+Cash | `pnl_cash_entries` | `PnlCashReport` | `ComputePnlCash` |
| BSheet | *(none)* | `BSheetReport` | `ComputeBSheet` |
| Ratios | *(none)* | `RatiosReport` | `ComputeRatios` |
| WCR | `wcr_entries` | `WCRReport` | `ComputeWCR` |
| Cash | `cash_monthly_overrides` | `CashReport` | `ComputeCash` |
| Budget | `budget_monthly_overrides` | `Budget1Report`, `Budget2Report` | `ComputeBudget1`, `ComputeBudget2` |
| Graphs | *(none)* | Chart data from above reports | — |
| **Totals** | **~25 tables** | **~15 report types** | **~15 compute functions** |
