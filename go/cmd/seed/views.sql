-- Analytics views over leads.smb_leads (Phase 1). Read by cmd/analytics -> docs/analytics.md.

-- Funnel: a lead counts at a stage if it has that stage's date (matches docs/CASE_README.md).
CREATE OR REPLACE VIEW leads.v_funnel AS
WITH s AS (
  SELECT 1 AS step, 'Lead' AS stage, count(*) AS leads FROM leads.smb_leads
  UNION ALL SELECT 2, 'Contacted',      count(contacted_date)      FROM leads.smb_leads
  UNION ALL SELECT 3, 'Needs assessed', count(needs_assessed_date) FROM leads.smb_leads
  UNION ALL SELECT 4, 'Quote accepted', count(quote_accepted_date) FROM leads.smb_leads
  UNION ALL SELECT 5, 'KYC complete',   count(kyc_complete_date)   FROM leads.smb_leads
  UNION ALL SELECT 6, 'Activated',      count(activated_date)      FROM leads.smb_leads
)
SELECT step, stage, leads,
       round(100.0 * leads / NULLIF(lag(leads) OVER (ORDER BY step), 0), 1) AS pct_of_previous,
       round(100.0 * leads / first_value(leads) OVER (ORDER BY step), 1)    AS pct_of_leads
FROM s;

-- Performance by segment and by source. CAC = all acquisition spend in the group / activations.
CREATE OR REPLACE VIEW leads.v_segment_perf AS
SELECT CASE WHEN GROUPING(segment) = 0 THEN 'segment' ELSE 'source' END AS dimension,
       COALESCE(segment, lead_source) AS value,
       count(*) AS leads,
       count(*) FILTER (WHERE outcome = 'Activated') AS activated,
       round(100.0 * count(*) FILTER (WHERE outcome = 'Activated') / count(*), 1) AS conversion_pct,
       round(sum(acquisition_cost_inr)::numeric / NULLIF(count(*) FILTER (WHERE outcome = 'Activated'), 0)) AS cac_inr,
       round(avg(monthly_bundle_value_inr)) AS avg_monthly_bundle_inr,
       round(100.0 * count(*) FILTER (WHERE churned_within_6_months) / NULLIF(count(*) FILTER (WHERE outcome = 'Activated'), 0), 1) AS churn_pct,
       round(avg(nps_score), 1) AS avg_nps
FROM leads.smb_leads
GROUP BY GROUPING SETS ((segment), (lead_source));

-- Drop reasons grouped by what can fix them.
CREATE OR REPLACE VIEW leads.v_drop_reasons AS
SELECT drop_reason,
       CASE
         WHEN drop_reason IN ('Owner had no time for a visit', 'Visit not scheduled by sales',
                              'Quote too complex to understand', 'Quote took too long to arrive',
                              'Missing GST or Udyam document', 'Document details did not match',
                              'Found paperwork too long', 'Authorised signatory not available')
           THEN 'fixable_by_digital'
         WHEN drop_reason IN ('Unreachable after 3 attempts', 'Wrong or invalid number', 'Not interested on first call')
           THEN 'before_first_contact'
         WHEN drop_reason IN ('Price too high', 'Competitor offered cheaper plan')
           THEN 'pricing'
         ELSE 'out_of_scope'
       END AS bucket,
       count(*) AS leads,
       round(100.0 * count(*) / sum(count(*)) OVER (), 1) AS pct_of_drops
FROM leads.smb_leads
WHERE outcome = 'Dropped'
GROUP BY drop_reason;
