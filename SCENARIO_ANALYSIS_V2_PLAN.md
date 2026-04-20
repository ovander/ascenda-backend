# ScenarioAnalysisService v2 — Strategy & Sprint Plan (Rev 2)

> **Status:** Pre-implementation — incorporates v1 spec evaluation + author feedback (Rev 3 — final)  
> **Scope:** `internal/service/scenario_analysis.go` and adjacent files only  
> **Constraint:** Compute engine is frozen. API contract is additive-only.

---

## 1. What v2 Is

v1 produced a financial summary — a structured read of `FullPlanOutput` with simple heuristics on top.

v2 targets a different bar: **decision-intelligence for a VC-review context**. The output must answer four questions a business plan reviewer actually asks:

1. *Is this fundable?* → Continuous viability score with financial rationale.
2. *Where does it break, and how close is it to working?* → Ranked risks with urgency, magnitude, and global coherence.
3. *What drives the outcome, and what caused it?* → Top 3 levers, distinguishing root causes from symptoms.
4. *How much capital does this need, and is the model stable?* → Funding requirement, trajectory signal, and fragility.

---

## 2. Architectural Decisions (Rev 2 — Post-Feedback)

### 2.1 Break-Even: Three-State Status + Approaching Detection

**Problem confirmed by feedback:** If `Economic.Total` values are always negative but trending toward zero (e.g., −100, −80, −60, −40, −20 …), the current approach returns 0 ("never reached"), producing a false negative and pessimistic bias. The business is viable — it just hasn't crossed the threshold yet.

**Solution:** Replace the binary `BreakEvenMonth` with a three-state `BreakEvenStatus` and add trend-based projection.

Detection logic for `"approaching"`:
1. Compute a linear regression slope over the last 12 months of cumulative `Economic.Total`.
2. **Noise floor guard (new):** two independent conditions must both pass — reject `"approaching"` if either fails:
   - **Proportional guard:** `slope ≥ 0.01 × avgMonthlyRevenue` — prevents a large-revenue signal being masked while still filtering noise on small-revenue scenarios.
   - **Absolute guard:** `slope ≥ minAbsoluteBEPSlope` where `minAbsoluteBEPSlope = 100` — prevents proportionally-valid but value-insignificant slopes on micro-revenue scenarios (a €3/month slope that is 1.5% of a €200/month pre-revenue startup should not trigger `"approaching"`).
   - Constants: `minBEPSlopeRevenueFraction = 0.01`, `minAbsoluteBEPSlope = 100.0`. Both named, both promotable to config.
3. If both guards pass, extrapolate: `projectedMonth = 36 + ceil(abs(cumulative_at_m36) / slope)`.
4. If `projectedMonth ≤ 60`: status = `"approaching"`, `BreakEvenMonth` = projected month, `BreakEvenMethod` = `"extrapolated"`.
5. Otherwise: status = `"not_reached"`, `BreakEvenMonth` = 0.

Using slope over 12 months (not first vs. last value) is intentional: it resists oscillation and detects genuine trend rather than noise.

```go
// BreakEvenStatus values
const (
    BEPReached     = "reached"      // cumulative Economic turned positive within 60 months
    BEPApproaching = "approaching"  // not yet positive but trend extrapolates to ≤ 60 months
    BEPNotReached  = "not_reached"  // cumulative negative with no improving trend in horizon
)
```

`ProjectionResult` gains:
```go
BreakEvenStatus  string `json:"break_even_status"`  // reached | approaching | not_reached
BreakEvenMethod  string `json:"break_even_method"`  // cash_economic | pnl_cashflow_approx | extrapolated
```

**Important for downstream consumers:** viability scoring uses `BreakEvenMonth` as a number (0–60), so `"approaching"` at projected month 45 still contributes a partial score. This is correct behaviour — a scenario approaching BEP at month 45 is better than one that never approaches at all.

**Sprint 1 test additions for the noise floor:**
- `TestBreakEven_Approaching_SlopeBelowThreshold_IsNotReached` — slope of 0.005 × avgRevenue → must return `"not_reached"`, not `"approaching"`.
- `TestBreakEven_Approaching_SlopeAboveThreshold_IsApproaching` — slope of 0.02 × avgRevenue → must return `"approaching"`.

---

### 2.2 Viability Scoring: Revenue5Y Dependency Fixed

**Problem confirmed by feedback:** `cashMin / revenue5Y` is structurally unsafe. When `revenue5Y` is small (pre-revenue startup), the ratio explodes and dominates the score. When `revenue5Y` is large, a significant cash hole is masked.

**Solution:** Two-branch cash health scoring.

