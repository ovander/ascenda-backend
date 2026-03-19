package locale

import (
	"strings"

	"github.com/shopspring/decimal"
)

// FormatDecimal formats a decimal number with thousand separators based on locale.
func FormatDecimal(d decimal.Decimal, locale string) string {
	str := d.String()

	// Parse sign
	sign := ""
	if strings.HasPrefix(str, "-") {
		sign = "-"
		str = str[1:]
	}

	// Split into integer and decimal parts
	parts := strings.Split(str, ".")
	integerPart := parts[0]
	decimalPart := ""
	if len(parts) > 1 {
		decimalPart = parts[1]
	}

	// Add thousand separators to integer part
	integerPart = addThousandSeparator(integerPart, locale)

	// Determine decimal separator
	decimalSeparator := "."
	if locale == "fr_FR" || locale == "de_DE" {
		decimalSeparator = ","
	}

	// Build result
	result := sign + integerPart
	if decimalPart != "" {
		result += decimalSeparator + decimalPart
	}

	return result
}

// FormatCurrency formats a decimal as currency with locale-specific formatting.
func FormatCurrency(d decimal.Decimal, currency, locale string) string {
	formatted := FormatDecimal(d, locale)

	// Determine currency symbol and position
	symbol := getCurrencySymbol(currency)
	var result string

	if locale == "fr_FR" || locale == "de_DE" {
		// European format: 1 234,50 EUR
		result = formatted + " " + symbol
	} else {
		// US format: EUR 1,234.50
		result = symbol + " " + formatted
	}

	return result
}

// addThousandSeparator adds thousand separators to a numeric string.
func addThousandSeparator(s string, locale string) string {
	separator := ","
	if locale == "fr_FR" || locale == "de_DE" {
		separator = " "
	}

	// Reverse string
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	// Add separators every 3 digits
	var result []rune
	for i, r := range runes {
		if i > 0 && i%3 == 0 {
			result = append(result, []rune(separator)...)
		}
		result = append(result, r)
	}

	// Reverse back
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// getCurrencySymbol returns the symbol for a currency code.
func getCurrencySymbol(currency string) string {
	symbols := map[string]string{
		"EUR": "€",
		"USD": "$",
		"GBP": "£",
		"CHF": "CHF",
		"JPY": "¥",
	}

	if symbol, ok := symbols[currency]; ok {
		return symbol
	}

	return currency
}
