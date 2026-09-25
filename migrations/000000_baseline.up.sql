-- Baseline schema (version 0).
--
-- Every table that the Ascenda models require but that no numbered migration
-- creates. Until this file existed the base schema came only from GORM
-- AutoMigrate, which is disabled in production, so a fresh production
-- database could not be provisioned from the migrations alone
-- (docs/AUDIT-2026-09-25.md, §3.1).
--
-- Generated from the GORM models with pg_dump and made idempotent
-- (IF NOT EXISTS / constraint existence checks), so it is also safe on a
-- database whose tables were created by AutoMigrate. golang-migrate only
-- applies it to databases below version 1: existing deployments (version 15)
-- never run it.
--
-- Tables created by later migrations are intentionally absent here:
--   country_rate_configs (000001), ai_narration_cache (000009),
--   organizations (000011), wcr_entries (000014),
--   budget_monthly_overrides and cash_monthly_overrides (000015).
-- migrations 000001-000015 are all guarded with IF [NOT] EXISTS and apply
-- cleanly on top of this baseline; the resulting schema is byte-for-byte the
-- schema of an AutoMigrate database at version 15 (see the CI integration
-- job, which provisions its test database from these migrations only).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ── Tables ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ai_usage_policies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    user_role character varying(20) NOT NULL,
    subscription_tier character varying(20) NOT NULL,
    feature_type character varying(40) NOT NULL,
    allowed boolean DEFAULT false,
    daily_limit bigint,
    weekly_limit bigint,
    monthly_limit bigint,
    priority bigint DEFAULT 0,
    cost_multiplier numeric DEFAULT 1
);

CREATE TABLE IF NOT EXISTS ai_usage_records (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    user_id uuid NOT NULL,
    feature_type character varying(40) NOT NULL,
    used_at timestamp with time zone NOT NULL,
    success boolean DEFAULT true,
    user_role character varying(20),
    subscription_tier character varying(20),
    request_id character varying(50),
    duration_ms bigint,
    tokens_used bigint,
    error_msg text,
    estimated_cost_cents numeric,
    daily_key character varying(10) NOT NULL,
    weekly_key character varying(10) NOT NULL,
    monthly_key character varying(7) NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    entity_type character varying(100) NOT NULL,
    entity_id uuid NOT NULL,
    action character varying(50) NOT NULL,
    changes jsonb,
    created_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS bep_fixed_cost_savings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    plan_id uuid NOT NULL,
    fixed_cost_line_id uuid NOT NULL,
    saving_amount numeric(18,2) DEFAULT '0'::numeric NOT NULL,
    new_amount numeric(18,2) DEFAULT '0'::numeric NOT NULL,
    comment text,
    pcg_account_refs text
);

CREATE TABLE IF NOT EXISTS bep_optimisation_plans (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    snapshot_id uuid NOT NULL,
    name text NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    created_by uuid,
    notes text
);

CREATE TABLE IF NOT EXISTS bep_pcg_review_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    plan_id uuid NOT NULL,
    pcg_code text NOT NULL,
    pcg_label text NOT NULL,
    checked boolean DEFAULT false,
    comment text
);

CREATE TABLE IF NOT EXISTS bep_sensitivity_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    snapshot_id uuid NOT NULL,
    analysis_type text NOT NULL,
    step_size_pct numeric(6,2) NOT NULL,
    range_pct numeric(6,2) NOT NULL,
    is_default boolean DEFAULT true
);

CREATE TABLE IF NOT EXISTS bep_snapshots (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    label text NOT NULL,
    fiscal_year bigint DEFAULT 0,
    period_start date,
    period_end date,
    source text DEFAULT 'manual'::text NOT NULL,
    fixed_costs_total numeric(18,2) DEFAULT '0'::numeric NOT NULL,
    contribution_margin_pct numeric(12,6) DEFAULT '0'::numeric NOT NULL,
    avg_order_value numeric(18,2),
    notes text
);

CREATE TABLE IF NOT EXISTS bep_variable_cost_savings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    plan_id uuid NOT NULL,
    variable_cost_line_id uuid NOT NULL,
    saving_amount numeric(18,4) DEFAULT '0'::numeric NOT NULL,
    new_amount numeric(18,4) DEFAULT '0'::numeric NOT NULL,
    comment text
);

