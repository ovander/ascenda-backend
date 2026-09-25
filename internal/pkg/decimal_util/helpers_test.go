package decimal_util

import (
	"testing"

	"github.com/shopspring/decimal"
)

func assertDecEq(t *testing.T, expected, actual decimal.Decimal, msgAndArgs ...interface{}) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Errorf("Expected %s but got %s. %v", expected.String(), actual.String(), msgAndArgs)
	}
}

func TestSum(t *testing.T) {
	tests := []struct {
		name     string
		values   []decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "sum of positive numbers",
			values:   []decimal.Decimal{decimal.NewFromInt(10), decimal.NewFromInt(20), decimal.NewFromInt(30)},
			expected: decimal.NewFromInt(60),
		},
		{
			name:     "sum with negatives",
			values:   []decimal.Decimal{decimal.NewFromInt(100), decimal.NewFromInt(-50), decimal.NewFromInt(25)},
			expected: decimal.NewFromInt(75),
		},
		{
			name:     "empty list returns zero",
			values:   []decimal.Decimal{},
			expected: decimal.Zero,
		},
		{
			name:     "single value",
			values:   []decimal.Decimal{decimal.NewFromInt(100)},
			expected: decimal.NewFromInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sum(tt.values...)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestSumArray(t *testing.T) {
	tests := []struct {
		name     string
		arr      [5]decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name: "sum of 5-element array",
			arr: [5]decimal.Decimal{
				decimal.NewFromInt(10), decimal.NewFromInt(20), decimal.NewFromInt(30),
				decimal.NewFromInt(40), decimal.NewFromInt(50),
			},
			expected: decimal.NewFromInt(150),
		},
		{
			name: "array with zeros",
			arr: [5]decimal.Decimal{
				decimal.NewFromInt(100), decimal.Zero, decimal.NewFromInt(50),
				decimal.Zero, decimal.NewFromInt(25),
			},
			expected: decimal.NewFromInt(175),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SumArray(tt.arr)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestPctSafe(t *testing.T) {
	tests := []struct {
		name     string
		value    decimal.Decimal
		pct      decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "10% of 100",
			value:    decimal.NewFromInt(100),
			pct:      decimal.NewFromInt(10),
			expected: decimal.NewFromInt(10),
		},
		{
			name:     "25% of 1000",
			value:    decimal.NewFromInt(1000),
			pct:      decimal.NewFromInt(25),
			expected: decimal.NewFromInt(250),
		},
		{
			name:     "zero percentage returns zero",
			value:    decimal.NewFromInt(1000),
			pct:      decimal.Zero,
			expected: decimal.Zero,
		},
		{
			name:     "decimal percentage",
			value:    decimal.NewFromInt(1000),
			pct:      decimal.NewFromFloat(0.5), // 0.5%
			expected: decimal.NewFromFloat(5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PctSafe(tt.value, tt.pct)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestPctOf(t *testing.T) {
	tests := []struct {
		name     string
		part     decimal.Decimal
		total    decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "50 is 50% of 100",
			part:     decimal.NewFromInt(50),
			total:    decimal.NewFromInt(100),
			expected: decimal.NewFromInt(50),
		},
		{
			name:     "25 is 25% of 100",
			part:     decimal.NewFromInt(25),
			total:    decimal.NewFromInt(100),
			expected: decimal.NewFromInt(25),
		},
		{
			name:     "zero total returns zero",
			part:     decimal.NewFromInt(50),
			total:    decimal.Zero,
			expected: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PctOf(tt.part, tt.total)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestRoundTo(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundTo(tt.value, tt.places)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestDivSafe(t *testing.T) {
	tests := []struct {
		name     string
		num      decimal.Decimal
		den      decimal.Decimal
		expected decimal.Decimal
	}{
		{
			name:     "normal division",
			num:      decimal.NewFromInt(100),
			den:      decimal.NewFromInt(2),
			expected: decimal.NewFromInt(50),
		},
		{
			name:     "zero denominator returns zero",
			num:      decimal.NewFromInt(100),
			den:      decimal.Zero,
			expected: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DivSafe(tt.num, tt.den)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestMax(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Max(tt.a, tt.b)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestMin(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Min(tt.a, tt.b)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestFiscalYearFactor(t *testing.T) {
	tests := []struct {
		name     string
		months   int
		expected decimal.Decimal
	}{
		{
			name:     "full year (12 months)",
			months:   12,
			expected: One,
		},
		{
			name:     "6 months",
			months:   6,
			expected: decimal.NewFromFloat(0.5),
		},
		{
			name:     "3 months",
			months:   3,
			expected: decimal.NewFromFloat(0.25),
		},
		{
			name:     "more than 12 months returns 1",
			months:   18,
			expected: One,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FiscalYearFactor(tt.months)
			assertDecEq(t, tt.expected, result)
		})
	}
}

func TestSpreadEvenly(t *testing.T) {
	t.Run("spread 12000 evenly across 12 months", func(t *testing.T) {
		annual := decimal.NewFromInt(12000)
		result := SpreadEvenly(annual)

		for i := 0; i < 12; i++ {
			assertDecEq(t, decimal.NewFromInt(1000), result[i])
		}
	})
}
