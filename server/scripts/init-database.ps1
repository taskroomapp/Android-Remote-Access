# Creates PostgreSQL role + database for Android Remote Access.
# Usage (PowerShell, from server/):
#   $env:PGPASSWORD = 'your-postgres-superuser-password'
#   .\scripts\init-database.ps1 -PostgresUser postgres -AppPassword 'eggarf123'

param(
    [string]$PostgresUser = "postgres",
    [string]$PostgresHost = "localhost",
    [int]$PostgresPort = 5432,
    [Parameter(Mandatory = $true)]
    [string]$AppPassword
)

$ErrorActionPreference = "Stop"
$psql = "psql"
if (Test-Path "C:\Program Files\PostgreSQL\18\bin\psql.exe") {
    $psql = "C:\Program Files\PostgreSQL\18\bin\psql.exe"
}

$sql = @"
DO `$`$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'remote_access') THEN
    CREATE ROLE remote_access LOGIN PASSWORD '$($AppPassword -replace "'", "''")';
  ELSE
    ALTER ROLE remote_access PASSWORD '$($AppPassword -replace "'", "''")';
  END IF;
END `$`$;
"@

& $psql -U $PostgresUser -h $PostgresHost -p $PostgresPort -d postgres -v ON_ERROR_STOP=1 -c $sql

$dbExists = & $psql -U $PostgresUser -h $PostgresHost -p $PostgresPort -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='android_remote_access'"
if (-not ($dbExists -and $dbExists.Trim())) {
    & $psql -U $PostgresUser -h $PostgresHost -p $PostgresPort -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE android_remote_access OWNER remote_access;"
    Write-Host "Created database android_remote_access"
} else {
    Write-Host "Database android_remote_access already exists"
}

& $psql -U $PostgresUser -h $PostgresHost -p $PostgresPort -d android_remote_access -v ON_ERROR_STOP=1 -c @"
GRANT ALL ON SCHEMA public TO remote_access;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO remote_access;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO remote_access;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
"@

Write-Host "Done. Connection string (local dev, no TLS on Postgres):"
Write-Host "postgres://remote_access:***@localhost:5432/android_remote_access?sslmode=disable"