```go
const (
    minMeaningfulRevenue = 10_000.0 // below this, absolute thresholds apply

    // Relative branch anchors (revenue5Y ≥ threshold)
    cashHealthCeiling = 0.10   // cashMin / rev ≥ +10% → 30 pts
    cashHealthFloor   = -0.50  // cashMin / rev ≤ −50% → 0 pts

    // Absolute branch anchors (revenue5Y < threshold)
    cashHealthAbsCeiling = 50_000.0   // cashMin ≥ +50K → 30 pts
    cashHealthAbsFloor   = -200_000.0 // cashMin ≤ −200K → 0 pts
)

func scoreCashHealth(cashMin, rev5Y decimal.Decimal) float64 {
    rev, _ := rev5Y.Float64()
    cash, _ := cashMin.Float64()

    if rev >= minMeaningfulRevenue {
        ratio := cash / rev
        return clamp64(30*(ratio-cashHealthFloor)/(cashHealthCeiling-cashHealthFloor), 0, 30)
    }
    // Absolute branch: pre-revenue startup
    return clamp64(30*(cash-cashHealthAbsFloor)/(cashHealthAbsCeiling-cashHealthAbsFloor), 0, 30)
}
```

The absolute thresholds (`50K` / `−200K`) are named constants, not magic numbers, and can be promoted to config when country-specific calibration is needed.

---

### 2.3 Risk Model: Global Coherence via RiskSummary

**Problem confirmed by feedback:** Three medium-severity risks with no global signal is analytically misleading. A scenario can look "medium risk" per line but be globally critical.

**Solution:** Add `RiskSummary` with a weighted aggregate.

Weighting: `high = 3`, `medium = 2`, `low = 1`. Sum all risk scores.

| Total Score | GlobalRiskLevel |
|---|---|
| 0 | `"none"` |
| 1–2 | `"low"` |
| 3–5 | `"moderate"` |
| 6–8 | `"elevated"` |
| ≥ 9 | `"critical"` |

```go
type RiskSummary struct {
    GlobalRiskLevel string `json:"global_risk_level"` // none | low | moderate | elevated | critical
    RiskScore       int    `json:"risk_score"`         // weighted aggregate (high=3, med=2, low=1)
    RiskCount       int    `json:"risk_count"`
}
```

`ScenarioAnalysisResult` gains `RiskSummary RiskSummary`. It is computed after `detectRisks` by aggregating the returned slice — pure function, no new inputs needed.

**Interaction with viability scoring — resolved (Option A):** A scenario with `score = 82` (classified "strong") and `GlobalRiskLevel = "critical"` is a product contradiction. Option A is confirmed: apply a hard cap of 60 when `GlobalRiskLevel == "critical"`. This forces critical-risk scenarios into the "moderate" band at most, making `"strong"` + `"critical"` structurally impossible.

```go
// In computeViability, after computing raw score from the 5 criteria:
if riskSummary.GlobalRiskLevel == "critical" {
    score = min(score, 60)
}
```

`computeViability` must therefore receive `RiskSummary` as an additional argument, or the cap is applied after the fact in `AnalyzeScenario`. The latter is cleaner — the viability function stays pure on projections; the orchestration layer applies the cap:

```go
// In AnalyzeScenario:
viability := computeViability(projections)
riskSummary := computeRiskSummary(risks)
if riskSummary.GlobalRiskLevel == "critical" {
    viability.Score = min(viability.Score, 60)
    viability.Status = classifyViability(viability.Score) // re-classify after cap
}
```

This order dependency (risks computed before viability is finalised) must be explicit in code comments.

---

### 2.4 Drivers: Causality Dimension

**Problem confirmed by feedback:** The current system identifies symptoms, not causes. "High burn" is a symptom; "aggressive hiring" and "low gross margin" are causes. A product decision layer that sees "high burn" without understanding why cannot act on it.

**Solution:** Add `RootCause bool` to `Driver`. Redefine:
- `RootCause = true`: the driver directly explains *why* an outcome occurs (e.g., payroll/revenue ratio explains burn).
- `RootCause = false`: the driver describes a resulting condition (e.g., "inefficient growth model" is a consequence of root causes).

**Ordering rule:** Root causes rank above symptoms of equal priority. In the top-3 selection sort:
1. Primary key: `priority` (desc)
2. Secondary key: `rootCause` (true before false)

This ensures the output surface explains *why* before describing *what*.

Updated `Driver` struct:
```go
type Driver struct {
    Name      string `json:"name"`
    Impact    string `json:"impact"`     // high | medium | low
    Effect    string `json:"effect"`
    RootCause bool   `json:"root_cause"` // true = root cause, false = symptom/consequence
}
```

