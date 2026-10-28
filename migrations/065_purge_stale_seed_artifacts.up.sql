-- Migration: 065_purge_stale_seed_artifacts.up.sql
-- Description: Purge stale incidents, fake placeholder Slack webhook channels, and align compliance framework mappings.

-- 1. Purge stale seed incidents referencing deleted hostnames or QA testing artifacts
DELETE FROM incidents
WHERE pod_name IN ('masterdb', 'k8sworker3', 'worker2', 'test-qa-incident')
   OR id::text LIKE '%test-qa-incident%'
   OR message LIKE '%test-qa-incident%'
   OR message LIKE '%masterdb%'
   OR message LIKE '%k8sworker3%'
   OR message LIKE '%worker2%'
   OR raw_data::text LIKE '%masterdb%'
   OR raw_data::text LIKE '%k8sworker3%'
   OR raw_data::text LIKE '%worker2%';

-- 2. Purge fake placeholder Slack webhook channels (seeded with dummy T00000000)
DELETE FROM notification_channels
WHERE webhook_url LIKE 'https://hooks.slack.com/services/T00000000/%'
   OR name = 'SRE Ops Channel'
   OR config::text LIKE '%T00000000%';

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE tablename = 'alert_channels') THEN
        EXECUTE 'DELETE FROM alert_channels WHERE webhook_url LIKE ''https://hooks.slack.com/services/T00000000/%'' OR name = ''SRE Ops Channel'' OR config::text LIKE ''%T00000000%''';
    END IF;
END $$;

-- 3. Add framework column and synchronize compliance violations framework mappings
ALTER TABLE compliance_violations ADD COLUMN IF NOT EXISTS framework VARCHAR(50) DEFAULT '';

UPDATE compliance_violations
SET framework = 'NSA-CISA',
    framework_id = 'c1000001-0000-0000-0000-000000000002'
WHERE policy LIKE 'NSA-CISA%' 
   OR message LIKE 'NSA-CISA%'
   OR policy LIKE 'NSA%';

UPDATE compliance_violations
SET framework = 'SOC 2',
    framework_id = 'c1000001-0000-0000-0000-000000000003'
WHERE policy LIKE 'SOC 2%' 
   OR message LIKE 'SOC 2%';

UPDATE compliance_violations
SET framework = 'CIS',
    framework_id = 'c1000001-0000-0000-0000-000000000001'
WHERE policy LIKE 'CIS%' 
   OR message LIKE 'CIS%';
