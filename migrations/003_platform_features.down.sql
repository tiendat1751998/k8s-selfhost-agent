-- Migration: 003_platform_features.down.sql
-- Description: Drops platform features tables (service_catalog, deployment_events, compliance_violations, compliance_frameworks).

DROP TABLE IF EXISTS service_catalog CASCADE;
DROP TABLE IF EXISTS deployment_events CASCADE;
DROP TABLE IF EXISTS compliance_violations CASCADE;
DROP TABLE IF EXISTS compliance_frameworks CASCADE;