Existing drivers classification:
| Driver | RootCause |
|---|---|
| Hiring too aggressive | `true` — explains high payroll cost |
| Pricing too low | `true` — explains low gross margin |
| Inefficient growth model | `false` — symptom of prior two combined |
| Strong operating leverage | `true` — explains EBITDA margin trajectory |
| Cash contribution: payroll dominant | `true` — causal, not derivative |

---

### 2.5 Sensitivity: Typed EconomicLever

**Problem confirmed by feedback:** `Parameter string` is unvalidated and unmaintainable. A typo produces a silent failure.

**Solution:** Type alias with const enumeration.

```go
type EconomicLever string

const (
    LeverRevenueScale   EconomicLever = "revenue_scale"
    LeverHeadcountScale EconomicLever = "headcount_scale"
    LeverCOGSScale      EconomicLever = "cogs_scale"
)

type Perturbation struct {
    Lever EconomicLever   `json:"lever"`
    Delta decimal.Decimal `json:"delta"` // e.g. +0.10 for +10%, −0.20 for −20%
}
```

`SensitivityEngine.Run` validates each `Lever` value at entry and returns `apierror.BadRequest` for unknown levers.

**Performance limits:** Add hard cap at `N ≤ 10` perturbations per call. Pass `context.Context` through. Use a named constant `sensitivityTimeout = 15 * time.Second` (separate from the 30s `reportTimeout`) — not a magic literal, not the same constant as the report timeout. Add early-stop: if a perturbation produces a viability delta of 0 and is the 3rd consecutive zero-delta result, stop evaluating further perturbations of the same lever.

---

### 2.6 Scenario Fragility (Strategic — Sprint 7)

**From feedback 5.3:** Sensitivity alone produces a table of numbers. Fragility is the signal above that table: how much does the business model depend on optimistic assumptions holding true?

**Definition:** Fragility = normalised standard deviation of viability deltas across all perturbations.

```go
// FragilityScore: 0 = stable (perturbations barely move viability), 100 = fragile (perturbations collapse it)
// FragilitySignal: "stable" | "sensitive" | "fragile"

type SensitivityReport struct {
    Results         []SensitivityResult `json:"results"`
    FragilityScore  int                 `json:"fragility_score"`
    FragilitySignal string              `json:"fragility_signal"` // stable | sensitive | fragile
}
```

Computation: normalise by `maxPossibleDelta = 100` (the full viability score range) to make the score comparable across scenarios regardless of the absolute magnitude of individual deltas:

```
FragilityScore = clamp(int(stddev(viabilityDeltas) * 100 / maxPossibleDelta), 0, 100)
               = clamp(int(stddev(viabilityDeltas)), 0, 100)   // since maxPossibleDelta = 100
```

This is directly interpretable: a fragility score of 30 means perturbations shift viability by ±30 points on average. The previous `× 5` multiplier was arbitrary and not scale-invariant — it has been removed. No named calibration constant is needed because the formula is now mathematically grounded.

Guard: if `len(results) < 2`, return `FragilityScore = 0`, `FragilitySignal = "stable"` immediately — a single perturbation result has no variance by definition.

Classification: `0–20 → stable`, `21–50 → sensitive`, `51+ → fragile`.

---

### 2.7 Funding Requirement (Strategic — Sprint 1)

**From feedback 5.1:** Telling the user "cash drops to −42K" is incomplete. The product question is "how much do you need to raise?" This is a direct derivation from data already computed.

```go
// ProjectionResult additions:
FundingRequired decimal.Decimal `json:"funding_required"` // max(0, cashMin.Neg())
FundingMonth    int             `json:"funding_month"`    // month when cashMin occurs (= CashMinMonth)
FundingUrgency  string          `json:"funding_urgency"`  // immediate | near_term | long_term (NEW)
```

`FundingRequired` = 0 when `CashMin ≥ 0`. When positive, it represents the minimum external capital needed to avoid a cash shortfall under this scenario's assumptions. This is not a recommendation — it is a mechanical derivation. The AI narration layer is responsible for contextualising it.

`FundingUrgency` is derived from `FundingMonth` and is only populated when `FundingRequired > 0`:

```go
func fundingUrgencyFromMonth(month int) string {
    switch {
    case month <= 6:  return "immediate"  // < 6 months: cannot wait to fundraise
    case month <= 18: return "near_term"  // 6–18 months: raise within current year
    default:          return "long_term"  // 18+ months: time to optimise before raising
    }
}
```

The distinction has direct product value: a founder with `"immediate"` urgency cannot afford a 6-month fundraising process; a `"long_term"` need gives them runway to optimise assumptions before going to market. Empty string when `FundingRequired == 0`.

---

### 2.8 Time to Scale (Strategic — Sprint 5)

**From feedback 5.2:** BEP tells you when profitability starts. CAGR tells you the average growth rate. Neither tells you *when the business enters its growth phase* — the inflection where revenue acceleration becomes self-sustaining.

