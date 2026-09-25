// Package service — internal localisation layer for analysis text.
//
// # Design
//
//   - All translations live in a single flat map keyed by a stable string
//     identifier (the "key").  Each key maps to a lang → template map.
//   - Two languages are supported in v1: "en" (default) and "fr".
//   - Param substitution uses {param_name} tokens; params are passed as
//     map[string]interface{} and formatted with %v.
//   - normalizeLanguage normalises BCP-47 tags ("fr-FR", "fr_FR") to a
//     2-letter code and falls back to "en" for unsupported languages.
//   - localize always returns a non-empty string (raw key as ultimate fallback).
//
// # Adding a new language
//
//  1. Add new entries to each key in translations.
//  2. Add the code to normalizeLanguage's allowed list.
//
// # Translation keys
//
// Risk messages  : cash_gap, late_profitability_late, late_profitability_none,
//
//	no_profitability, unrealistic_growth, high_leverage_no_fcf,
//	high_leverage_elevated
//
// Driver names   : driver_hiring_aggressive, driver_pricing_low,
//
//	driver_inefficient_growth, driver_strong_leverage,
//	driver_payroll_dominant
//
// Driver effects : effect_hiring_aggressive, effect_pricing_low,
//
//	effect_inefficient_growth, effect_strong_leverage,
//	effect_payroll_dominant
//
// Highlights     : hl_no_signal, hl_cash_positive, hl_ebitda_positive,
//
//	hl_weaknesses_none, hl_top_risks_none, hl_top_drivers_none
//
// Headlines      : headline_healthy, headline_strong_risk,
//
//	headline_moderate_root, headline_moderate_no_root,
//	headline_critical, headline_approaching,
//	headline_not_fundable
package service

import (
	"fmt"
	"strings"
)

