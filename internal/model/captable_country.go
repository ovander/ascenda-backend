package model

// captable_country.go — country-aware defaults for the Cap Table Evolution module.
//
// Follows the same pattern as country.go / PlanConfig.Country: the scenario's
// PlanConfig.Country code is the single source of truth. The cap table module
// reads its default legal forms, equity instruments, nominal values, and
// terminology from the profile returned by GetCapTableCountryProfile(code).
//
// Fields that users have overridden in CapTableCompany are always respected;
// these profiles are used only when creating a new CapTableCompany (wizard
// auto-fill) or when the frontend needs to display the available options.

// ────────────────────────────────────────────────────────────────────────────
// Equity Instrument Availability
// ────────────────────────────────────────────────────────────────────────────

// EquityInstrumentDef describes one equity instrument available in a country.
type EquityInstrumentDef struct {
	Code           StockOptionInstrument `json:"code"`
	LocalName      string                `json:"localName"`      // name in the local language
	EnglishName    string                `json:"englishName"`
	Description    string                `json:"description"`    // short plain-text note
	EligibilityNote string               `json:"eligibilityNote,omitempty"` // legal constraints
	IsTaxAdvantaged bool                 `json:"isTaxAdvantaged"`
	MaxAgeLimitYears *int                `json:"maxAgeLimitYears,omitempty"` // e.g. BSPCE ≤ 15 yrs
}

// ────────────────────────────────────────────────────────────────────────────
// Legal Form Availability
// ────────────────────────────────────────────────────────────────────────────

// LegalFormDef describes one legal form available in a country.
type LegalFormDef struct {
	Code        string `json:"code"`        // e.g. "SAS", "Ltd", "GmbH"
	LocalName   string `json:"localName"`
	EnglishName string `json:"englishName"`
	IsCommon    bool   `json:"isCommon"` // show first / pre-selected
}

// ────────────────────────────────────────────────────────────────────────────
// Cap Table Country Profile
// ────────────────────────────────────────────────────────────────────────────

// CapTableCountryProfile holds all equity-module defaults for one country.
// It is derived from PlanConfig.Country and used to auto-fill CapTableCompany
// fields and restrict instrument / legal-form selectors in the frontend.
type CapTableCountryProfile struct {
	// Mirrors CountryConfig identity
	CountryCode     string `json:"countryCode"`
	CountryName     string `json:"countryName"`
	Currency        string `json:"currency"`
	CurrencySymbol  string `json:"currencySymbol"`
	DateFormat      string `json:"dateFormat"`
	PrimaryLanguage string `json:"primaryLanguage"`

	// Equity-specific defaults
	DefaultNominalValueCents int64  `json:"defaultNominalValueCents"` // default share nominal
	DefaultLegalForm         string `json:"defaultLegalForm"`         // pre-selected in wizard
	BookEquityTerm           string `json:"bookEquityTerm"`           // local label for capitaux propres
	ShareCapitalTerm         string `json:"shareCapitalTerm"`         // local label for capital social

	// Available legal forms (ordered: common first)
	LegalForms []LegalFormDef `json:"legalForms"`

	// Available equity instruments
	Instruments []EquityInstrumentDef `json:"instruments"`

	// Guidance notes shown in the UI
	NominalValueNote string `json:"nominalValueNote,omitempty"`
	ESOPNote         string `json:"esopNote,omitempty"`
}

// ────────────────────────────────────────────────────────────────────────────
// Profile Registry
// ────────────────────────────────────────────────────────────────────────────

// GetCapTableCountryProfile returns the equity defaults for the given ISO country code.
// Falls back to Belgium if the code is not registered.
func GetCapTableCountryProfile(code string) CapTableCountryProfile {
	if p, ok := capTableCountryProfiles[code]; ok {
		return p
	}
	return capTableCountryProfiles["BE"]
}

// AllCapTableCountryCodes returns all country codes that have a registered profile.
func AllCapTableCountryCodes() []string {
	codes := make([]string, 0, len(capTableCountryProfiles))
	for k := range capTableCountryProfiles {
		codes = append(codes, k)
	}
	return codes
}

// ptr is a local helper for optional int fields.
func capTableIntPtr(i int) *int { return &i }

