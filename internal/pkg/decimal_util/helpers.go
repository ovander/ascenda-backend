package decimal_util

import (
	"github.com/shopspring/decimal"
)

// Standard precision constants for financial calculations.
var (
	DivPrecision  int32 = 10 // Intermediate division precision
	DisplayScale  int32 = 2  // Final rounding for display/storage
	Zero                = decimal.Zero
	One                 = decimal.NewFromInt(1)
	Hundred             = decimal.NewFromInt(100)
	Twelve              = decimal.NewFromInt(12)
	ThreeSixtyFive      = decimal.NewFromInt(365)
)

// Sum returns the sum of a slice of decimals.
func Sum(vals ...decimal.Decimal) decimal.Decimal {
	result := decimal.Zero
	for _, v := range vals {
		result = result.Add(v)
	}
	return result
}

// SumArray returns the sum of a fixed-size decimal array (5 years).
func SumArray(arr [5]decimal.Decimal) decimal.Decimal {
	result := decimal.Zero
	for _, v := range arr {
		result = result.Add(v)
	}
	return result
}

// SumArray6 returns the sum of a 6-element array (initial + 5 years).
func SumArray6(arr [6]decimal.Decimal) decimal.Decimal {
	result := decimal.Zero
	for _, v := range arr {
		result = result.Add(v)
	}
	return result
}

// SumMonthly returns the sum of 12 monthly values.
func SumMonthly(arr [12]decimal.Decimal) decimal.Decimal {
	result := decimal.Zero
	for _, v := range arr {
		result = result.Add(v)
	}
	return result
}

// PctSafe computes (value * pct / 100) safely (returns Zero if pct is zero).
func PctSafe(value, pct decimal.Decimal) decimal.Decimal {
	if pct.IsZero() {
		return decimal.Zero
	}
	return value.Mul(pct).Div(Hundred)
}

// PctOf computes the percentage that `part` is of `total`.
// Returns Zero if total is zero.
func PctOf(part, total decimal.Decimal) decimal.Decimal {
	if total.IsZero() {
		return decimal.Zero
	}
	return part.Div(total).Mul(Hundred)
}

// RoundTo rounds a decimal to the specified number of decimal places using banker's rounding.
func RoundTo(d decimal.Decimal, places int32) decimal.Decimal {
	return d.RoundBank(places)
}

// RoundDisplay rounds to DisplayScale (2 decimal places).
func RoundDisplay(d decimal.Decimal) decimal.Decimal {
	return d.RoundBank(DisplayScale)
}

// DivSafe divides numerator by denominator, returning Zero if denominator is zero.
func DivSafe(num, den decimal.Decimal) decimal.Decimal {
	if den.IsZero() {
		return decimal.Zero
	}
	return num.Div(den)
}

// Max returns the larger of two decimals.
func Max(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThanOrEqual(b) {
		return a
	}
	return b
}

// Min returns the smaller of two decimals.
func Min(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThanOrEqual(b) {
		return a
	}
	return b
}

// IsPositive returns true if d > 0.
func IsPositive(d decimal.Decimal) bool {
	return d.IsPositive()
}

// IsNegative returns true if d < 0.
func IsNegative(d decimal.Decimal) bool {
	return d.IsNegative()
}

// FiscalYearFactor computes the scaling factor when the first fiscal year is < 12 months.
// Returns months/12 as a decimal. If months >= 12, returns 1.
func FiscalYearFactor(months int) decimal.Decimal {
	if months >= 12 {
		return One
	}
	return decimal.NewFromInt(int64(months)).Div(Twelve)
}

// SpreadEvenly distributes an annual value across 12 months (annual / 12).
func SpreadEvenly(annual decimal.Decimal) [12]decimal.Decimal {
	var monthly [12]decimal.Decimal
	m := annual.Div(Twelve)
	for i := 0; i < 12; i++ {
		monthly[i] = m
	}
	return monthly
}

// NewFromString creates a decimal from string, panicking if invalid (use for constants only).
func NewFromString(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

// NewFromInt creates a decimal from an int64.
func NewFromInt(i int64) decimal.Decimal {
	return decimal.NewFromInt(i)
}
