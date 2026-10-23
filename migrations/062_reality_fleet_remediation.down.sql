-- Migration: 062_reality_fleet_remediation.down.sql
-- Description: Reverts the reality fleet remediation seeds.

DELETE FROM automation_rules WHERE id::text LIKE 'e1000001%';
DELETE FROM compliance_violations WHERE id::text LIKE 'd1000001%';
DELETE FROM compliance_frameworks WHERE id::text LIKE 'c1000001%';
DELETE FROM slo_snapshots WHERE id::text LIKE 'b1000001%';
DELETE FROM slo_definitions WHERE id::text LIKE 'a1000001%';