CREATE TABLE IF NOT EXISTS business_plans (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    name character varying(255) NOT NULL,
    description text,
    status character varying(50) DEFAULT 'draft'::character varying NOT NULL,
    created_by uuid NOT NULL,
    is_demo boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS cap_table_companies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    company_name character varying(255) NOT NULL,
    legal_form character varying(50),
    creation_date timestamp with time zone,
    currency character varying(3) DEFAULT 'EUR'::character varying NOT NULL,
    currency_symbol character varying(5) DEFAULT '€'::character varying NOT NULL,
    nominal_value_cents bigint DEFAULT 1 NOT NULL,
    initial_shares bigint DEFAULT 0 NOT NULL,
    initial_capital_k numeric(15,2),
    display_language character varying(5) DEFAULT 'fr'::character varying NOT NULL,
    date_format character varying(12) DEFAULT 'DD/MM/YYYY'::character varying NOT NULL,
    book_equity_term character varying(100),
    share_capital_term character varying(100),
    max_phases bigint DEFAULT 7 NOT NULL,
    founder_alert_pct numeric(5,2) DEFAULT 20.00
);

CREATE TABLE IF NOT EXISTS cap_table_positions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    round_id uuid NOT NULL,
    shareholder_id uuid NOT NULL,
    shares_before_split bigint,
    shares_after_split bigint,
    new_shares_received bigint,
    shares_after_round bigint,
    pct_basic_before numeric(8,4),
    pct_basic_after numeric(8,4),
    pct_fully_diluted numeric(8,4),
    dilution_delta numeric(8,4),
    implied_value_k numeric(15,2),
    amount_invested_k numeric(15,2)
);

CREATE TABLE IF NOT EXISTS cap_table_rounds (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    phase_number bigint NOT NULL,
    label character varying(100) NOT NULL,
    event_date timestamp with time zone,
    event_type character varying(30) NOT NULL,
    financing_source_type character varying(50),
    nominal_value_cents bigint DEFAULT 1 NOT NULL,
    split_coefficient numeric(10,6) DEFAULT 1.000000,
    new_shares_created bigint DEFAULT 0 NOT NULL,
    pre_money_valuation_k numeric(15,2),
    amount_raised_k numeric(15,2),
    pct_granted numeric(8,4),
    share_class_type character varying(30) NOT NULL,
    book_equity_k numeric(15,2),
    emission_premium_reinteg_k numeric(15,2),
    sort_order bigint DEFAULT 0 NOT NULL,
    fiscal_year_index bigint,
    fiplan_synced boolean DEFAULT false NOT NULL,
    fiplan_synced_amount_k numeric(15,2),
    opening_balance_synced boolean DEFAULT false NOT NULL,
    opening_balance_synced_amount_k numeric(15,2)
);

CREATE TABLE IF NOT EXISTS cap_table_scenario_branches (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    branched_from_round_id uuid,
    created_by_user_id uuid NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL
);

CREATE TABLE IF NOT EXISTS cap_table_share_classes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    class_type character varying(30) NOT NULL,
    label character varying(100),
    voting_rights boolean DEFAULT true NOT NULL,
    voting_multiple numeric(5,2) DEFAULT 1.00,
    liquidation_pref character varying(30) DEFAULT 'none'::character varying NOT NULL,
    liquidation_multiple numeric(5,2) DEFAULT 1.00,
    participation_cap numeric(5,2),
    anti_dilution character varying(30) DEFAULT 'none'::character varying NOT NULL,
    conversion_ratio numeric(10,6) DEFAULT 1.000000,
    dividend_rate_pct numeric(5,2),
    sort_order bigint DEFAULT 0 NOT NULL
);

CREATE TABLE IF NOT EXISTS cap_table_shareholders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    type character varying(30) NOT NULL,
    class_type character varying(30) NOT NULL,
    init_shares bigint DEFAULT 0 NOT NULL,
    invested_amount_k numeric(15,2),
    note text,
    sort_order bigint DEFAULT 0 NOT NULL
);

CREATE TABLE IF NOT EXISTS capex_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    category character varying(50) NOT NULL,
    year_index bigint NOT NULL,
    amount numeric(15,2),
    depreciation_years bigint NOT NULL,
    is_manual_override boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS capex_per_hire (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    furniture_per_hire numeric(12,4),
    it_equip_per_hire numeric(12,4)
);

