-- Run as PostgreSQL superuser (e.g. postgres):
--   psql -U postgres -f scripts/init-database.sql
--
-- Set password before running (must match server/config.yaml database.url):
\set app_password 'eggarf123'

-- Role
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'remote_access') THEN
    EXECUTE format('CREATE ROLE remote_access LOGIN PASSWORD %L', :'app_password');
  ELSE
    EXECUTE format('ALTER ROLE remote_access PASSWORD %L', :'app_password');
  END IF;
END
$$;

-- Database
SELECT 'CREATE DATABASE android_remote_access OWNER remote_access'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'android_remote_access')\gexec

GRANT ALL PRIVILEGES ON DATABASE android_remote_access TO remote_access;

\c android_remote_access

GRANT ALL ON SCHEMA public TO remote_access;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO remote_access;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO remote_access;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
