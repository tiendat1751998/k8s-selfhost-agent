-- Deduplicate active incidents keeping only the latest one per (namespace, pod_name, type)
DELETE FROM incidents
WHERE id NOT IN (
    SELECT DISTINCT ON (namespace, pod_name, type) id
    FROM incidents
    WHERE status != 'resolved'
    ORDER BY namespace, pod_name, type, created_at DESC
) AND status != 'resolved';