CREATE TABLE IF NOT EXISTS feature_policies (
    feature text NOT NULL,
    category text NOT NULL,
    label text NOT NULL,
    feature_type text NOT NULL,
    freemium jsonb NOT NULL,
    pro jsonb NOT NULL,
    enterprise jsonb NOT NULL,
    updated_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS fiplan_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    line_id character varying(100) NOT NULL,
    year bigint NOT NULL,
    amount numeric(15,2),
    cap_table_round_id uuid,
    cap_table_round_label character varying(100)
);

CREATE TABLE IF NOT EXISTS fixed_cost_lines (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    snapshot_id uuid NOT NULL,
    category text NOT NULL,
    label text NOT NULL,
    amount_annual numeric(18,2) DEFAULT '0'::numeric NOT NULL,
    is_custom_category boolean DEFAULT false,
    sort_order bigint DEFAULT 0
);

CREATE TABLE IF NOT EXISTS magic_link_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email character varying(255) NOT NULL,
    token_hash character varying(64) NOT NULL,
    redirect_url text,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS multi_year_adjustments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    year_index bigint NOT NULL,
    previous_depreciation numeric(15,2),
    potential_tax_credits numeric(15,2)
);

CREATE TABLE IF NOT EXISTS opening_balances (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    noncurrent_assets numeric(15,2),
    inventories numeric(15,2),
    customer_receivables numeric(15,2),
    cash_and_securities numeric(15,2),
    share_capital numeric(15,2),
    retained_earnings numeric(15,2),
    loans_and_debt numeric(15,2),
    supplier_payables numeric(15,2),
    social_and_tax_debts numeric(15,2)
);

CREATE TABLE IF NOT EXISTS opex_manual_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    line_id character varying(100) NOT NULL,
    year_index bigint NOT NULL,
    amount numeric(15,2)
);

CREATE TABLE IF NOT EXISTS opex_per_hire (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    property_rentals numeric(12,4),
    postage_telecom numeric(12,4),
    supplies_purchases numeric(12,4),
    studies_documentation numeric(12,4),
    insurance_costs_pct_sales numeric(5,4),
    royalty_payments_pct_sales numeric(5,4),
    travel_transportation numeric(12,4),
    mission_representation numeric(12,4),
    recruit_training_pct_payroll numeric(5,4)
);

CREATE TABLE IF NOT EXISTS option_grants (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    plan_id uuid NOT NULL,
    shareholder_id uuid NOT NULL,
    round_id uuid NOT NULL,
    options_granted bigint DEFAULT 0 NOT NULL,
    options_exercised bigint DEFAULT 0 NOT NULL,
    options_cancelled bigint DEFAULT 0 NOT NULL,
    vesting_start timestamp with time zone,
    vesting_months bigint,
    cliff_months bigint,
    note text
);

CREATE TABLE IF NOT EXISTS plan_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    language character varying(10) DEFAULT 'fr'::character varying,
    company_name character varying(255),
    forecast_start timestamp with time zone NOT NULL,
    previous_staff bigint DEFAULT 0 NOT NULL,
    prior_year_turnover numeric(15,2),
    avg_distributor_discount numeric(5,4),
    mlt_interest_rate numeric(5,4),
    discounted_sales_pct numeric(5,4),
    bills_discount_rate numeric(5,4),
    avg_bill_term_months bigint DEFAULT 3 NOT NULL,
    vat_rate numeric(5,4),
    corporate_tax_rate numeric(5,4),
    taxes_and_duties_rate numeric(5,4),
    interest_on_positive_cash numeric(5,4),
    mlt_loan_term_years bigint DEFAULT 5 NOT NULL,
    currency_symbol character varying(10) DEFAULT '€'::character varying,
    salary_months_per_year bigint DEFAULT 12 NOT NULL,
    employer_tax_rate numeric(5,4),
    first_fiscal_year_months bigint DEFAULT 12 NOT NULL,
    discount_rate numeric(5,4),
    incentive_cap numeric(5,4),
    country character varying(50) DEFAULT 'BE'::character varying
);

CREATE TABLE IF NOT EXISTS plan_members (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role character varying(50) NOT NULL,
    granted_by uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS plan_shareholders (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    plan_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    type character varying(30) NOT NULL,
    shares bigint DEFAULT 0 NOT NULL,
    ownership_pct numeric(10,4) DEFAULT '0'::numeric,
    invested_amount numeric(15,2) DEFAULT '0'::numeric,
    notes text
);

CREATE TABLE IF NOT EXISTS plan_snapshots (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    version bigint NOT NULL,
    label character varying(255),
    description text,
    created_by uuid NOT NULL,
    reason text,
    data jsonb NOT NULL,
    report_hash character varying(64)
);

CREATE TABLE IF NOT EXISTS pnl_cash_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    line_id character varying(100) NOT NULL,
    year bigint NOT NULL,
    amount numeric(15,2)
);

