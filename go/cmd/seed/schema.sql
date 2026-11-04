-- Phase 1 reference data. Seed drops and recreates these tables every run, so it is idempotent
-- and picks up schema edits. Column order matches the CSV headers (COPY ... HEADER).

DROP TABLE IF EXISTS leads.smb_leads CASCADE;
CREATE TABLE leads.smb_leads (
  lead_id                   text PRIMARY KEY,
  created_date              date NOT NULL,
  lead_source               text NOT NULL,
  business_name             text NOT NULL,
  business_type             text NOT NULL,
  segment                   text NOT NULL,
  city                      text NOT NULL,
  city_tier                 int  NOT NULL,
  state                     text NOT NULL,
  preferred_language        text NOT NULL,
  employee_count            int  NOT NULL,
  num_sites                 int  NOT NULL,
  annual_turnover_band      text NOT NULL,
  has_gstin                 boolean NOT NULL,
  gstin                     text,
  has_udyam                 boolean NOT NULL,
  udyam_number              text,
  current_internet          text,
  current_provider          text,
  monthly_telecom_spend_inr int  NOT NULL,
  products_of_interest      text NOT NULL,
  contact_name              text NOT NULL,
  contact_role              text NOT NULL,
  stage_reached             text NOT NULL,
  outcome                   text NOT NULL,
  drop_reason               text,
  contacted_date            date,
  needs_assessed_date       date,
  quote_accepted_date       date,
  kyc_complete_date         date,
  activated_date            date,
  days_lead_to_activation   numeric,
  customer_follow_up_calls  int NOT NULL,
  bundle_products           text,
  monthly_bundle_value_inr  numeric,
  one_time_setup_inr        numeric,
  contract_months           numeric,
  churned_within_6_months   boolean,
  nps_score                 numeric,
  acquisition_cost_inr      int NOT NULL
);

DROP TABLE IF EXISTS catalogue.product_catalogue CASCADE;
CREATE TABLE catalogue.product_catalogue (
  product_id              text PRIMARY KEY,
  category                text NOT NULL,
  product_name            text NOT NULL,
  description             text NOT NULL,
  unit                    text NOT NULL,
  price_min_inr_per_month int  NOT NULL,
  price_max_inr_per_month int  NOT NULL,
  one_time_setup_inr      int  NOT NULL,
  min_contract_months     int  NOT NULL,
  available_city_tiers    text NOT NULL,
  min_employees           int  NOT NULL,
  min_sites               int  NOT NULL,
  requires_gstin          boolean NOT NULL,
  requires_one_of         text,
  typical_install_days    int  NOT NULL,
  target_segments         text NOT NULL
);

DROP TABLE IF EXISTS registry.gstin CASCADE;
CREATE TABLE registry.gstin (
  gstin                       text PRIMARY KEY,
  legal_name                  text NOT NULL,
  trade_name                  text NOT NULL,
  constitution                text NOT NULL,
  principal_place_of_business text NOT NULL,
  state                       text NOT NULL,
  registration_date           date NOT NULL,
  status                      text NOT NULL
);

DROP TABLE IF EXISTS registry.udyam CASCADE;
CREATE TABLE registry.udyam (
  udyam_number         text PRIMARY KEY,
  enterprise_name      text NOT NULL,
  enterprise_type      text NOT NULL,
  major_activity       text NOT NULL,
  nic_description      text NOT NULL,
  state                text NOT NULL,
  city                 text NOT NULL,
  date_of_registration date NOT NULL
);