// ────────────────────────────────────────────────────────────────────────────
// Per-country profiles
// ────────────────────────────────────────────────────────────────────────────

var capTableCountryProfiles = map[string]CapTableCountryProfile{

	// ── France ─────────────────────────────────────────────────────────────
	"FR": {
		CountryCode:              "FR",
		CountryName:              "France",
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "fr",
		DefaultNominalValueCents: 1,      // €0.01 — most common for SAS/SARL startups
		DefaultLegalForm:         "SAS",
		BookEquityTerm:           "Capitaux propres",
		ShareCapitalTerm:         "Capital social",
		NominalValueNote:         "La valeur nominale peut être librement fixée. €0,01 est la valeur la plus courante pour les startups.",
		ESOPNote:                 "Les BSPCE sont réservés aux sociétés de moins de 15 ans soumises à l'IS et détenues à moins de 25% par des personnes morales.",
		LegalForms: []LegalFormDef{
			{"SAS", "Société par Actions Simplifiée", "Simplified Joint-Stock Company", true},
			{"SARL", "Société à Responsabilité Limitée", "Limited Liability Company", true},
			{"SA", "Société Anonyme", "Public Limited Company", false},
			{"SCA", "Société en Commandite par Actions", "Limited Partnership with Shares", false},
			{"SNC", "Société en Nom Collectif", "General Partnership", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIBSPCE,
				LocalName:       "BSPCE — Bons de Souscription de Parts de Créateur d'Entreprise",
				EnglishName:     "Founder Warrant (BSPCE)",
				Description:     "Tax-advantaged warrants for employees/managers of qualifying young companies.",
				EligibilityNote: "Company < 15 years old, IS taxpayer, < 25% held by non-natural-person corps.",
				IsTaxAdvantaged: true,
				MaxAgeLimitYears: capTableIntPtr(15),
			},
			{
				Code:            SOIBCE,
				LocalName:       "BCE — Bons de Créateur d'Entreprise",
				EnglishName:     "Entrepreneur Warrant (BCE)",
				Description:     "Warrants for employees/managers; broader eligibility than BSPCE.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "BSA — Bons de Souscription d'Actions",
				EnglishName:     "Share Subscription Warrant (BSA)",
				Description:     "Warrants for any beneficiary (including non-employees); no tax advantage.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIStockOption,
				LocalName:       "Options de souscription d'actions",
				EnglishName:     "Stock Option",
				Description:     "Classic stock options; taxed as salary on exercise for employees.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "AGA — Actions Gratuites Attribuées",
				EnglishName:     "Free Share / RSU",
				Description:     "Free share allocation (AGA); favourable tax regime if holding period respected.",
				IsTaxAdvantaged: true,
			},
		},
	},

	// ── Belgium ────────────────────────────────────────────────────────────
	"BE": {
		CountryCode:              "BE",
		CountryName:              "Belgium",
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "fr",
		DefaultNominalValueCents: 1,
		DefaultLegalForm:         "SRL",
		BookEquityTerm:           "Capitaux propres / Eigen vermogen",
		ShareCapitalTerm:         "Capital social / Maatschappelijk kapitaal",
		NominalValueNote:         "La BV/SRL n'a plus de capital minimum légal depuis le CSA 2019. La SA/NV requiert €61 500.",
		ESOPNote:                 "Les warrants (bons de souscription) sont l'instrument le plus courant pour les stock options en Belgique.",
		LegalForms: []LegalFormDef{
			{"SRL", "Société à Responsabilité Limitée / BV", "Private Limited Company (BV/SRL)", true},
			{"SA", "Société Anonyme / NV", "Public Limited Company (SA/NV)", true},
			{"SNC", "Société en Nom Collectif / VOF", "General Partnership", false},
			{"SCS", "Société en Commandite Simple / GCV", "Limited Partnership", false},
			{"SC", "Société Coopérative / CV", "Cooperative Company", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Warrant (Bon de souscription / Inschrijvingsrecht)",
				EnglishName:     "Share Subscription Warrant",
				Description:     "Most common equity incentive instrument in Belgium. Favourable flat tax on grant if conditions met.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIStockOption,
				LocalName:       "Option sur actions / Aandelenoptie",
				EnglishName:     "Stock Option",
				Description:     "Stock option plan under the Belgian Stock Option Law of 1999.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIRSU,
				LocalName:       "Actions gratuites / Gratis aandelen",
				EnglishName:     "Free Share / RSU",
				Description:     "Free share allocation; taxed as benefit in kind at grant.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Netherlands ────────────────────────────────────────────────────────
	"NL": {
		CountryCode:              "NL",
		CountryName:              "Netherlands",
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "nl",
		DefaultNominalValueCents: 1, // €0.01
		DefaultLegalForm:         "BV",
		BookEquityTerm:           "Eigen vermogen",
		ShareCapitalTerm:         "Aandelenkapitaal",
		NominalValueNote:         "BV minimum kapitaal is €0,01. De nominale waarde is vrij te kiezen.",
		LegalForms: []LegalFormDef{
			{"BV", "Besloten Vennootschap", "Private Limited Company (BV)", true},
			{"NV", "Naamloze Vennootschap", "Public Limited Company (NV)", false},
			{"Coöperatie", "Coöperatie", "Cooperative", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "Aandelenoptie",
				EnglishName:     "Stock Option",
				Description:     "Taxed as employment income at exercise; 30% ruling may apply for expats.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "Restricted Stock Unit (RSU)",
				EnglishName:     "Restricted Stock Unit",
				Description:     "Taxed as employment income at vesting.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Warrant / Optierecht",
				EnglishName:     "Warrant",
				Description:     "Common in VC-backed rounds.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Germany ────────────────────────────────────────────────────────────
	"DE": {
		CountryCode:              "DE",
		CountryName:              "Germany",
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "de",
		DefaultNominalValueCents: 100, // GmbH: min €1 per Anteil; AG: min €1 per share
		DefaultLegalForm:         "GmbH",
		BookEquityTerm:           "Eigenkapital",
		ShareCapitalTerm:         "Stammkapital",
		NominalValueNote:         "GmbH: Mindestbetrag €1 je Anteil, Mindestkapital €25 000. AG: Nennwert mindestens €1 je Aktie.",
		ESOPNote:                 "GmbH-Anteile sind nicht frei übertragbar (Notarpflicht). Phantom-Optionen und virtuelle Beteiligungen (VSOP) sind in der Praxis die häufigsten Instrumente.",
		LegalForms: []LegalFormDef{
			{"GmbH", "Gesellschaft mit beschränkter Haftung", "Private Limited Company (GmbH)", true},
			{"AG", "Aktiengesellschaft", "Public Limited Company (AG)", true},
			{"UG", "Unternehmergesellschaft (haftungsbeschränkt)", "Mini-GmbH (UG)", false},
			{"KGaA", "Kommanditgesellschaft auf Aktien", "Partnership Limited by Shares", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "Virtuelle Beteiligung / VSOP (Virtual Stock Option Plan)",
				EnglishName:     "Virtual Stock Option (Phantom)",
				Description:     "Most common for GmbH. Cash-settled phantom plan avoids notarial share transfer requirements.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Wandelschuldverschreibung / Option",
				EnglishName:     "Warrant / Convertible",
				Description:     "Used mainly in AG structures or at the point of conversion to AG.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "Restricted Stock Unit (RSU)",
				EnglishName:     "Restricted Stock Unit",
				Description:     "Taxed as employment income at vesting.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── United Kingdom ─────────────────────────────────────────────────────
	"GB": {
		CountryCode:              "GB",
		CountryName:              "United Kingdom",
		Currency:                 "GBP",
		CurrencySymbol:           "£",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "en",
		DefaultNominalValueCents: 1, // 1p (£0.01) is standard
		DefaultLegalForm:         "Ltd",
		BookEquityTerm:           "Shareholders' equity",
		ShareCapitalTerm:         "Share capital",
		NominalValueNote:         "1p (£0.01) par value is the most common for UK Ltd companies.",
		ESOPNote:                 "EMI is the most tax-efficient option scheme for qualifying companies (< £30M gross assets, < 250 FTE employees). CSOP applies where EMI is not available.",
		LegalForms: []LegalFormDef{
			{"Ltd", "Private Limited Company", "Private Limited Company (Ltd)", true},
			{"PLC", "Public Limited Company", "Public Limited Company (PLC)", false},
			{"LLP", "Limited Liability Partnership", "Limited Liability Partnership", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "EMI Option (Enterprise Management Incentive)",
				EnglishName:     "EMI Option",
				Description:     "Most tax-efficient UK option. No income tax or NIC on grant/exercise if conditions met; CGT on disposal.",
				EligibilityNote: "Company must have gross assets < £30M, < 250 full-time employees, be independent, and carry on a qualifying trade.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIBCE,
				LocalName:       "CSOP Option (Company Share Option Plan)",
				EnglishName:     "CSOP Option",
				Description:     "HMRC-approved option plan; no income tax on exercise. Limit £60k per employee.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Unapproved / Non-tax-advantaged Option",
				EnglishName:     "Unapproved Option",
				Description:     "No HMRC approval needed; income tax and NIC charged on exercise on the spread.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "RSU / Restricted Share Award",
				EnglishName:     "Restricted Stock Unit",
				Description:     "Taxed as employment income at vesting on market value.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── United States ──────────────────────────────────────────────────────
	"US": {
		CountryCode:              "US",
		CountryName:              "United States",
		Currency:                 "USD",
		CurrencySymbol:           "$",
		DateFormat:               "MM/DD/YYYY",
		PrimaryLanguage:          "en",
		DefaultNominalValueCents: 0, // $0.00001 — represented as 0 (sub-cent par value)
		DefaultLegalForm:         "C-Corp (Delaware)",
		BookEquityTerm:           "Stockholders' equity",
		ShareCapitalTerm:         "Common stock",
		NominalValueNote:         "Delaware C-Corp par value is typically $0.00001 or $0.0001 per share. Enter 0 for sub-cent values.",
		ESOPNote:                 "ISOs are limited to $100k/year of options becoming exercisable, must be granted to employees, and have a 10-year term. NSOs have no limit but are taxed as ordinary income on exercise.",
		LegalForms: []LegalFormDef{
			{"C-Corp (Delaware)", "C Corporation (Delaware)", "C Corporation (Delaware)", true},
			{"LLC", "Limited Liability Company", "Limited Liability Company", true},
			{"S-Corp", "S Corporation", "S Corporation", false},
			{"PBC", "Public Benefit Corporation", "Public Benefit Corporation", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "ISO — Incentive Stock Option",
				EnglishName:     "Incentive Stock Option (ISO)",
				Description:     "Tax-advantaged option for employees. No regular income tax on exercise; AMT may apply.",
				EligibilityNote: "Employee only. $100k/year exercise limit. 10-year term maximum.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIBCE,
				LocalName:       "NSO — Non-Qualified Stock Option",
				EnglishName:     "Non-Qualified Stock Option (NSO)",
				Description:     "Can be issued to employees, contractors, and advisors. Spread taxed as ordinary income on exercise.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "RSU — Restricted Stock Unit",
				EnglishName:     "Restricted Stock Unit (RSU)",
				Description:     "Common at later stages. Taxed as ordinary income at vesting.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Warrant",
				EnglishName:     "Warrant",
				Description:     "Typically issued to investors or advisors as part of a financing round.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Switzerland ────────────────────────────────────────────────────────
	"CH": {
		CountryCode:              "CH",
		CountryName:              "Switzerland",
		Currency:                 "CHF",
		CurrencySymbol:           "CHF",
		DateFormat:               "DD.MM.YYYY",
		PrimaryLanguage:          "fr",
		DefaultNominalValueCents: 1, // CHF 0.01 typical for SA/AG
		DefaultLegalForm:         "SA / AG",
		BookEquityTerm:           "Fonds propres / Eigenkapital",
		ShareCapitalTerm:         "Capital-actions / Aktienkapital",
		NominalValueNote:         "SA/AG: valeur nominale minimale CHF 0.01. Capital minimum: CHF 100 000.",
		LegalForms: []LegalFormDef{
			{"SA / AG", "Société Anonyme / Aktiengesellschaft", "Joint-Stock Company (SA/AG)", true},
			{"Sàrl / GmbH", "Société à responsabilité limitée / GmbH", "Limited Liability Company", true},
			{"Soc. Coop.", "Société coopérative / Genossenschaft", "Cooperative", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "Option sur actions / Aktienoption",
				EnglishName:     "Stock Option",
				Description:     "Taxed as employment income at exercise on the spread; capital gain on subsequent disposal.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Warrant / Optionsschein",
				EnglishName:     "Warrant",
				Description:     "Common in VC-backed rounds; taxed as income on grant at fair value.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "RSU / Action restreinte",
				EnglishName:     "Restricted Stock Unit",
				Description:     "Taxed as employment income at vesting.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Spain ──────────────────────────────────────────────────────────────
	"ES": {
		CountryCode:              "ES",
		CountryName:              "Spain",
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "es",
		DefaultNominalValueCents: 1, // €0.01
		DefaultLegalForm:         "SL",
		BookEquityTerm:           "Patrimonio neto",
		ShareCapitalTerm:         "Capital social",
		NominalValueNote:         "SL: capital mínimo €3 000. SA: capital mínimo €60 000.",
		LegalForms: []LegalFormDef{
			{"SL", "Sociedad de Responsabilidad Limitada", "Private Limited Company (SL)", true},
			{"SA", "Sociedad Anónima", "Public Limited Company (SA)", false},
			{"SLNE", "Sociedad Limitada Nueva Empresa", "Simplified Limited Company (SLNE)", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "Opción sobre acciones / participaciones",
				EnglishName:     "Stock Option",
				Description:     "Taxed as employment income at exercise; €12 000/year exemption for qualifying startups (Ley de Startups 2022).",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIRSU,
				LocalName:       "Acciones / Participaciones restringidas",
				EnglishName:     "Restricted Share / RSU",
				Description:     "Taxed as employment income at vesting.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Italy ──────────────────────────────────────────────────────────────
	"IT": {
		CountryCode:              "IT",
		CountryName:              "Italy",
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "it",
		DefaultNominalValueCents: 1,
		DefaultLegalForm:         "SRL",
		BookEquityTerm:           "Patrimonio netto",
		ShareCapitalTerm:         "Capitale sociale",
		NominalValueNote:         "SRL: capitale minimo €10 000 (€1 per SRL semplificata). SPA: capitale minimo €50 000.",
		ESOPNote:                 "Le startup innovative registrate possono emettere strumenti finanziari partecipativi a condizioni agevolate (art. 27 D.L. 179/2012).",
		LegalForms: []LegalFormDef{
			{"SRL", "Società a Responsabilità Limitata", "Private Limited Company (SRL)", true},
			{"SPA", "Società per Azioni", "Joint-Stock Company (SPA)", false},
			{"SRLS", "SRL Semplificata", "Simplified SRL (SRLS)", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "Stock option / Opzione su azioni",
				EnglishName:     "Stock Option",
				Description:     "Tax-exempt for startup agevolata employees up to €2M lifetime (D.L. 179/2012).",
				EligibilityNote: "Company must be registered as 'startup innovativa' in the Registro delle Imprese.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Warrant / Strumenti finanziari partecipativi",
				EnglishName:     "Warrant / Participatory Financial Instrument",
				Description:     "Common in VC rounds for SPA.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "Restricted Stock Unit (RSU)",
				EnglishName:     "Restricted Stock Unit",
				Description:     "Taxed as employment income at vesting.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Luxembourg ─────────────────────────────────────────────────────────
	"LU": {
		CountryCode:              "LU",
		CountryName:              "Luxembourg",
		Currency:                 "EUR",
		CurrencySymbol:           "€",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "fr",
		DefaultNominalValueCents: 1,
		DefaultLegalForm:         "SA",
		BookEquityTerm:           "Capitaux propres",
		ShareCapitalTerm:         "Capital social",
		LegalForms: []LegalFormDef{
			{"SA", "Société Anonyme", "Public Limited Company (SA)", true},
			{"SARL", "Société à Responsabilité Limitée", "Private Limited Company (SARL)", true},
			{"SCA", "Société en Commandite par Actions", "Partnership Limited by Shares", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Warrant / Bon de souscription",
				EnglishName:     "Warrant",
				Description:     "Commonly used in Luxembourg holding / fund structures. Favourable tax treatment possible.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIStockOption,
				LocalName:       "Option sur actions",
				EnglishName:     "Stock Option",
				Description:     "Circular 104/2 allows 50% exemption on gain for qualifying employees.",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIRSU,
				LocalName:       "RSU / Actions restreintes",
				EnglishName:     "Restricted Stock Unit",
				Description:     "Taxed as employment income at vesting.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Sweden ─────────────────────────────────────────────────────────────
	"SE": {
		CountryCode:              "SE",
		CountryName:              "Sweden",
		Currency:                 "SEK",
		CurrencySymbol:           "kr",
		DateFormat:               "YYYY-MM-DD",
		PrimaryLanguage:          "sv",
		DefaultNominalValueCents: 100, // SEK 1 typical
		DefaultLegalForm:         "AB",
		BookEquityTerm:           "Eget kapital",
		ShareCapitalTerm:         "Aktiekapital",
		NominalValueNote:         "AB: aktiekapital minimum SEK 25 000 (privat AB).",
		ESOPNote:                 "Qualified Employee Stock Options (QESO / Kvalificerade personaloptioner) introduced 2018: no tax at grant/exercise for qualifying companies; capital gains tax at sale.",
		LegalForms: []LegalFormDef{
			{"AB", "Aktiebolag", "Limited Company (AB)", true},
			{"Privat AB", "Privat Aktiebolag", "Private Limited Company", true},
			{"Publikt AB", "Publikt Aktiebolag", "Public Limited Company", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "Kvalificerat personaloption (QESO)",
				EnglishName:     "Qualified Employee Stock Option (QESO)",
				Description:     "No income tax at grant or exercise; CGT only at sale. Requires < 150 employees, < 10 years old, < SEK 80M turnover.",
				IsTaxAdvantaged: true,
				MaxAgeLimitYears: capTableIntPtr(10),
			},
			{
				Code:            SOIBCE,
				LocalName:       "Personaloption (standard)",
				EnglishName:     "Standard Employee Stock Option",
				Description:     "Taxed as employment income at exercise on the spread.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIRSU,
				LocalName:       "RSU / Begränsade aktierätter",
				EnglishName:     "Restricted Stock Unit",
				Description:     "Taxed as employment income at vesting.",
				IsTaxAdvantaged: false,
			},
		},
	},

	// ── Canada ─────────────────────────────────────────────────────────────
	"CA": {
		CountryCode:              "CA",
		CountryName:              "Canada",
		Currency:                 "CAD",
		CurrencySymbol:           "CA$",
		DateFormat:               "DD/MM/YYYY",
		PrimaryLanguage:          "en",
		DefaultNominalValueCents: 0, // No-par-value shares common under CBCA
		DefaultLegalForm:         "Inc. (CBCA)",
		BookEquityTerm:           "Shareholders' equity",
		ShareCapitalTerm:         "Share capital",
		NominalValueNote:         "Canadian corporations under the CBCA typically issue no-par-value shares. Enter 0.",
		LegalForms: []LegalFormDef{
			{"Inc. (CBCA)", "Federal Corporation (CBCA)", "Federal Corporation (CBCA)", true},
			{"Inc. (Ontario)", "Ontario Business Corporation", "Ontario Business Corporation", false},
			{"Inc. (BC)", "BC Business Corporation", "BC Business Corporation", false},
		},
		Instruments: []EquityInstrumentDef{
			{
				Code:            SOIStockOption,
				LocalName:       "Employee Stock Option (ESO)",
				EnglishName:     "Employee Stock Option",
				Description:     "Half of the spread may be deducted under the stock option deduction (subject to $200k/year cap).",
				IsTaxAdvantaged: true,
			},
			{
				Code:            SOIRSU,
				LocalName:       "Restricted Share Unit (RSU)",
				EnglishName:     "Restricted Share Unit (RSU)",
				Description:     "Taxed as employment income at vesting. No deduction available.",
				IsTaxAdvantaged: false,
			},
			{
				Code:            SOIBSAWarrant,
				LocalName:       "Warrant",
				EnglishName:     "Warrant",
				Description:     "Common in VC rounds.",
				IsTaxAdvantaged: false,
			},
		},
	},
}