**Definition:** `TimeToScale` = first month where the month-over-month revenue growth rate (from `CashReport.Years[y].Revenue.Total[m]`) increases for two consecutive months after a period of deceleration or flatness. This is the second-derivative turning positive on revenue.

**Guards (new — prevent false signals from noise):**
1. **Minimum revenue threshold:** `minScaleRevenue = 5_000`. Monthly revenue must exceed this before growth-rate analysis is meaningful. Below this level, month-over-month changes are dominated by rounding and sparse sales data rather than genuine acceleration.
2. **Minimum variation threshold:** `minScaleVariation = 0.02` (2%). A month-over-month revenue change of less than 2% does not constitute a meaningful acceleration signal. Both consecutive months must individually exceed this variation to qualify.

If either guard is not met for the two consecutive months, `TimeToScale = 0`.

Guard: if revenue is monotonically accelerating from month 1 with both months clearing the guards, `TimeToScale = 1`. If no qualifying inflection is observed within 36 months, `TimeToScale = 0`.

Added to `TrendInsights`:
```go
TimeToScale int `json:"time_to_scale"` // month when revenue acceleration begins; 0 = not observed
```

---

### 2.9 Trend Signal (Secondary — Sprint 5)

**From feedback 4.1:** `TrendInsights` is descriptive but not prescriptive. Adding a single summarising signal bridges the gap.

```go
TrendSignal string `json:"trend_signal"` // improving | deteriorating | volatile
```

Classification logic using already-computed fields:
- If `RevenueCAGR > 0.10` AND `BurnRateMonthly` decreasing across years: `"improving"`
- If `RevenueCAGR < 0` OR `BurnRateMonthly` is increasing: `"deteriorating"`
- If `RevenueVolatility > 0.50`: `"volatile"` (overrides improving/deteriorating — high volatility means the trend is not reliable regardless of direction)

---

### 2.10 Headline Variation

**From feedback 3.6:** A fixed template produces repetitive output with low narrative value.

**Solution:** Small decision tree over `(status, globalRiskLevel, topDriverRootCause)` producing semantically distinct headlines. No AI required — the variation is deterministic.

```
if status == "strong"  && globalRiskLevel ∈ {"none","low"}                → "Healthy scenario: [driver] drives profitability by month [bep]."
if status == "strong"  && globalRiskLevel ∈ {"moderate","elevated"}       → "Strong viability ([score]/100) but [top_risk_type] poses a risk at month [when]."
if status == "moderate" && rootCause driver exists                         → "[RootCause] limits growth: break-even at month [bep], viability [score]/100."
if status == "moderate" && no root cause                                   → "Moderate scenario: [bep_status] at month [bep], [risk_count] risk(s) identified."
if status == "risky"   && globalRiskLevel == "critical"                    → "Critical: [N] compounding risks — model requires structural revision before funding."
if status == "risky"   && bepStatus == "approaching"                       → "Approaching viability but not yet fundable: projected break-even at month [bep]."
if status == "risky"   && bepStatus == "not_reached"                       → "Not fundable under current assumptions: no break-even path within 5 years."
```

7 distinct headline types. Implementable as a `switch` with ordered conditions.

---

## 3. Complete v2 Output Shape

