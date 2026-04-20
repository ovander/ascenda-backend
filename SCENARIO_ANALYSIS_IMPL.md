# ScenarioAnalysisService — Implementation Report

**Project:** Ascenda  
**Date:** 2026-04-12  
**Scope:** Decision-intelligence layer on top of the existing compute engine  
**Status:** ✅ Implemented & tests passing

---

## 1. Summary

This report documents the design decisions, architecture, and implementation details of the `ScenarioAnalysisService` — a new backend service that transforms raw financial model outputs into structured, decision-oriented analysis for each business plan scenario.

The service was designed around three constraints:

1. **Do not touch the compute engine.** All financial math already exists; the service only reads `FullPlanOutput`.
2. **Eliminate code duplication.** The `FullPlanInput` assembly logic was previously duplicated in `ReportService`. A shared `PlanComputeOrchestrator` now owns it.
3. **Keep business logic pure and testable.** All analysis functions are pure (no I/O, no DB), so the full suite runs without a database or mocks.

---

## 2. Architecture

### 2.1 Position in the stack

```
HTTP Handlers
     ↓
ScenarioAnalysisService        ← new
     ↓
ReportService (cache-aware)    ← modified
     ↓
PlanComputeOrchestrator        ← new (shared)
     ↓
compute.ComputeFullPlan()      ← unchanged
     ↓
Repositories                   ← unchanged
```

The service sits at the same layer as every other service in the codebase. It has no peer dependencies — only a downward dependency on `ReportService`.

### 2.2 Option B — shared orchestrator

Before this work, `ReportService` embedded ~200 lines of repository-loading logic in a private `loadAllInputs()` method. The analysis service would have needed to duplicate that logic to assemble its own `FullPlanInput`.

**Option B** instead extracts `loadAllInputs` into a `PlanComputeOrchestrator` that is injected into both services:

```
PlanComputeOrchestrator.LoadInputs()
        ↑ used by
ReportService.GetFullReport()   →  cached FullPlanOutput
ScenarioAnalysisService         →  via ReportService (inherits cache)
```

`ScenarioAnalysisService` calls `reportService.GetFullReport()` — it gets the LRU cache for free, with no additional infrastructure.

---

## 3. Files changed

### 3.1 New files

| File | Purpose |
|---|---|
| `internal/service/plan_compute_orchestrator.go` | Owns all `FullPlanInput` assembly; single source of truth for repo → compute wiring |
| `internal/service/scenario_analysis.go` | Analysis service + all DTOs + pure business logic functions |
| `internal/handler/scenario_analysis_handler.go` | HTTP handler (`GET .../analysis`) |
| `internal/service/scenario_analysis_test.go` | 30+ unit tests for all pure logic functions |

### 3.2 Modified files

| File | Change |
|---|---|
| `internal/service/report_service.go` | Replaced `repos *repo.RepoBundle` + 200-line `loadAllInputs()` with `orchestrator *PlanComputeOrchestrator` |
| `internal/service/service_bundle.go` | Wired `PlanComputeOrchestrator`, updated `NewReportService` call, added `ScenarioAnalysis` field |
| `internal/handler/handler_bundle.go` | Added `ScenarioAnalysis *ScenarioAnalysisHandler` to `PlanHandlers` |
| `internal/router/router.go` | Registered `GET /analysis` route under the scenario sub-router |

---

## 4. HTTP endpoint

```
GET /api/v1/plans/{planId}/scenarios/{scenarioId}/analysis
```

**Middleware:** same stack as all scenario routes — auth, tenant, plan access check, `reportTimeout` (30 s), `reportLimiter` (10 req/s).

**Response (200):**

```json
{
  "viability": {
    "score": 75,
    "status": "moderate"
  },
  "projections": {
    "revenue_5y": "2450000",
    "ebitda_peak": "380000",
    "break_even_month": 19,
    "cash_min": "-42000"
  },
  "risks": [
    {
      "type": "cash_gap",
      "severity": "high",
      "message": "Cash position drops to -42000 — the scenario may require additional financing.",
      "value": "-42000"
    },
    {
      "type": "late_profitability",
      "severity": "medium",
      "message": "Break-even is reached at month 19, which is beyond the 24-month threshold.",
      "when": 19
    }
  ],
  "drivers": [
    {
      "name": "Hiring too aggressive",
      "impact": "high",
      "effect": "Payroll represents 62% of year-1 revenue — reduce hiring pace or increase revenue to improve the ratio."
    }
  ],
  "trends": {
    "revenue": ["320000", "510000", "740000", "920000", "1100000"],
    "cash": ["80000", "62000", "41000", "...]
  }
}
```

---

## 5. Business logic

### 5.1 Projections

All four KPIs are derived directly from `FullPlanOutput`:

