-- Migration: add_country_rate_configs
-- Moves statutory financial rates (corporate tax, VAT, employer charge, MLT
-- interest rate) out of hard-coded Go source into a database table so that
-- platform admins can update them without a code deployment.
--
-- The table is seeded at startup by CountryRateConfigService.SeedDefaults().

CREATE TABLE IF NOT EXISTS country_rate_configs (
    country_code       VARCHAR(2)     NOT NULL PRIMARY KEY,
    country_name       VARCHAR(100)   NOT NULL DEFAULT '',
    corporate_tax_rate NUMERIC(6, 4)  NOT NULL DEFAULT 0,
    vat_rate           NUMERIC(6, 4)  NOT NULL DEFAULT 0,
    employer_tax_rate  NUMERIC(6, 4)  NOT NULL DEFAULT 0,
    mlt_interest_rate  NUMERIC(6, 4)  NOT NULL DEFAULT 0,
    language           VARCHAR(10)    NOT NULL DEFAULT 'en',
    currency_symbol    VARCHAR(10)    NOT NULL DEFAULT '€',
    updated_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