CREATE TABLE IF NOT EXISTS pnl_manual_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    line_id character varying(100) NOT NULL,
    year_index bigint NOT NULL,
    amount numeric(15,2)
);

CREATE TABLE IF NOT EXISTS product_assumptions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    product_id uuid NOT NULL,
    year bigint NOT NULL,
    raw_material_cost numeric(15,4),
    royalties_cost numeric(15,4),
    logistics_cost numeric(15,4),
    cost_coefficient numeric(8,4) DEFAULT 1.0,
    price_coefficient numeric(8,4) DEFAULT 1.0,
    base_unit_price numeric(15,4),
    is_overridden boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS product_distributor_margins (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    product_id uuid NOT NULL,
    year bigint NOT NULL,
    zone character varying(20) NOT NULL,
    margin_percent numeric(8,4),
    is_overridden boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS product_sales_volumes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    product_id uuid NOT NULL,
    year bigint NOT NULL,
    zone character varying(20) NOT NULL,
    channel character varying(20) NOT NULL,
    units_sold bigint DEFAULT 0 NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    product_type character varying(50) DEFAULT 'product'::character varying NOT NULL,
    sort_order bigint DEFAULT 0 NOT NULL,
    direct_cost_variability numeric(5,4),
    external_charge_variability numeric(5,4),
    tax_variability numeric(5,4),
    staff_variability numeric(5,4),
    depreciation_variability numeric(5,4),
    driver_type character varying(50) DEFAULT 'generic'::character varying NOT NULL,
    driver_params jsonb
);

CREATE TABLE IF NOT EXISTS reports (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    data jsonb NOT NULL
);

