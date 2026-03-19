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

// ComputeNPV calculates Net Present Value using discount rate
func ComputeNPV(cashFlows [MaxYears]decimal.Decimal, discountRate decimal.Decimal) decimal.Decimal {
	npv := decimal.Zero
	for year := 0; year < MaxYears; year++ {
		yearDecimal := decimal.NewFromInt(int64(year))
		// Discount factor = 1 / (1 + rate)^year
		discountFactor := decimal.NewFromInt(1).Add(discountRate)
		discountFactor = discountFactor.Pow(yearDecimal)
		discountFactor = SafeDiv(decimal.NewFromInt(1), discountFactor)

		pv := cashFlows[year].Mul(discountFactor)
		npv = npv.Add(pv)
	}
	return npv
}

// ComputeIRR calculates Internal Rate of Return using Newton's method
func ComputeIRR(cashFlows [MaxYears]decimal.Decimal, initialInvestment decimal.Decimal) decimal.Decimal {
	// Initialize guess at 10% IRR
	rate := ParseDecimal(0.1)
	tolerance := ParseDecimal(0.0001)

	for iteration := 0; iteration < IRRMaxIterations; iteration++ {
		// Calculate NPV at current rate
		npv := initialInvestment.Neg()
		for year := 0; year < MaxYears; year++ {
			yearDecimal := decimal.NewFromInt(int64(year))
			discountFactor := decimal.NewFromInt(1).Add(rate)
			discountFactor = discountFactor.Pow(yearDecimal)
			discountFactor = SafeDiv(decimal.NewFromInt(1), discountFactor)
			npv = npv.Add(cashFlows[year].Mul(discountFactor))
		}

		// Calculate derivative (NPV')
		derivNPV := decimal.Zero
		for year := 1; year < MaxYears; year++ {
			yearDecimal := decimal.NewFromInt(int64(year))
			yMinusOne := yearDecimal.Sub(decimal.NewFromInt(1))
			discountFactor := decimal.NewFromInt(1).Add(rate)
			discountFactor = discountFactor.Pow(yMinusOne)
			discountFactor = SafeDiv(yearDecimal.Neg(), discountFactor)
			derivNPV = derivNPV.Add(cashFlows[year].Mul(discountFactor))
		}

		// Newton's method: rate_new = rate_old - f(rate_old) / f'(rate_old)
		if derivNPV.IsZero() {
			break
		}

		rateNew := rate.Sub(SafeDiv(npv, derivNPV))

		// Check convergence
		if rateNew.Sub(rate).Abs().LessThan(tolerance) {
			return rateNew
		}

		rate = rateNew
	}

	// Clamp IRR to reasonable bounds (-100% to +500%)
	minRate := ParseDecimal(-1.0)
	maxRate := ParseDecimal(5.0)
	if rate.LessThan(minRate) {
		return minRate
	}
	if rate.GreaterThan(maxRate) {
		return maxRate
	}

	return rate
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
