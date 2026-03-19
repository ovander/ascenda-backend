package locale

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		name     string
		value    decimal.Decimal
		locale   string
		expected string
	}{
		{
			name:     "US format with thousand separators",
			value:    decimal.NewFromInt(1234567),
			locale:   "en_US",
			expected: "1,234,567",
		},
		{
			name:     "French format with space separators",
			value:    decimal.NewFromInt(1234567),
			locale:   "fr_FR",
			expected: "1 234 567",
		},
		{
			name:     "German format with space separators",
			value:    decimal.NewFromInt(1234567),
			locale:   "de_DE",
			expected: "1 234 567",
		},
		{
			name:     "US format with decimals",
			value:    decimal.NewFromFloat(1234.56),
			locale:   "en_US",
			expected: "1,234.56",
		},
		{
			name:     "French format with decimals (comma separator)",
			value:    decimal.NewFromFloat(1234.56),
			locale:   "fr_FR",
			expected: "1 234,56",
		},
		{
			name:     "negative number US",
			value:    decimal.NewFromInt(-1234567),
			locale:   "en_US",
			expected: "-1,234,567",
		},
		{
			name:     "negative number French",
			value:    decimal.NewFromInt(-1234567),
			locale:   "fr_FR",
			expected: "-1 234 567",
		},
		{
			name:     "small number",
			value:    decimal.NewFromInt(123),
			locale:   "en_US",
			expected: "123",
		},
		{
			name:     "zero",
			value:    decimal.Zero,
			locale:   "en_US",
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDecimal(tt.value, tt.locale)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		name     string
		value    decimal.Decimal
		currency string
		locale   string
		expected string
	}{
		{
			name:     "EUR in US locale",
			value:    decimal.NewFromInt(1000),
			currency: "EUR",
			locale:   "en_US",
			expected: "€ 1,000",
		},
		{
			name:     "EUR in French locale",
			value:    decimal.NewFromInt(1000),
			currency: "EUR",
			locale:   "fr_FR",
			expected: "1 000 €",
		},
		{
			name:     "USD in US locale",
			value:    decimal.NewFromInt(2500),
			currency: "USD",
			locale:   "en_US",
			expected: "$ 2,500",
		},
		{
			name:     "GBP in US locale",
			value:    decimal.NewFromInt(1500),
			currency: "GBP",
			locale:   "en_US",
			expected: "£ 1,500",
		},
		{
			name:     "EUR with decimals in French",
			value:    decimal.NewFromFloat(1234.56),
			currency: "EUR",
			locale:   "fr_FR",
			expected: "1 234,56 €",
		},
		{
			name:     "CHF currency",
			value:    decimal.NewFromInt(5000),
			currency: "CHF",
			locale:   "en_US",
			expected: "CHF 5,000",
		},
		{
			name:     "JPY currency",
			value:    decimal.NewFromInt(100000),
			currency: "JPY",
			locale:   "en_US",
			expected: "¥ 100,000",
		},
		{
			name:     "negative currency",
			value:    decimal.NewFromInt(-1000),
			currency: "EUR",
			locale:   "en_US",
			expected: "€ -1,000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCurrency(tt.value, tt.currency, tt.locale)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddThousandSeparator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		locale   string
		expected string
	}{
		{
			name:     "US format",
			input:    "1234567",
			locale:   "en_US",
			expected: "1,234,567",
		},
		{
			name:     "French format",
			input:    "1234567",
			locale:   "fr_FR",
			expected: "1 234 567",
		},
		{
			name:     "small number",
			input:    "123",
			locale:   "en_US",
			expected: "123",
		},
		{
			name:     "single digit",
			input:    "5",
			locale:   "en_US",
			expected: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addThousandSeparator(tt.input, tt.locale)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCurrencySymbol(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		expected string
	}{
		{
			name:     "EUR",
			currency: "EUR",
			expected: "€",
		},
		{
			name:     "USD",
			currency: "USD",
			expected: "$",
		},
		{
			name:     "GBP",
			currency: "GBP",
			expected: "£",
		},
		{
			name:     "CHF",
			currency: "CHF",
			expected: "CHF",
		},
		{
			name:     "JPY",
			currency: "JPY",
			expected: "¥",
		},
		{
			name:     "unknown currency defaults to code",
			currency: "XYZ",
			expected: "XYZ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCurrencySymbol(tt.currency)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLocaleVariations(t *testing.T) {
	value := decimal.NewFromFloat(12345.67)

	tests := []struct {
		name     string
		locale   string
		expectedContains string
	}{
		{
			name:     "US uses comma",
			locale:   "en_US",
			expectedContains: "12,345.67",
		},
		{
			name:     "French uses space and comma",
			locale:   "fr_FR",
			expectedContains: "12 345,67",
		},
		{
			name:     "German uses space and comma",
			locale:   "de_DE",
			expectedContains: "12 345,67",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDecimal(value, tt.locale)
			assert.Contains(t, result, tt.expectedContains)
		})
	}
}