CREATE TABLE IF NOT EXISTS scenarios (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    plan_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    is_default boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS staff_headcounts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    category character varying(50) NOT NULL,
    year bigint NOT NULL,
    fte numeric(8,2),
    is_overridden boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS staff_incentives (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    year bigint NOT NULL,
    incentive_pct numeric(8,4),
    specific_incentives numeric(12,4),
    is_overridden boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS staff_salaries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    category character varying(50) NOT NULL,
    year bigint NOT NULL,
    monthly_gross_salary numeric(12,4),
    annual_increase_pct numeric(8,4),
    is_overridden boolean DEFAULT false NOT NULL
);

CREATE TABLE IF NOT EXISTS stock_option_plans (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    round_id uuid,
    plan_label character varying(100) NOT NULL,
    instrument character varying(20) NOT NULL,
    exercise_price numeric(12,4) NOT NULL,
    options_voted bigint DEFAULT 0 NOT NULL,
    options_attributed bigint DEFAULT 0 NOT NULL,
    options_exercised bigint DEFAULT 0 NOT NULL,
    options_cancelled bigint DEFAULT 0 NOT NULL,
    company_age_at_grant_years bigint,
    is_eligible_bspce boolean,
    sort_order bigint DEFAULT 0 NOT NULL
);

CREATE TABLE IF NOT EXISTS tenants (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    organization_id uuid,
    type character varying(50) DEFAULT 'workspace'::character varying NOT NULL,
    plan character varying(50) DEFAULT 'freemium'::character varying NOT NULL,
    name character varying(255) NOT NULL,
    slug character varying(100),
    is_active boolean DEFAULT true NOT NULL,
    ai_credits bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    external_id character varying(255),
    email character varying(255) NOT NULL,
    name character varying(255),
    role character varying(50) DEFAULT 'user'::character varying NOT NULL,
    plan character varying(50) DEFAULT 'freemium'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    invited_by uuid,
    joined_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS valuation_scenarios (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    calc_type character varying(40) NOT NULL,
    label character varying(100),
    investment_k numeric(15,2),
    horizon_years numeric(5,2),
    annual_yield_pct numeric(8,4),
    final_investor_pct numeric(8,4),
    exit_company_value_k numeric(15,2),
    money_multiple numeric(10,4),
    discount_rate_pct numeric(8,4),
    pre_money_input_k numeric(15,2),
    new_money_k numeric(15,2),
    out_irr_pct numeric(8,4),
    out_terminal_value_k numeric(15,2),
    out_multiple numeric(10,4),
    out_investor_share_k numeric(15,2),
    out_npvk numeric(15,2),
    out_pre_money_k numeric(15,2),
    out_post_money_k numeric(15,2),
    currency character varying(3) DEFAULT 'EUR'::character varying,
    display_language character varying(5) DEFAULT 'fr'::character varying
);

CREATE TABLE IF NOT EXISTS variable_cost_lines (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    snapshot_id uuid NOT NULL,
    category text NOT NULL,
    label text NOT NULL,
    amount_per_unit numeric(18,4) DEFAULT '0'::numeric NOT NULL,
    is_custom_category boolean DEFAULT false,
    sort_order bigint DEFAULT 0
);

CREATE TABLE IF NOT EXISTS working_capital_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    scenario_id uuid NOT NULL,
    customer_pct0_days numeric(5,4),
    customer_pct30_days numeric(5,4),
    customer_pct60_days numeric(5,4),
    customer_pct90_days numeric(5,4),
    supplier_pct0_days numeric(5,4),
    supplier_pct30_days numeric(5,4),
    supplier_pct60_days numeric(5,4),
    supplier_pct90_days numeric(5,4),
    inventory_pct_year1 numeric(5,4),
    inventory_pct_year2 numeric(5,4),
    inventory_pct_year3 numeric(5,4),
    inventory_pct_year4 numeric(5,4),
    inventory_pct_year5 numeric(5,4)
);

-- ── Primary keys and unique constraints ───────────────────────────────────────

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ai_usage_policies_pkey') THEN
        ALTER TABLE ONLY ai_usage_policies ADD CONSTRAINT ai_usage_policies_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ai_usage_records_pkey') THEN
        ALTER TABLE ONLY ai_usage_records ADD CONSTRAINT ai_usage_records_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'audit_logs_pkey') THEN
        ALTER TABLE ONLY audit_logs ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bep_fixed_cost_savings_pkey') THEN
        ALTER TABLE ONLY bep_fixed_cost_savings ADD CONSTRAINT bep_fixed_cost_savings_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bep_optimisation_plans_pkey') THEN
        ALTER TABLE ONLY bep_optimisation_plans ADD CONSTRAINT bep_optimisation_plans_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bep_pcg_review_items_pkey') THEN
        ALTER TABLE ONLY bep_pcg_review_items ADD CONSTRAINT bep_pcg_review_items_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bep_sensitivity_configs_pkey') THEN
        ALTER TABLE ONLY bep_sensitivity_configs ADD CONSTRAINT bep_sensitivity_configs_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bep_snapshots_pkey') THEN
        ALTER TABLE ONLY bep_snapshots ADD CONSTRAINT bep_snapshots_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bep_variable_cost_savings_pkey') THEN
        ALTER TABLE ONLY bep_variable_cost_savings ADD CONSTRAINT bep_variable_cost_savings_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'business_plans_pkey') THEN
        ALTER TABLE ONLY business_plans ADD CONSTRAINT business_plans_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cap_table_companies_pkey') THEN
        ALTER TABLE ONLY cap_table_companies ADD CONSTRAINT cap_table_companies_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cap_table_positions_pkey') THEN
        ALTER TABLE ONLY cap_table_positions ADD CONSTRAINT cap_table_positions_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cap_table_rounds_pkey') THEN
        ALTER TABLE ONLY cap_table_rounds ADD CONSTRAINT cap_table_rounds_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cap_table_scenario_branches_pkey') THEN
        ALTER TABLE ONLY cap_table_scenario_branches ADD CONSTRAINT cap_table_scenario_branches_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cap_table_share_classes_pkey') THEN
        ALTER TABLE ONLY cap_table_share_classes ADD CONSTRAINT cap_table_share_classes_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cap_table_shareholders_pkey') THEN
        ALTER TABLE ONLY cap_table_shareholders ADD CONSTRAINT cap_table_shareholders_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'capex_entries_pkey') THEN
        ALTER TABLE ONLY capex_entries ADD CONSTRAINT capex_entries_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'capex_per_hire_pkey') THEN
        ALTER TABLE ONLY capex_per_hire ADD CONSTRAINT capex_per_hire_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'feature_policies_pkey') THEN
        ALTER TABLE ONLY feature_policies ADD CONSTRAINT feature_policies_pkey PRIMARY KEY (feature);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fiplan_entries_pkey') THEN
        ALTER TABLE ONLY fiplan_entries ADD CONSTRAINT fiplan_entries_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fixed_cost_lines_pkey') THEN
        ALTER TABLE ONLY fixed_cost_lines ADD CONSTRAINT fixed_cost_lines_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'magic_link_tokens_pkey') THEN
        ALTER TABLE ONLY magic_link_tokens ADD CONSTRAINT magic_link_tokens_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'multi_year_adjustments_pkey') THEN
        ALTER TABLE ONLY multi_year_adjustments ADD CONSTRAINT multi_year_adjustments_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'opening_balances_pkey') THEN
        ALTER TABLE ONLY opening_balances ADD CONSTRAINT opening_balances_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'opex_manual_entries_pkey') THEN
        ALTER TABLE ONLY opex_manual_entries ADD CONSTRAINT opex_manual_entries_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'opex_per_hire_pkey') THEN
        ALTER TABLE ONLY opex_per_hire ADD CONSTRAINT opex_per_hire_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'option_grants_pkey') THEN
        ALTER TABLE ONLY option_grants ADD CONSTRAINT option_grants_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'plan_configs_pkey') THEN
        ALTER TABLE ONLY plan_configs ADD CONSTRAINT plan_configs_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'plan_members_pkey') THEN
        ALTER TABLE ONLY plan_members ADD CONSTRAINT plan_members_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'plan_shareholders_pkey') THEN
        ALTER TABLE ONLY plan_shareholders ADD CONSTRAINT plan_shareholders_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'plan_snapshots_pkey') THEN
        ALTER TABLE ONLY plan_snapshots ADD CONSTRAINT plan_snapshots_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'pnl_cash_entries_pkey') THEN
        ALTER TABLE ONLY pnl_cash_entries ADD CONSTRAINT pnl_cash_entries_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'pnl_manual_entries_pkey') THEN
        ALTER TABLE ONLY pnl_manual_entries ADD CONSTRAINT pnl_manual_entries_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'product_assumptions_pkey') THEN
        ALTER TABLE ONLY product_assumptions ADD CONSTRAINT product_assumptions_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'product_distributor_margins_pkey') THEN
        ALTER TABLE ONLY product_distributor_margins ADD CONSTRAINT product_distributor_margins_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'product_sales_volumes_pkey') THEN
        ALTER TABLE ONLY product_sales_volumes ADD CONSTRAINT product_sales_volumes_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'products_pkey') THEN
        ALTER TABLE ONLY products ADD CONSTRAINT products_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'reports_pkey') THEN
        ALTER TABLE ONLY reports ADD CONSTRAINT reports_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'scenarios_pkey') THEN
        ALTER TABLE ONLY scenarios ADD CONSTRAINT scenarios_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'staff_headcounts_pkey') THEN
        ALTER TABLE ONLY staff_headcounts ADD CONSTRAINT staff_headcounts_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'staff_incentives_pkey') THEN
        ALTER TABLE ONLY staff_incentives ADD CONSTRAINT staff_incentives_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'staff_salaries_pkey') THEN
        ALTER TABLE ONLY staff_salaries ADD CONSTRAINT staff_salaries_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'stock_option_plans_pkey') THEN
        ALTER TABLE ONLY stock_option_plans ADD CONSTRAINT stock_option_plans_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tenants_pkey') THEN
        ALTER TABLE ONLY tenants ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_pkey') THEN
        ALTER TABLE ONLY users ADD CONSTRAINT users_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'valuation_scenarios_pkey') THEN
        ALTER TABLE ONLY valuation_scenarios ADD CONSTRAINT valuation_scenarios_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'variable_cost_lines_pkey') THEN
        ALTER TABLE ONLY variable_cost_lines ADD CONSTRAINT variable_cost_lines_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'working_capital_configs_pkey') THEN
        ALTER TABLE ONLY working_capital_configs ADD CONSTRAINT working_capital_configs_pkey PRIMARY KEY (id);
    END IF;