```go
type ScenarioAnalysisResult struct {
    Viability   ViabilityResult  `json:"viability"`
    Projections ProjectionResult `json:"projections"`
    Risks       []Risk           `json:"risks"`
    RiskSummary RiskSummary      `json:"risk_summary"`  // NEW (Sprint 3)
    Drivers     []Driver         `json:"drivers"`       // max 3
    Trends      Trends           `json:"trends"`
    Insights    TrendInsights    `json:"insights"`      // NEW (Sprint 5)
    Highlights  Highlights       `json:"highlights"`    // NEW (Sprint 6)
}

type ProjectionResult struct {
    Revenue5Y       decimal.Decimal `json:"revenue_5y"`
    EbitdaPeak      decimal.Decimal `json:"ebitda_peak"`
    BreakEvenMonth  int             `json:"break_even_month"`
    BreakEvenStatus string          `json:"break_even_status"` // NEW (Sprint 1)
    BreakEvenMethod string          `json:"break_even_method"` // NEW (Sprint 1)
    CashMin         decimal.Decimal `json:"cash_min"`
    CashMinMonth    int             `json:"cash_min_month"`    // NEW (Sprint 1)
    FundingRequired decimal.Decimal `json:"funding_required"`  // NEW (Sprint 1)
    FundingMonth    int             `json:"funding_month"`     // NEW (Sprint 1)
    FundingUrgency  string          `json:"funding_urgency"`   // NEW (Sprint 1) — empty when FundingRequired == 0
}

type Risk struct {
    Type     string          `json:"type"`
    Severity string          `json:"severity"`
    Urgency  string          `json:"urgency"`        // NEW (Sprint 3)
    Message  string          `json:"message"`
    When     *int            `json:"when,omitempty"`
    Value    decimal.Decimal `json:"value,omitempty"`
}

type RiskSummary struct {
    GlobalRiskLevel string `json:"global_risk_level"` // NEW (Sprint 3)
    RiskScore       int    `json:"risk_score"`
    RiskCount       int    `json:"risk_count"`
}

type Driver struct {
    Name      string `json:"name"`
    Impact    string `json:"impact"`
    Effect    string `json:"effect"`
    RootCause bool   `json:"root_cause"` // NEW (Sprint 4)
}

type TrendInsights struct {
    RevenueCAGR       decimal.Decimal `json:"revenue_cagr"`
    BurnRateMonthly   decimal.Decimal `json:"burn_rate_monthly"`
    RevenueVolatility decimal.Decimal `json:"revenue_volatility"`
    InflectionMonth   int             `json:"inflection_month"`
    TimeToScale       int             `json:"time_to_scale"`   // NEW (Sprint 5)
    TrendSignal       string          `json:"trend_signal"`    // NEW (Sprint 5)
}

type Highlights struct {
    Headline   string   `json:"headline"`
    Strengths  []string `json:"strengths"`
    Weaknesses []string `json:"weaknesses"`
    TopRisks   []string `json:"top_risks"`
    TopDrivers []string `json:"top_drivers"`
}
```

---

## 4. Sprint Plan (Rev 2)

### Sprint 1 — Core Accuracy + Funding Signal
**Goal:** Fix the most critical analytical errors; introduce funding requirement.  
**Files:** `scenario_analysis.go`, `plan_compute_orchestrator.go`, `scenario_analysis_test.go`

**Tasks:**
1. Add `ComputeFromInput(input compute.FullPlanInput) *model.FullPlanOutput` to `PlanComputeOrchestrator`.
2. Implement `computeBreakEvenV2(pnl model.PnlReport, cash model.CashReport) (month int, status, method string)`:
   - Phase 1 (months 1–36): scan cumulative `Economic.Total[m]`, return on first > 0 with status=`"reached"`, method=`"cash_economic"`.
   - Phase 2 (months 37–60): linear spread of `PnL.Years[y].CashFlow`, continue accumulating, return on first > 0 with status=`"reached"`, method=`"pnl_cashflow_approx"`.
   - Approaching detection: if cumulative never > 0, compute linear regression slope on last 12 months of `Economic.Total`. If slope > 0, extrapolate crossing month. If ≤ 60, return projected month with status=`"approaching"`, method=`"extrapolated"`.
   - Otherwise: return 0, `"not_reached"`, `"pnl_cashflow_approx"`.
3. Implement `computeCashMinMonth(cash model.CashReport) int`.
4. Add `FundingRequired`, `FundingMonth`, `CashMinMonth`, `BreakEvenStatus`, `BreakEvenMethod` to `ProjectionResult`.
5. Update `computeProjections` to populate all new fields.
6. Update `makeCash` test helper to populate `Economic.Total` (not just `ClosingBalance`).
7. Tests:
   - `TestBreakEven_ReachedViaEconomicCash`
   - `TestBreakEven_ReachedViaPnLApprox_Months37to60`
   - `TestBreakEven_Approaching_ConvergingNegative` ← the missed edge case
   - `TestBreakEven_NotReached_FlatNegative`
   - `TestBreakEven_NotReached_DivergingNegative`
   - `TestFundingRequired_NegativeCash`
   - `TestFundingRequired_PositiveCash_IsZero`

**Acceptance:** `BreakEvenStatus` is populated in all paths. "Approaching" test proves a converging-negative scenario no longer returns `status = "not_reached"`. `FundingRequired` is zero when cash never dips negative.

---

### Sprint 2 — Continuous Viability Scoring
**Goal:** Replace 5 binary criteria. Fix revenue5Y denominator risk.  
**Files:** `scenario_analysis.go`, `scenario_analysis_test.go`

