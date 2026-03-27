package compute

import (
	"math"

	"github.com/shopspring/decimal"
)

// SafeDiv returns a/b or zero if b is zero
func SafeDiv(a, b decimal.Decimal) decimal.Decimal {
	if b.IsZero() {
		return decimal.Zero
	}
	return a.Div(b)
}

// ParseDecimal converts float64 to decimal.Decimal
func ParseDecimal(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}

// NormalizeYear converts 1-based year to 0-based index
func NormalizeYear(year int) int {
	if year < 1 {
		return 0
	}
	return year - 1
}

// ComputeNPV calculates Net Present Value using discount rate.
// cashFlows[0..4] represent annual cash flows for periods 1..5.
// Each flow is discounted by 1/(1+r)^(year+1) so the first cash flow
// is one period out (standard end-of-period convention).
func ComputeNPV(cashFlows [MaxYears]decimal.Decimal, discountRate decimal.Decimal) decimal.Decimal {
	npv := decimal.Zero
	for year := 0; year < MaxYears; year++ {
		// period = year+1 so cashFlows[0] is discounted by (1+r)^1, etc.
		period := decimal.NewFromInt(int64(year + 1))
		discountFactor := decimal.NewFromInt(1).Add(discountRate)
		discountFactor = discountFactor.Pow(period)
		discountFactor = SafeDiv(decimal.NewFromInt(1), discountFactor)

		pv := cashFlows[year].Mul(discountFactor)
		npv = npv.Add(pv)
	}
	return npv
}

// ComputeIRR calculates the Internal Rate of Return using Newton's method.
// cashFlows[0..4] are annual flows for periods 1..5 (same convention as ComputeNPV).
// initialInvestment is the positive capital outlay at period 0 (negated inside).
// Returns (irr, converged). If Newton's method diverges or the result exceeds
// the clamp bounds the second return value is false.
func ComputeIRR(cashFlows [MaxYears]decimal.Decimal, initialInvestment decimal.Decimal) (decimal.Decimal, bool) {
	rate := ParseDecimal(0.1) // initial guess 10%
	tolerance := ParseDecimal(0.0001)
	converged := false

	for iteration := 0; iteration < IRRMaxIterations; iteration++ {
		// f(r) = -C0 + Σ CF[y]/(1+r)^(y+1)   for y = 0..4
		npv := initialInvestment.Neg()
		for year := 0; year < MaxYears; year++ {
			period := decimal.NewFromInt(int64(year + 1))
			onePlusR := decimal.NewFromInt(1).Add(rate)
			df := SafeDiv(decimal.NewFromInt(1), onePlusR.Pow(period))
			npv = npv.Add(cashFlows[year].Mul(df))
		}

		// f'(r) = -Σ (y+1)·CF[y]/(1+r)^(y+2)   for y = 0..4
		derivNPV := decimal.Zero
		for year := 0; year < MaxYears; year++ {
			period := decimal.NewFromInt(int64(year + 1))
			power := decimal.NewFromInt(int64(year + 2))
			onePlusR := decimal.NewFromInt(1).Add(rate)
			df := SafeDiv(period.Neg(), onePlusR.Pow(power))
			derivNPV = derivNPV.Add(cashFlows[year].Mul(df))
		}

		if derivNPV.IsZero() {
			break
		}

		rateNew := rate.Sub(SafeDiv(npv, derivNPV))

		if rateNew.Sub(rate).Abs().LessThan(tolerance) {
			rate = rateNew
			converged = true
			break
		}

		rate = rateNew
	}

	// Clamp to reasonable bounds; return false if clamping was needed
	minRate := ParseDecimal(-1.0)
	maxRate := ParseDecimal(5.0)
	if rate.LessThan(minRate) {
		return minRate, false
	}
	if rate.GreaterThan(maxRate) {
		return maxRate, false
	}

	return rate, converged
}

// RoundDecimal rounds a decimal to n places
func RoundDecimal(d decimal.Decimal, places int32) decimal.Decimal {
	return d.Round(places)
}

// MustDecimal converts float64 to decimal, panics on error
func MustDecimal(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}

// SignDecimal returns -1, 0, or 1 based on sign
func SignDecimal(d decimal.Decimal) int {
	if d.IsNegative() {
		return -1
	}
	if d.IsZero() {
		return 0
	}
	return 1
}

// MaxDecimal returns the maximum of two decimals
func MaxDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

// MinDecimal returns the minimum of two decimals
func MinDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

// DecimalToFloat64 converts decimal to float64 for math functions
func DecimalToFloat64(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

// Abs returns absolute value of decimal
func Abs(d decimal.Decimal) decimal.Decimal {
	return d.Abs()
}

// PowDecimal raises a decimal to a power (float exponent)
func PowDecimal(base decimal.Decimal, exponent float64) decimal.Decimal {
	f := DecimalToFloat64(base)
	result := math.Pow(f, exponent)
	return ParseDecimal(result)
}
