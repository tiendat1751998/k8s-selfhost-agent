package metrics

import (
	"strings"
)

// extractServiceName retrieves or derives the service name for a container.
func extractServiceName(cm ContainerMetrics) string {
	if cm.ServiceName != "" {
		return cm.ServiceName
	}
	if cm.Labels != nil {
		if sn, ok := cm.Labels["com.docker.swarm.service.name"]; ok && sn != "" {
			return sn
		}
		if sn, ok := cm.Labels["com.docker.compose.service"]; ok && sn != "" {
			return sn
		}
	}
	name := strings.TrimPrefix(cm.ContainerName, "/")
	if name == "" {
		name = cm.ContainerID
		if len(name) > 12 {
			name = name[:12]
		}
		return name
	}
	if parts := strings.Split(name, "."); len(parts) >= 3 {
		return parts[0]
	}
	return name
}

// isTraefikService checks whether a service is the Traefik ingress proxy / gateway.
func isTraefikService(name string) bool {
	low := strings.ToLower(strings.TrimSpace(name))
	norm := normalizeServiceNameForLB(name)
	return norm == "traefik" || strings.Contains(low, "traefik") || norm == "ingress" || low == "ingress" || low == "ingress-controller"
}

// isDatabaseService checks whether a service is a database service.
func isDatabaseService(name string) bool {
	low := strings.ToLower(strings.TrimSpace(name))
	norm := normalizeServiceNameForLB(name)
	return norm == "db" || norm == "database" || norm == "postgres" || norm == "postgres_db" || norm == "postgresql" || norm == "mysql" || norm == "mariadb" ||
		strings.Contains(low, "postgres") || strings.HasSuffix(low, "_db") || strings.HasSuffix(low, "-db") || low == "db" || low == "database" || strings.Contains(low, "mysql")
}

// isMessagingService checks whether a service is a messaging or event broker service.
func isMessagingService(name string) bool {
	low := strings.ToLower(strings.TrimSpace(name))
	norm := normalizeServiceNameForLB(name)
	return norm == "nats" || norm == "kafka" || norm == "rabbitmq" ||
		strings.Contains(low, "nats") || strings.Contains(low, "kafka") || strings.Contains(low, "rabbitmq") || strings.Contains(low, "broker")
}

// isCacheService checks whether a service is a cache or in-memory store.
func isCacheService(name string) bool {
	low := strings.ToLower(strings.TrimSpace(name))
	norm := normalizeServiceNameForLB(name)
	return norm == "redis" || norm == "memcached" || strings.Contains(low, "redis") || strings.Contains(low, "memcached")
}

// normalizeServiceNameForLB normalizes a service name by stripping load balancer and cluster prefixes/suffixes.
func normalizeServiceNameForLB(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if idx := strings.Index(name, "@"); idx != -1 {
		name = name[:idx]
	}
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimPrefix(name, "tiki_")
	name = strings.TrimPrefix(name, "tiki-")
	name = strings.TrimPrefix(name, "k8s_")
	name = strings.TrimPrefix(name, "k8s-")
	return name
}
