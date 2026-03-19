package compute

// Domain-wide constants that replace magic numbers throughout the compute layer.
const (
	// MaxYears is the forecast horizon (5-year plan).
	MaxYears = 5

	// MaxZones is the number of geographic sales zones per product.
	MaxZones = 3

	// MonthsPerYear is the number of months in a fiscal year.
	MonthsPerYear = 12

	// QuartersPerYear is the number of quarters in a fiscal year.
	QuartersPerYear = 4

	// BSheetPeriods is the number of balance-sheet columns: opening (year 0)
	// plus one per forecast year. Equal to MaxYears + 1.
	BSheetPeriods = MaxYears + 1

	// CapexCategories is the number of predefined capex asset categories.
	CapexCategories = 14

	// DefaultDepreciationYears is the default useful life for assets without
	// an explicit depreciation schedule.
	DefaultDepreciationYears = 5

	// IRRMaxIterations is the maximum number of Newton-method iterations
	// when computing IRR.
	IRRMaxIterations = 100
)