**Tasks:**
1. Replace weight constants with anchor-point constants (§2.2 table above).
2. Implement `scoreCashHealth(cashMin, rev5Y decimal.Decimal) float64` with two-branch logic (§2.2).
3. Implement `scoreBreakEven(bep int) float64`: `bep = 0 → 0`, else `25 × (60 − bep) / 59` clamped.
4. Implement `scoreEbitdaMargin(ebitdaPeak, year5Sales decimal.Decimal) float64`: `clamp(10 + 10 × margin/0.25, 0, 20)`.
5. Implement `scoreRevenueRealism(maxYoYPct float64) float64`.
6. Implement `scoreBurnControl(cashMin, rev5Y decimal.Decimal) float64`.
7. Add `EbitdaMarginY5` and `MaxYoYGrowthPct` to `ProjectionResult` (precomputed, keeps `computeViability` signature clean).
8. Rewrite all viability tests with range assertions. Add:
   - `TestScoreCashHealth_RelativeBranch_*` (floor / midpoint / ceiling)
   - `TestScoreCashHealth_AbsoluteBranch_LowRevenue` ← the unsafe-ratio case
   - `TestComputeViability_ContinuousDistinction`: two scenarios identical in v1 scoring but differentiated in v2.

**Acceptance:** No test hardcodes a specific score integer. The "AbsoluteBranch" test demonstrates a pre-revenue scenario scores sensibly without ratio explosion.

---

### Sprint 3 — Risk Intelligence + Global Coherence
**Goal:** Add urgency, populate `CashGap.When`, add `high_leverage`, add `RiskSummary`.  
**Files:** `scenario_analysis.go`, `scenario_analysis_test.go`

**Tasks:**
1. Add `Urgency string` to `Risk`.
2. Implement `urgencyFromMonth(when *int) string` per §2.3 rule.
3. Add `computeCashMinMonth` result as `When` on `cash_gap` risk (introduced Sprint 1, wired here).
4. Add `high_leverage` risk detection using `BSheet.Analysis.NetDebt[4]` and `Ratios.Profitability.FreeCashFlow[4]`.
5. Fix `maxYoYGrowthPct` double-call in `detectRisks`.
6. Implement `computeRiskSummary(risks []Risk) RiskSummary`.
7. Add `RiskSummary` to `ScenarioAnalysisResult`.
8. **Product decision resolved (Option A):** apply viability cap `min(score, 60)` when `GlobalRiskLevel == "critical"`, then re-classify. This is applied in `AnalyzeScenario` after both `computeViability` and `computeRiskSummary` have returned — not inside either function. The execution order must be:
   ```
   projections → viability (raw) → risks → riskSummary → cap if critical → re-classify
   ```
   Add a code comment `// Cap applied after risk summary: prevents strong/critical contradiction.`
9. Tests:
   - `TestUrgencyDerivation_*` (one per urgency tier + nil when)
   - `TestCashGap_HasWhen_Populated`
   - `TestDetectRisks_HighLeverage`
   - `TestRiskSummary_ThreeMediumRisks_IsElevated`
   - `TestRiskSummary_OneLowRisk_IsLow`
   - `TestRiskSummary_NoRisks_IsNone`
   - `TestViabilityCap_StrongScoreWithCriticalRisk_IsCappedAtModerate` ← key regression guard

**Acceptance:** Every emitted risk has a non-empty `Urgency`. `RiskSummary.GlobalRiskLevel` always populated. `cash_gap` risk has `When != nil`. A scenario with raw score ≥ 80 and `GlobalRiskLevel == "critical"` must never return `Status == "strong"`.

---

### Sprint 4 — Driver Ranking + Causality
**Goal:** Enforce top-3, root cause classification, trajectory-based hiring, cash contribution.  
**Files:** `scenario_analysis.go`, `scenario_analysis_test.go`

**Tasks:**
1. Add `RootCause bool` to `Driver`.
2. Define internal `candidateDriver{Driver; priority int}`.
3. Replace all `drivers = append(...)` with `candidates = append(...)`.
4. Implement `rankDrivers(candidates []candidateDriver) []Driver`: sort by priority desc, then root cause first on ties, take ≤ 3.
5. Upgrade hiring driver: compute `hiringRatioY3`, emit `RootCause: true`. Impact = high if ratio worsening, medium if improving.
6. Add cash-contribution driver: use `Cash.Years[0].Operating.Total` to compute payroll fraction of operating outflow. If payroll fraction > 0.60, emit driver with `RootCause: true` **only if the hiring driver was not already added to candidates**. Deduplication rule: the cash-contribution driver is a fallback — it fires when the cash picture shows payroll dominance even though the P&L-ratio hiring threshold was not crossed. Emitting both for the same scenario would duplicate the root cause signal. Check `hiringDriverEmitted bool` flag before appending.
7. Classify `"Inefficient growth model"` as `RootCause: false`.
8. Tests:
   - `TestRankDrivers_MaxThree`
   - `TestRankDrivers_RootCauseBeforeSymptomOnTie`
   - `TestHiringDriver_WorseningTrajectory_HighImpact`
   - `TestHiringDriver_ImprovingTrajectory_MediumImpact`
   - `TestCashContributionDriver_HighPayrollFraction_HiringNotEmitted`
   - `TestCashContributionDriver_Suppressed_WhenHiringAlreadyEmitted` ← deduplication test

