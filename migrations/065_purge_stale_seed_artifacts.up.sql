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

-- 4. Purge stale metric rollups referencing deleted or synthetic seed hostnames
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE tablename = 'node_metric_rollups') THEN
        EXECUTE 'DELETE FROM node_metric_rollups 
                 WHERE node_name IN (''masterdb'', ''k8sworker3'', ''worker2'', ''worker1'', ''workerdb1'', ''test-qa-incident'')
                    OR node_id IN (''13bc198b-a491-4692-9ba3-aba1909f0d8c'', ''59f43036-ba45-4de9-8958-c95c8af4d8c6'', ''c2cf1210-2a89-4fed-9fd7-70ffa340584d'', ''870c47f1-902e-42fb-a491-ab57a45fa9f9'', ''9d85e3d2-bd8a-44d0-b226-3943149aee40'')
                    OR node_name LIKE ''Test Full Day%''
                    OR node_id LIKE ''test-node%''';
    END IF;
    IF EXISTS (SELECT FROM pg_tables WHERE tablename = 'node_metrics') THEN
        EXECUTE 'DELETE FROM node_metrics 
                 WHERE node_name IN (''masterdb'', ''k8sworker3'', ''worker2'', ''worker1'', ''workerdb1'', ''test-qa-incident'')
                    OR node_id IN (''13bc198b-a491-4692-9ba3-aba1909f0d8c'', ''59f43036-ba45-4de9-8958-c95c8af4d8c6'', ''c2cf1210-2a89-4fed-9fd7-70ffa340584d'', ''870c47f1-902e-42fb-a491-ab57a45fa9f9'', ''9d85e3d2-bd8a-44d0-b226-3943149aee40'')';
    END IF;
END $$;

