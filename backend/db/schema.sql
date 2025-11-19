-- Lucendex Database Initialization Script
-- Creates schemas, roles, and base tables

-- Create schemas
CREATE SCHEMA IF NOT EXISTS core;
CREATE SCHEMA IF NOT EXISTS metering;

-- Create roles with least-privilege access
-- indexer_rw: Read/write for indexer
CREATE ROLE indexer_rw WITH LOGIN PASSWORD :'indexer_password';
GRANT CONNECT ON DATABASE lucendex TO indexer_rw;
GRANT USAGE ON SCHEMA core TO indexer_rw;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA core TO indexer_rw;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA core TO indexer_rw;
ALTER DEFAULT PRIVILEGES IN SCHEMA core GRANT ALL PRIVILEGES ON TABLES TO indexer_rw;
ALTER DEFAULT PRIVILEGES IN SCHEMA core GRANT ALL PRIVILEGES ON SEQUENCES TO indexer_rw;

-- router_ro: Read-only for router
CREATE ROLE router_ro WITH LOGIN PASSWORD :'router_password';
GRANT CONNECT ON DATABASE lucendex TO router_ro;
GRANT USAGE ON SCHEMA core TO router_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA core TO router_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA core GRANT SELECT ON TABLES TO router_ro;

-- api_ro: Read-only for API handlers + partner management
CREATE ROLE api_ro WITH LOGIN PASSWORD :'api_password';
GRANT CONNECT ON DATABASE lucendex TO api_ro;
GRANT USAGE ON SCHEMA core TO api_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA core TO api_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA core GRANT SELECT ON TABLES TO api_ro;
GRANT USAGE ON SCHEMA metering TO api_ro;
GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA metering TO api_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA metering GRANT SELECT, INSERT ON TABLES TO api_ro;
-- Partner API tables in public schema
GRANT USAGE ON SCHEMA public TO api_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO api_ro;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO api_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO api_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO api_ro;

-- Enable Row-Level Security (RLS) extension
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Create audit logging function
CREATE OR REPLACE FUNCTION core.audit_trigger_func()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'INSERT') THEN
        NEW.created_at = COALESCE(NEW.created_at, now());
        NEW.updated_at = now();
        RETURN NEW;
    ELSIF (TG_OP = 'UPDATE') THEN
        NEW.updated_at = now();
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Success message
DO $$ 
BEGIN
    RAISE NOTICE 'Database initialization complete';
    RAISE NOTICE 'Schemas: core, metering';
    RAISE NOTICE 'Roles: indexer_rw, router_ro, api_ro';
END $$;