**Acceptance:** No scenario returns more than 3 drivers. Root causes precede symptoms in equal-priority groups. Trajectory test distinguishes worsening from improving hiring plans. Cash-contribution driver and hiring driver never appear together.

---

### Sprint 5 — Trend Intelligence Layer
**Goal:** Add `TrendInsights` with six derived scalars.  
**Files:** `scenario_analysis.go`, `scenario_analysis_test.go`

**Tasks:**
1. Add `TrendInsights` struct (§3).
2. Add `Insights TrendInsights` to `ScenarioAnalysisResult`.
3. Implement six pure functions:
   - `computeRevenueCAGR(pnl model.PnlReport) decimal.Decimal`
   - `computeBurnRateMonthly(cash model.CashReport) decimal.Decimal` — average of negative-`Economic.Total[m]` months
   - `computeRevenueVolatility(pnl model.PnlReport) decimal.Decimal` — CV of 5 annual sales values
   - `computeInflectionMonth(cash model.CashReport) int` — first month where `NetCashFlow[m]` turns positive after negative
   - `computeTimeToScale(cash model.CashReport) int` — first month where month-over-month revenue growth accelerates for 2 consecutive months, subject to guards: monthly revenue ≥ `minScaleRevenue` (5,000) AND month-over-month change ≥ `minScaleVariation` (2%) for both qualifying months. Returns 0 if guards not met or no inflection found within 36 months.
   - `computeTrendSignal(insights TrendInsights) string` — see §2.9 classification
4. Compose in `computeInsights(output *model.FullPlanOutput) TrendInsights`.
5. Tests (one per function + edge cases):
   - `TestRevenueCAGR_ZeroBase_ReturnsZero`
   - `TestBurnRate_NoNegativeMonths_ReturnsZero`
   - `TestVolatility_FlatRevenue_ReturnsZero`
   - `TestInflectionMonth_*`
   - `TestTimeToScale_MonotonicAcceleration_ReturnsOne`
   - `TestTimeToScale_NeverAccelerates_ReturnsZero`
   - `TestTimeToScale_RevenueBelowMinThreshold_ReturnsZero` ← guard test
   - `TestTimeToScale_VariationBelowMinThreshold_ReturnsZero` ← guard test
   - `TestTrendSignal_HighVolatility_OverridesImproving`

**Acceptance:** All six fields populated. `TrendSignal = "volatile"` overrides direction signal. `TimeToScale` is distinct from `InflectionMonth` in test that shows they differ.

---

### Sprint 6 — AI-Ready Highlights
**Goal:** Add `Highlights` with varied headlines and derived content only.  
**Files:** `scenario_analysis.go`, `scenario_analysis_test.go`

**Tasks:**
1. Add `Highlights` struct.
2. Implement `buildHighlights(result *ScenarioAnalysisResult) Highlights`:
   - Implement 7-branch headline decision tree (§2.10).
   - `Strengths`: extract from drivers with `impact == "high"` and `RootCause == true` (max 2); supplement with projection signals (`CashMin > 0`, `EbitdaPeak positive`) if fewer than 2 driver-based strengths. **Fallback (new):** if no strengths can be derived at all (no positive drivers, no positive projections), use `[]string{"No dominant positive signal detected — scenario appears balanced but inconclusive."}`. Never return an empty slice.
   - `Weaknesses`: top 2 risks ordered by severity, using `risk.Message`. **Fallback (new):** if `result.Risks` is empty, use `[]string{"No structural risks detected — scenario appears stable under current assumptions."}`. Never return an empty slice.
   - `TopRisks`: top 2 risk messages, severity-ordered. **Fallback (new):** if empty, use `[]string{"No risks flagged — model appears robust at current assumptions."}`.
   - `TopDrivers`: top 2 driver effects, from the ranked list. **Fallback (new):** if `result.Drivers` is empty, use `[]string{"No dominant signal detected — scenario appears balanced but inconclusive."}`. Never return an empty slice.
3. Rule: zero decimal arithmetic inside `buildHighlights`. Assert with a `go vet`-style comment.
4. Tests:
   - `TestHeadline_AllSevenBranches` — one scenario per branch
   - `TestBuildHighlights_MaxTwoStrengths`
   - `TestBuildHighlights_FundingRequiredInWeaknesses_WhenNegativeCash`
   - `TestBuildHighlights_NoRisks_FallbackUsed` ← fallback test
   - `TestBuildHighlights_NoDrivers_FallbackUsed` ← fallback test
   - `TestBuildHighlights_NoStrengths_FallbackUsed` ← fallback test
   - `TestBuildHighlights_NeverCallsDecimalArithmetic` — structural: confirm via function signature inspection

