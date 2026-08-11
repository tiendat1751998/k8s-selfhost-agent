-- DOWN migration for 022_cost_backup_seeds.sql

DELETE FROM backup_history WHERE action IN ('backup', 'restore');
DELETE FROM resource_waste WHERE type IN ('Underutilized Pod', 'Orphaned PVC', 'Unused ConfigMap', 'Over-provisioned Deploy');
DELETE FROM namespace_costs WHERE namespace IN ('production', 'staging', 'monitoring', 'kube-system', 'dev-apps', 'qa-testing');
DELETE FROM cluster_costs WHERE name IN ('production-us-east', 'production-eu-west', 'dev-cluster-local');
DELETE FROM promotions WHERE service IN ('payment-gateway', 'auth-service', 'portal-frontend');
DELETE FROM correlated_events WHERE title IN ('Payment API Gateway Timeout Cascading Failure', 'High CPU on Authentication Service');

DROP TABLE IF EXISTS backup_history;
DROP TABLE IF EXISTS resource_waste;
DROP TABLE IF EXISTS namespace_costs;
DROP TABLE IF EXISTS cluster_costs;
