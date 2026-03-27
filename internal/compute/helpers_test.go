package compute

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// d is a decimal literal helper shared by all compute package tests.
func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic("invalid decimal literal in test: " + s)
	}
	return v
}

// assertDecEq compares two decimal.Decimal values for numerical equality.
func assertDecEq(t *testing.T, expected, actual decimal.Decimal, msgAndArgs ...interface{}) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Errorf("Expected %s but got %s. %v", expected.String(), actual.String(), msgAndArgs)
	}
}

// assertDecEqApprox asserts that two decimal values are approximately equal within tolerance.
func assertDecEqApprox(t *testing.T, expected, actual, tolerance decimal.Decimal, message string, args ...interface{}) {
	t.Helper()
	diff := expected.Sub(actual).Abs()
	if diff.GreaterThan(tolerance) {
		t.Errorf("%s: expected %s, got %s (diff: %s, tolerance: %s)",
			message, expected.String(), actual.String(), diff.String(), tolerance.String())
	}
}

func TestSafeDiv(t *testing.T) {
	tests := []struct {
		name     string
		a        decimal.Decimal
		b        decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "normal division",
			a:        decimal.NewFromInt(10),
			b:        decimal.NewFromInt(2),
			expected: decimal.NewFromInt(5),
		},
		{
			name:     "zero divisor returns zero",
			a:        decimal.NewFromInt(10),
			b:        decimal.Zero,
			expected: decimal.Zero,
		},
		{
			name:     "both zero returns zero",
			a:        decimal.Zero,
			b:        decimal.Zero,
			expected: decimal.Zero,
		},
		{
			name:     "result is decimal",
			a:        decimal.NewFromInt(10),
			b:        decimal.NewFromInt(3),
			expected: decimal.NewFromFloat(3.333333),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeDiv(tt.a, tt.b)
			if tt.expected.IsZero() && result.IsZero() {
				assert.True(t, result.IsZero())
			} else {
				assert.True(t, result.Sub(tt.expected).Abs().LessThan(decimal.NewFromFloat(0.0001)))
			}
		})
	}
}

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected decimal.Decimal
	}{
		{
			name:     "positive float",
			input:    123.45,
			expected: decimal.NewFromFloat(123.45),
		},
		{
			name:     "negative float",
			input:    -99.99,
			expected: decimal.NewFromFloat(-99.99),
		},
		{
			name:     "zero",
			input:    0,
			expected: decimal.Zero,
		},
		{
			name:     "integer as float",
			input:    100,
			expected: decimal.NewFromInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDecimal(tt.input)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestNormalizeYear(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		expected int
	}{
		{
			name:     "year 1 converts to index 0",
			year:     1,
			expected: 0,
		},
		{
			name:     "year 5 converts to index 4",
			year:     5,
			expected: 4,
		},
		{
			name:     "year 0 returns 0",
			year:     0,
			expected: 0,
		},
		{
			name:     "negative year returns 0",
			year:     -1,
			expected: 0,
		},
		{
			name:     "year 3 converts to index 2",
			year:     3,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeYear(tt.year)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeNPV(t *testing.T) {
	tests := []struct {
		name         string
		cashFlows    [5]decimal.Decimal
		discountRate decimal.Decimal
		expected     decimal.Decimal
	}{
		{
			name: "positive cash flows",
			cashFlows: [5]decimal.Decimal{
				decimal.NewFromInt(1000),
				decimal.NewFromInt(1000),
				decimal.NewFromInt(1000),
				decimal.NewFromInt(1000),
				decimal.NewFromInt(1000),
			},
			discountRate: decimal.NewFromFloat(0.10),
			// 1000/1.1 + 1000/1.21 + 1000/1.331 + 1000/1.4641 + 1000/1.61051 = 3790.79
			expected: decimal.NewFromFloat(3790.79),
		},
		{
			name: "zero cash flows",
			cashFlows: [5]decimal.Decimal{
				decimal.Zero,
				decimal.Zero,
				decimal.Zero,
				decimal.Zero,
				decimal.Zero,
			},
			discountRate: decimal.NewFromFloat(0.10),
			expected:     decimal.Zero,
		},
		{
			name: "mixed cash flows",
			cashFlows: [5]decimal.Decimal{
				decimal.NewFromInt(500),
				decimal.NewFromInt(1000),
				decimal.NewFromInt(1500),
				decimal.NewFromInt(2000),
				decimal.NewFromInt(2500),
			},
			discountRate: decimal.NewFromFloat(0.05),
			// 500/1.05 + 1000/1.1025 + 1500/1.157625 + 2000/1.21550625 + 2500/1.2762815... = 6282.95
			expected: decimal.NewFromFloat(6282.95),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeNPV(tt.cashFlows, tt.discountRate)
			// Allow tolerance due to floating point precision
			tolerance := decimal.NewFromFloat(1.0)
			diff := result.Sub(tt.expected).Abs()
			assert.True(t, diff.LessThan(tolerance), "NPV mismatch: got %v, expected %v", result, tt.expected)
		})
	}
}

func TestComputeIRR(t *testing.T) {
	tests := []struct {
		name              string
		cashFlows         [5]decimal.Decimal
		initialInvestment decimal.Decimal
		expectedRange     [2]float64 // min, max
	}{
		{
			name: "positive IRR scenario",
			cashFlows: [5]decimal.Decimal{
				decimal.NewFromInt(500),
				decimal.NewFromInt(500),
				decimal.NewFromInt(500),
				decimal.NewFromInt(500),
				decimal.NewFromInt(500),
			},
			initialInvestment: decimal.NewFromInt(1000),
			expectedRange:     [2]float64{0.05, 1.5}, // IRR should be between 5% and 150%
		},
		{
			name: "zero initial investment",
			cashFlows: [5]decimal.Decimal{
				decimal.NewFromInt(100),
				decimal.NewFromInt(100),
				decimal.NewFromInt(100),
				decimal.NewFromInt(100),
				decimal.NewFromInt(100),
			},
			initialInvestment: decimal.Zero,
			expectedRange:     [2]float64{-1.0, 5.0}, // Should clamp to reasonable bounds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := ComputeIRR(tt.cashFlows, tt.initialInvestment)
			resultFloat, _ := result.Float64()
			assert.True(t,
				resultFloat >= tt.expectedRange[0] && resultFloat <= tt.expectedRange[1],
				"IRR %v not in expected range [%v, %v]", resultFloat, tt.expectedRange[0], tt.expectedRange[1])
		})
	}
}

func TestRoundDecimal(t *testing.T) {
	tests := []struct {
		name     string
		value    decimal.Decimal
		places   int32
		expected decimal.Decimal
	}{
		{
			name:     "round to 2 places",
			value:    decimal.NewFromFloat(123.456),
			places:   2,
			expected: decimal.NewFromFloat(123.46),
		},
		{
			name:     "round to 0 places",
			value:    decimal.NewFromFloat(123.5),
			places:   0,
			expected: decimal.NewFromFloat(124),
		},
		{
			name:     "already rounded",
			value:    decimal.NewFromFloat(100.00),
			places:   2,
			expected: decimal.NewFromFloat(100.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundDecimal(tt.value, tt.places)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestSignDecimal(t *testing.T) {
	tests := []struct {
		name     string
		value    decimal.Decimal
		expected int
	}{
		{
			name:     "positive number",
			value:    decimal.NewFromInt(100),
			expected: 1,
		},
		{
			name:     "negative number",
			value:    decimal.NewFromInt(-100),
			expected: -1,
		},
		{
			name:     "zero",
			value:    decimal.Zero,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SignDecimal(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaxDecimal(t *testing.T) {
	tests := []struct {
		name     string
		a        decimal.Decimal
		b        decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "first is larger",
			a:        decimal.NewFromInt(100),
			b:        decimal.NewFromInt(50),
			expected: decimal.NewFromInt(100),
		},
		{
			name:     "second is larger",
			a:        decimal.NewFromInt(50),
			b:        decimal.NewFromInt(100),
			expected: decimal.NewFromInt(100),
		},
		{
			name:     "equal values",
			a:        decimal.NewFromInt(100),
			b:        decimal.NewFromInt(100),
			expected: decimal.NewFromInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaxDecimal(tt.a, tt.b)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestMinDecimal(t *testing.T) {
	tests := []struct {
		name     string
		a        decimal.Decimal
		b        decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "first is smaller",
			a:        decimal.NewFromInt(50),
			b:        decimal.NewFromInt(100),
			expected: decimal.NewFromInt(50),
		},
		{
			name:     "second is smaller",
			a:        decimal.NewFromInt(100),
			b:        decimal.NewFromInt(50),
			expected: decimal.NewFromInt(50),
		},
		{
			name:     "equal values",
			a:        decimal.NewFromInt(100),
			b:        decimal.NewFromInt(100),
			expected: decimal.NewFromInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinDecimal(tt.a, tt.b)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		name     string
		value    decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "positive number",
			value:    decimal.NewFromInt(100),
			expected: decimal.NewFromInt(100),
		},
		{
			name:     "negative number",
			value:    decimal.NewFromInt(-100),
			expected: decimal.NewFromInt(100),
		},
		{
			name:     "zero",
			value:    decimal.Zero,
			expected: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Abs(tt.value)
			assertDecEq(t, tt.expected, result)
		})
	}
}
