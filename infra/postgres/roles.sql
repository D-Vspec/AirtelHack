-- Per-service Postgres roles (Phase 8). Idempotent: the db-roles compose job runs it on every
-- `make up`, so it also upgrades a database created before roles existed.
--
-- Each service connects as its own role and can use only its own schema. registry and rules read
-- reference data that `make seed` (the admin role) loads into registry and catalogue. No service
-- role can read another service's schema: Postgres refuses with "permission denied for schema".
-- The password comes from SERVICE_DB_PASSWORD (psql -v pw=...); production would use one secret
-- per role from a vault.

\set ON_ERROR_STOP on

DO $$
DECLARE
  svc text;
  pw text := current_setting('udyam.pw');
BEGIN
  FOREACH svc IN ARRAY ARRAY['registry', 'rules', 'quote', 'kyc', 'tracker', 'notify', 'scoring'] LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'svc_' || svc) THEN
      EXECUTE format('CREATE ROLE %I LOGIN PASSWORD %L', 'svc_' || svc, pw);
    ELSE
      EXECUTE format('ALTER ROLE %I LOGIN PASSWORD %L', 'svc_' || svc, pw);
    END IF;
  END LOOP;
END $$;

-- nobody but the owner creates objects in public
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- services that own their data: schema and every existing table/sequence/view go to the role
DO $$
DECLARE
  s text;
  r record;
BEGIN
  FOREACH s IN ARRAY ARRAY['quote', 'kyc', 'tracker', 'notify', 'scoring'] LOOP
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', s);
    EXECUTE format('ALTER SCHEMA %I OWNER TO %I', s, 'svc_' || s);
    FOR r IN SELECT c.relname, c.relkind FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = s AND c.relkind IN ('r', 'p', 'v', 'm', 'S')
               AND NOT (c.relkind = 'S' AND EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = c.oid AND d.deptype IN ('a', 'i'))) LOOP
      -- sequences owned by a column move with their table
      EXECUTE format('ALTER %s %I.%I OWNER TO %I',
        CASE r.relkind WHEN 'S' THEN 'SEQUENCE' WHEN 'v' THEN 'VIEW' WHEN 'm' THEN 'MATERIALIZED VIEW' ELSE 'TABLE' END,
        s, r.relname, 'svc_' || s);
    END LOOP;
  END LOOP;
END $$;

-- read-only reference data loaded by `make seed`
GRANT USAGE ON SCHEMA registry TO svc_registry;
GRANT SELECT ON ALL TABLES IN SCHEMA registry TO svc_registry;
ALTER DEFAULT PRIVILEGES IN SCHEMA registry GRANT SELECT ON TABLES TO svc_registry;

GRANT USAGE ON SCHEMA catalogue TO svc_rules;
GRANT SELECT ON ALL TABLES IN SCHEMA catalogue TO svc_rules;
ALTER DEFAULT PRIVILEGES IN SCHEMA catalogue GRANT SELECT ON TABLES TO svc_rules;
