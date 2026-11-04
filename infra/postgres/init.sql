-- One database, one schema per service. Services only touch their own schema
-- (search_path is set per service); cross-schema reads are not allowed by convention
-- and Phase 8 enforces it with per-service roles.
CREATE SCHEMA IF NOT EXISTS leads;      -- case leads (loaded in Phase 1), read-only reference data
CREATE SCHEMA IF NOT EXISTS catalogue;  -- owned by rules service
CREATE SCHEMA IF NOT EXISTS registry;   -- owned by registry service
CREATE SCHEMA IF NOT EXISTS advisor;
CREATE SCHEMA IF NOT EXISTS quote;
CREATE SCHEMA IF NOT EXISTS kyc;
CREATE SCHEMA IF NOT EXISTS tracker;
CREATE SCHEMA IF NOT EXISTS notify;
CREATE SCHEMA IF NOT EXISTS scoring;