| KPI | Source | Formula |
|---|---|---|
| `revenue_5y` | `PnlReport.Years[i].Sales` | Sum across 5 years |
| `ebitda_peak` | `PnlReport.Years[i].EBITDA` | Max across 5 years |
| `break_even_month` | `PnlReport.Years[i].NetProfit` | First month where cumulative net profit > 0, spreading each year's profit linearly over 12 months. Returns 0 if never reached. |
| `cash_min` | `CashReport.Years[y].ClosingBalance[m]` | Min over 36 monthly closing balances (3-year cash horizon) |

### 5.2 Viability scoring

Five weighted criteria summing to 100:

| Criterion | Weight | Condition |
|---|---|---|
| Cash stays positive | 30 | `cash_min >= 0` |
| Break-even < 24 months | 25 | `break_even_month > 0 && <= 24` |
| EBITDA becomes positive | 20 | `ebitda_peak > 0` |
| Revenue growth is real | 15 | `revenue_5y > 0` |
| Burn control | 10 | Cash losses < 20% of 5Y revenue |

Classification: `strong` (80–100), `moderate` (50–79), `risky` (< 50).

### 5.3 Risk detection

Four risk types, each a simple threshold check:

| Risk type | Severity | Trigger |
|---|---|---|
| `cash_gap` | high | `cash_min < 0` |
| `late_profitability` | medium | `break_even_month > 24` |
| `late_profitability` | high | `break_even_month == 0` (never) |
| `no_profitability` | high | `ebitda_peak <= 0` |
| `unrealistic_growth` | medium | Max YoY revenue growth > 100% |

### 5.4 Driver identification (v1 heuristics)

Four heuristics operate on year-1 P&L ratios and year-5 EBITDA margin:

| Driver name | Trigger | Impact |
|---|---|---|
| Hiring too aggressive | Payroll / Revenue (Y1) > 50% | high |
| Pricing too low | Gross margin (Y1) < 20% | high |
| Inefficient growth model | Burn > 50% of revenue AND YoY growth < 20% | medium |
| Strong operating leverage | EBITDA margin (Y5) > 25% | high (positive) |

### 5.5 Trends

- **Revenue:** 5 annual `Sales` values from `PnlReport`
- **Cash:** 36 monthly `ClosingBalance` values from `CashReport` (3 years)

These are intentionally compact for mobile rendering and AI narration input.

---

## 6. Caching

No new cache infrastructure was added. `ScenarioAnalysisService` calls `ReportService.GetFullReport()`, which already uses the existing LRU cache (max 50 scenarios) wired to the `ReportInvalidator` event subscriber. Any data mutation that invalidates the report cache also invalidates the analysis result transparently.

---

## 7. Testing

All business logic functions are pure — they take `model.PnlReport` / `model.CashReport` / `model.FullPlanOutput` structs and return values, with no I/O. This means the full test suite runs in-process with zero infrastructure:

```
ok  ascenda/internal/service  2.494s
```

### 7.1 Test coverage

| Area | Test count | Notes |
|---|---|---|
| `computeRevenue5Y` | 2 | Sum, all-zero |
| `computeEbitdaPeak` | 2 | Max selection, all-negative |
| `computeBreakEvenMonth` | 3 | Mid-horizon, month 1, never |
| `computeCashMin` | 2 | Negative spike, always positive |
| `computeViability` | 4 | Strong, risky, moderate, thresholds |
| `detectRisks` | 6 | Each risk type + healthy no-risk scenario |
| `identifyDrivers` | 4 | Each driver + efficient no-driver scenario |
| `extractTrends` | 2 | Series length and values |
| `maxYoYGrowthPct` | 2 | Triple growth, zero base year |

### 7.2 Known issue fixed during implementation

`assert.Equal` performs a deep structural comparison of `decimal.Decimal` internals. Two decimals representing the same numeric value can have different `(mantissa, exponent)` representations — e.g. `15 × 10²` vs `150 × 10¹` after arithmetic. All decimal assertions in the test file use a `assertDecEq` helper that calls `.Equal()` instead.

---

## 8. Future evolution

The spec identifies three natural next steps, all accommodated by the current design:

**Sensitivity analysis:** The `PlanComputeOrchestrator.LoadInputs()` method returns the raw `FullPlanInput`, which gives direct access to all assumptions. A v2 driver identification step could perturb individual inputs and re-run `Compute()` to measure actual sensitivity.

**Scenario comparison:** `AnalyzeScenario` takes a single `scenarioID`. A `CompareScenarios(ctx, tenantID, []uuid.UUID)` method can be added to the service interface that calls `AnalyzeScenario` in parallel for each ID and diffs the results.

**AI narration:** The `ScenarioAnalysisResult` struct is already structured as an `AIInput` payload. The AI narration service (`AINarrationService`) can be injected into `ScenarioAnalysisService` to add an optional `Narrative string` field to the result, gated by the existing `AIAccessMiddleware`.

**Redis cache (v2):** The `ReportCache` interface can be swapped for a Redis-backed implementation without touching the analysis service.