// translations is the master localisation map.
// Structure: key → lang → template string.
// Template tokens use {param} syntax; replaced by localize().
var translations = map[string]map[string]string{

	// ── Risk messages ────────────────────────────────────────────────────────

	"cash_gap": {
		"en": "Cash position drops to {cash} at month {month} — the scenario may require additional financing.",
		"fr": "La trésorerie tombe à {cash} au mois {month} — le scénario peut nécessiter un financement complémentaire.",
	},
	"late_profitability_late": {
		"en": "Break-even is reached at month {month}, which is beyond the 24-month threshold.",
		"fr": "Le seuil de rentabilité est atteint au mois {month}, ce qui dépasse le seuil de 24 mois.",
	},
	"late_profitability_none": {
		"en": "The scenario does not reach profitability within the 5-year horizon.",
		"fr": "Le scénario n'atteint pas la rentabilité dans l'horizon de 5 ans.",
	},
	"no_profitability": {
		"en": "EBITDA never turns positive — the scenario generates structural operating losses.",
		"fr": "L'EBITDA ne devient jamais positif — le scénario génère des pertes opérationnelles structurelles.",
	},
	"unrealistic_growth": {
		"en": "Revenue growth reaches {pct} % year-over-year, which may be difficult to achieve and sustain.",
		"fr": "La croissance du chiffre d'affaires atteint {pct} % d'une année sur l'autre, ce qui peut être difficile à atteindre et à maintenir.",
	},
	"high_leverage_no_fcf": {
		"en": "Net debt of {debt} in year 5 cannot be serviced from free cash flow — the scenario carries structural refinancing risk.",
		"fr": "Une dette nette de {debt} en année 5 ne peut pas être couverte par les flux de trésorerie disponibles — risque de refinancement structurel.",
	},
	"high_leverage_elevated": {
		"en": "Net debt is {ratio}x free cash flow in year 5 — leverage is elevated and may constrain future financing.",
		"fr": "La dette nette représente {ratio}x les flux de trésorerie disponibles en année 5 — l'effet de levier est élevé et pourrait limiter les financements futurs.",
	},

	// ── Driver names (stable labels per language) ────────────────────────────

	"driver_hiring_aggressive": {
		"en": "Hiring too aggressive",
		"fr": "Embauche trop agressive",
	},
	"driver_pricing_low": {
		"en": "Pricing too low",
		"fr": "Prix de vente trop bas",
	},
	"driver_inefficient_growth": {
		"en": "Inefficient growth model",
		"fr": "Modèle de croissance inefficace",
	},
	"driver_strong_leverage": {
		"en": "Strong operating leverage",
		"fr": "Effet de levier opérationnel fort",
	},
	"driver_payroll_dominant": {
		"en": "Cash contribution: payroll dominant",
		"fr": "Contribution cash : masse salariale dominante",
	},

	// ── Driver effects ────────────────────────────────────────────────────────

	"effect_hiring_aggressive": {
		"en": "Payroll represents {pct} % of year-1 revenue — reduce hiring pace or increase revenue to improve the ratio.",
		"fr": "La masse salariale représente {pct} % du chiffre d'affaires de l'année 1 — réduisez le rythme d'embauche ou augmentez les revenus pour améliorer ce ratio.",
	},
	"effect_pricing_low": {
		"en": "Gross margin is {pct} % in year 1 — consider increasing prices or reducing COGS.",
		"fr": "La marge brute est de {pct} % en année 1 — envisagez d'augmenter les prix ou de réduire les coûts de production.",
	},
	"effect_inefficient_growth": {
		"en": "Year-1 burn is {burn} % of revenue but year-2 growth is only {growth} % — costs are scaling faster than revenue.",
		"fr": "La consommation de trésorerie en année 1 est de {burn} % du chiffre d'affaires alors que la croissance en année 2 n'est que de {growth} % — les coûts augmentent plus vite que les revenus.",
	},
	"effect_strong_leverage": {
		"en": "EBITDA margin reaches {pct} % by year 5 — costs are scaling sub-linearly relative to revenue.",
		"fr": "La marge d'EBITDA atteint {pct} % en année 5 — les coûts progressent moins vite que les revenus.",
	},
	"effect_payroll_dominant": {
		"en": "Payroll accounts for {pct} % of year-1 operating cash outflow — headcount cost is the primary cash drain.",
		"fr": "La masse salariale représente {pct} % des sorties de trésorerie opérationnelles de l'année 1 — le coût des effectifs est le principal poste de consommation de trésorerie.",
	},

	// ── Highlights — fallback strings ────────────────────────────────────────

	"hl_no_signal": {
		"en": "No dominant positive signal detected — scenario appears balanced but inconclusive.",
		"fr": "Aucun signal positif dominant détecté — le scénario semble équilibré mais peu concluant.",
	},
	"hl_cash_positive": {
		"en": "Cash position is positive throughout the 5-year horizon.",
		"fr": "La trésorerie reste positive tout au long de l'horizon de 5 ans.",
	},
	"hl_ebitda_positive": {
		"en": "EBITDA turns positive during the plan period.",
		"fr": "L'EBITDA devient positif au cours de la période du plan.",
	},
	"hl_weaknesses_none": {
		"en": "No structural risks detected — scenario appears stable under current assumptions.",
		"fr": "Aucun risque structurel détecté — le scénario semble stable dans les hypothèses actuelles.",
	},
	"hl_top_risks_none": {
		"en": "No risks flagged — model appears robust at current assumptions.",
		"fr": "Aucun risque identifié — le modèle semble robuste dans les hypothèses actuelles.",
	},
	"hl_top_drivers_none": {
		"en": "No dominant signal detected — scenario appears balanced but inconclusive.",
		"fr": "Aucun signal dominant détecté — le scénario semble équilibré mais peu concluant.",
	},

	// ── Headline driver fallback (when no drivers detected) ─────────────────

	"strong_fundamentals": {
		"en": "strong fundamentals",
		"fr": "fondamentaux solides",
	},

	// ── Headlines (7 templates) ───────────────────────────────────────────────

	"headline_healthy": {
		"en": "Healthy scenario: {driver} drives profitability by month {bep}.",
		"fr": "Scénario sain : {driver} génère de la rentabilité à partir du mois {bep}.",
	},
	"headline_strong_risk": {
		"en": "Strong viability ({score}/100) but {risk_type} poses a risk{risk_when}.",
		"fr": "Viabilité forte ({score}/100) mais {risk_type} présente un risque{risk_when}.",
	},
	"headline_moderate_root": {
		"en": "{driver} limits growth: break-even {bep_str}, viability {score}/100.",
		"fr": "{driver} limite la croissance : seuil de rentabilité {bep_str}, viabilité {score}/100.",
	},
	"headline_moderate_no_root": {
		"en": "Moderate scenario: {bep_status} at month {bep}, {risk_count} risk(s) identified.",
		"fr": "Scénario modéré : {bep_status} au mois {bep}, {risk_count} risque(s) identifié(s).",
	},
	"headline_critical": {
		"en": "Critical: {risk_count} compounding risks — model requires structural revision before funding.",
		"fr": "Critique : {risk_count} risques cumulatifs — révision structurelle requise avant financement.",
	},
	"headline_approaching": {
		"en": "Approaching viability but not yet fundable: projected break-even at month {bep}.",
		"fr": "Viabilité en approche mais pas encore finançable : seuil de rentabilité projeté au mois {bep}.",
	},
	"headline_not_fundable": {
		"en": "Not fundable under current assumptions: no break-even path within 5 years.",
		"fr": "Non finançable dans les hypothèses actuelles : aucun chemin vers la rentabilité dans les 5 ans.",
	},
}

// normalizeLanguage normalises a language tag to a supported 2-letter code.
// BCP-47 subtags ("fr-FR", "fr_FR") are stripped to the base tag.
// Unknown or empty tags default to "en".
func normalizeLanguage(lang string) string {
	if lang == "" {
		return "en"
	}
	lower := strings.ToLower(lang)
	if idx := strings.IndexAny(lower, "-_"); idx != -1 {
		lower = lower[:idx]
	}
	switch lower {
	case "fr", "en":
		return lower
	default:
		return "en"
	}
}

// localize returns the localised string for key in lang, with {param} tokens
// replaced by values from params.
// Falls back to "en" when lang is unsupported or the key has no "en" entry.
// Returns the raw key when the key is completely absent from translations.
func localize(lang, key string, params map[string]interface{}) string {
	m, ok := translations[key]
	if !ok {
		return key
	}
	text, ok := m[lang]
	if !ok {
		text, ok = m["en"]
		if !ok {
			return key
		}
	}
	if len(params) == 0 {
		return text
	}
	// Build flat old/new pairs for strings.NewReplacer.
	pairs := make([]string, 0, len(params)*2)
	for k, v := range params {
		pairs = append(pairs, "{"+k+"}", fmt.Sprintf("%v", v))
	}
	return strings.NewReplacer(pairs...).Replace(text)
}