**Acceptance:** Seven distinct headline outputs demonstrated. No `decimal.Decimal` operation inside `buildHighlights`. All four slices (`Strengths`, `Weaknesses`, `TopRisks`, `TopDrivers`) are never empty — fallback strings are always present.

---

### Sprint 7 — Sensitivity Foundation + Fragility
**Goal:** In-memory perturbation engine, typed levers, fragility score, performance limits.  
**Files:** `plan_compute_orchestrator.go`, new `sensitivity_engine.go`, `sensitivity_engine_test.go`

**Tasks:**
1. Define `EconomicLever`, `Perturbation`, `SensitivityResult`, `SensitivityReport` structs (§2.5, §2.6).
2. Implement three perturbation functions (pure, no I/O):
   - `perturbRevenueScale(input compute.FullPlanInput, factor decimal.Decimal) compute.FullPlanInput`
   - `perturbHeadcountScale(input compute.FullPlanInput, factor decimal.Decimal) compute.FullPlanInput`
   - `perturbCOGSScale(input compute.FullPlanInput, factor decimal.Decimal) compute.FullPlanInput`
3. Implement `cloneFullPlanInput(input compute.FullPlanInput) compute.FullPlanInput` — deep copy, no shared slice pointers.
4. Implement `SensitivityEngine.Run`:
   - Validate levers, reject unknown with `apierror.BadRequest`.
   - Cap at `N ≤ 10` perturbations (`maxSensitivityPerturbations = 10` — named constant).
   - Pass `context.Context`; respect named constant `sensitivityTimeout = 15 * time.Second`. This is distinct from the report's `reportTimeout` — they are separate concerns and must not share a constant.
   - Early stop: 3 consecutive zero-delta results on the same lever halt evaluation for that lever.
5. Implement `computeFragility(results []SensitivityResult) (score int, signal string)` — normalised stddev of viability deltas (§2.6).
6. Tests:
   - `TestCloneFullPlanInput_NoAliasedSlices`
   - `TestRevenueScale_PositiveDelta_IncreasesViability`
   - `TestHeadcountScale_2x_DegradesCashHealth`
   - `TestSensitivityRun_UnknownLever_ReturnsError`
   - `TestSensitivityRun_ExceedsMaxN_ReturnsError`
   - `TestSensitivityRun_EarlyStop_After3ZeroDeltas`
   - `TestFragility_HighVariance_IsFragile`
   - `TestFragility_ZeroVariance_IsStable`

**Acceptance:** Zero DB calls in any sensitivity test. Alias test proves clone correctness. Fragility test demonstrates stable vs fragile classification.

---

## 5. File Inventory

| Sprint | New files | Modified files |
|---|---|---|
| 1 | — | `scenario_analysis.go`, `plan_compute_orchestrator.go`, `scenario_analysis_test.go` |
| 2 | — | `scenario_analysis.go`, `scenario_analysis_test.go` |
| 3 | — | `scenario_analysis.go`, `scenario_analysis_test.go` |
| 4 | — | `scenario_analysis.go`, `scenario_analysis_test.go` |
| 5 | — | `scenario_analysis.go`, `scenario_analysis_test.go` |
| 6 | — | `scenario_analysis.go`, `scenario_analysis_test.go` |
| 7 | `sensitivity_engine.go`, `sensitivity_engine_test.go` | `plan_compute_orchestrator.go` |

Estimated: ~900 lines production code, ~550 lines test code.

---

## 6. Pre-Sprint Checklist

- [ ] **Front-end does not hardcode score thresholds** — continuous scoring changes score values
- [ ] **Front-end treats new fields as optional** — `urgency`, `break_even_status`, `break_even_method`, `root_cause` etc. must not trigger `DisallowUnknownFields` errors
- [ ] **`Economic.Total` is populated in real scenarios** — if the WCR section is also empty and Economic.Total is uniformly zero, Sprint 1's cash-economic BEP silently falls through to P&L approximation for all 36 months. Verify with a live integration test before Sprint 1.
- [ ] **Product decision: viability cap-override for `GlobalRiskLevel == "critical"`** — Sprint 3 task 8 requires a yes/no on this before implementation
- [ ] **`compute.FullPlanInput` fields are all exported** — required for `cloneFullPlanInput` in Sprint 7
- [ ] **Sensitivity endpoint out-of-scope** — Sprint 7 builds the engine only; HTTP wiring is a separate sprint

---

## 7. What This Does Not Include

- HTTP endpoint for sensitivity analysis
- Redis caching of sensitivity or analysis results
- AI API integration (`Highlights` feeds the AI; calling the API is application-layer)
- Multi-scenario comparison
- Country-specific anchor-point calibration (constants are designed to be promotable to config)