END $$;

-- ── Indexes ───────────────────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_ai_usage_policies_feature_type ON ai_usage_policies USING btree (feature_type);
CREATE INDEX IF NOT EXISTS idx_ai_usage_policies_subscription_tier ON ai_usage_policies USING btree (subscription_tier);
CREATE INDEX IF NOT EXISTS idx_ai_usage_policies_tenant_id ON ai_usage_policies USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_policies_user_role ON ai_usage_policies USING btree (user_role);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_daily_key ON ai_usage_records USING btree (daily_key);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_feature_type ON ai_usage_records USING btree (feature_type);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_monthly_key ON ai_usage_records USING btree (monthly_key);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_subscription_tier ON ai_usage_records USING btree (subscription_tier);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_tenant_id ON ai_usage_records USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_used_at ON ai_usage_records USING btree (used_at);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_user_id ON ai_usage_records USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_user_role ON ai_usage_records USING btree (user_role);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_weekly_key ON ai_usage_records USING btree (weekly_key);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs USING btree (created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity_id ON audit_logs USING btree (entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity_type ON audit_logs USING btree (entity_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_id ON audit_logs USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bep_fixed_cost_savings_plan_id ON bep_fixed_cost_savings USING btree (plan_id);
CREATE INDEX IF NOT EXISTS idx_bep_fixed_cost_savings_tenant_id ON bep_fixed_cost_savings USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bep_optimisation_plans_snapshot_id ON bep_optimisation_plans USING btree (snapshot_id);
CREATE INDEX IF NOT EXISTS idx_bep_optimisation_plans_tenant_id ON bep_optimisation_plans USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bep_pcg_review_items_tenant_id ON bep_pcg_review_items USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bep_sensitivity_configs_tenant_id ON bep_sensitivity_configs USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bep_snapshots_scenario_id ON bep_snapshots USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_bep_snapshots_tenant_id ON bep_snapshots USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_bep_variable_cost_savings_plan_id ON bep_variable_cost_savings USING btree (plan_id);
CREATE INDEX IF NOT EXISTS idx_bep_variable_cost_savings_tenant_id ON bep_variable_cost_savings USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_business_plans_tenant_id ON business_plans USING btree (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cap_table_companies_scenario_id ON cap_table_companies USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_companies_tenant_id ON cap_table_companies USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_positions_tenant_id ON cap_table_positions USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_rounds_scenario_id ON cap_table_rounds USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_rounds_tenant_id ON cap_table_rounds USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_scenario_branches_scenario_id ON cap_table_scenario_branches USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_scenario_branches_tenant_id ON cap_table_scenario_branches USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_share_classes_tenant_id ON cap_table_share_classes USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cap_table_shareholders_tenant_id ON cap_table_shareholders USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_capex_entries_tenant_id ON capex_entries USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_capex_per_hire_scenario_id ON capex_per_hire USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_capex_per_hire_tenant_id ON capex_per_hire USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_feature_policies_category ON feature_policies USING btree (category);
CREATE INDEX IF NOT EXISTS idx_fiplan_entries_tenant_id ON fiplan_entries USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_fixed_cost_lines_snapshot_id ON fixed_cost_lines USING btree (snapshot_id);
CREATE INDEX IF NOT EXISTS idx_fixed_cost_lines_tenant_id ON fixed_cost_lines USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_email ON magic_link_tokens USING btree (email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_magic_link_tokens_token_hash ON magic_link_tokens USING btree (token_hash);
CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_used_at ON magic_link_tokens USING btree (used_at);
CREATE INDEX IF NOT EXISTS idx_multi_year_adjustments_scenario_id ON multi_year_adjustments USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_multi_year_adjustments_tenant_id ON multi_year_adjustments USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_opening_balances_scenario_id ON opening_balances USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_opening_balances_tenant_id ON opening_balances USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_opex_manual_entries_tenant_id ON opex_manual_entries USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_opex_per_hire_scenario_id ON opex_per_hire USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_opex_per_hire_tenant_id ON opex_per_hire USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_option_grants_plan_id ON option_grants USING btree (plan_id);
CREATE INDEX IF NOT EXISTS idx_option_grants_round_id ON option_grants USING btree (round_id);
CREATE INDEX IF NOT EXISTS idx_option_grants_shareholder_id ON option_grants USING btree (shareholder_id);
CREATE INDEX IF NOT EXISTS idx_option_grants_tenant_id ON option_grants USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_plan_configs_scenario_id ON plan_configs USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_plan_configs_tenant_id ON plan_configs USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_plan_members_plan_id ON plan_members USING btree (plan_id);
CREATE INDEX IF NOT EXISTS idx_plan_members_tenant_id ON plan_members USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_plan_members_user_id ON plan_members USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_plan_shareholders_plan_id ON plan_shareholders USING btree (plan_id);
CREATE INDEX IF NOT EXISTS idx_plan_shareholders_tenant_id ON plan_shareholders USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_plan_snapshots_scenario_id ON plan_snapshots USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_plan_snapshots_tenant_id ON plan_snapshots USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pnl_cash_entries_tenant_id ON pnl_cash_entries USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pnl_manual_entries_scenario_id ON pnl_manual_entries USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_pnl_manual_entries_tenant_id ON pnl_manual_entries USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_product_assumptions_product_id ON product_assumptions USING btree (product_id);
CREATE INDEX IF NOT EXISTS idx_product_assumptions_tenant_id ON product_assumptions USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_product_distributor_margins_product_id ON product_distributor_margins USING btree (product_id);
CREATE INDEX IF NOT EXISTS idx_product_distributor_margins_tenant_id ON product_distributor_margins USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_product_sales_volumes_product_id ON product_sales_volumes USING btree (product_id);
CREATE INDEX IF NOT EXISTS idx_product_sales_volumes_tenant_id ON product_sales_volumes USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_products_scenario_id ON products USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_products_tenant_id ON products USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_reports_scenario_id ON reports USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_reports_tenant_id ON reports USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_scenarios_plan_id ON scenarios USING btree (plan_id);
CREATE INDEX IF NOT EXISTS idx_scenarios_tenant_id ON scenarios USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_headcounts_tenant_id ON staff_headcounts USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_incentives_tenant_id ON staff_incentives USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_salaries_tenant_id ON staff_salaries USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_stock_option_plans_round_id ON stock_option_plans USING btree (round_id);
CREATE INDEX IF NOT EXISTS idx_stock_option_plans_scenario_id ON stock_option_plans USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_stock_option_plans_tenant_id ON stock_option_plans USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenants_organization_id ON tenants USING btree (organization_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_slug ON tenants USING btree (slug);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_valuation_scenarios_scenario_id ON valuation_scenarios USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_valuation_scenarios_tenant_id ON valuation_scenarios USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_variable_cost_lines_snapshot_id ON variable_cost_lines USING btree (snapshot_id);
CREATE INDEX IF NOT EXISTS idx_variable_cost_lines_tenant_id ON variable_cost_lines USING btree (tenant_id);
CREATE INDEX IF NOT EXISTS idx_working_capital_configs_scenario_id ON working_capital_configs USING btree (scenario_id);
CREATE INDEX IF NOT EXISTS idx_working_capital_configs_tenant_id ON working_capital_configs USING btree (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS uix_cap_position ON cap_table_positions USING btree (round_id, shareholder_id);
CREATE UNIQUE INDEX IF NOT EXISTS uix_cap_shareholder ON cap_table_shareholders USING btree (scenario_id, name);
CREATE UNIQUE INDEX IF NOT EXISTS uix_capex_entries ON capex_entries USING btree (scenario_id, category, year_index);
CREATE UNIQUE INDEX IF NOT EXISTS uix_fiplan_entries ON fiplan_entries USING btree (scenario_id, line_id, year);
CREATE UNIQUE INDEX IF NOT EXISTS uix_opex_entries ON opex_manual_entries USING btree (scenario_id, line_id, year_index);
CREATE UNIQUE INDEX IF NOT EXISTS uix_pcg_review ON bep_pcg_review_items USING btree (plan_id, pcg_code);
CREATE UNIQUE INDEX IF NOT EXISTS uix_pnl_cash_entries ON pnl_cash_entries USING btree (scenario_id, line_id, year);
CREATE UNIQUE INDEX IF NOT EXISTS uix_sens_config ON bep_sensitivity_configs USING btree (snapshot_id, analysis_type);
CREATE UNIQUE INDEX IF NOT EXISTS uix_share_class ON cap_table_share_classes USING btree (scenario_id, class_type);
CREATE UNIQUE INDEX IF NOT EXISTS uix_staff_headcounts ON staff_headcounts USING btree (scenario_id, category, year);
CREATE UNIQUE INDEX IF NOT EXISTS uix_staff_incentives ON staff_incentives USING btree (scenario_id, year);
CREATE UNIQUE INDEX IF NOT EXISTS uix_staff_salaries ON staff_salaries USING btree (scenario_id, category, year);
