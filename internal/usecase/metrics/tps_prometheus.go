package metrics

import (
	"context"
	"math"
	"strings"

	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
	"go.uber.org/zap"
)

// enrichServicesWithLBStats merges per-service LoadBalancer request statistics into ServiceTPS metrics
// and maps aliases so Traefik ingress and backend services (DB, Messaging) receive accurate stats.
func (c *TPSCollector) enrichServicesWithLBStats(ctx context.Context, services []ServiceTPS, httpTPS HTTPTPS, dbTPS DatabaseTPS, msgTPS MessagingTPS) []ServiceTPS {
	var lbStats []domainLB.ServiceRequestStats
	if c.lbProvider != nil {
		stats, err := c.lbProvider.GetServiceStats(ctx)
		if err == nil {
			lbStats = stats
		} else {
			c.logger.Debug("Failed to get load balancer service stats", zap.Error(err))
		}
	}

	// If httpTPS has 0 RequestsPerSec but lbStats has non-zero stats, sum them up
	var sumFromLBStats float64
	var errRateFromLBStats float64
	var countWithRPS int
	for _, s := range lbStats {
		if s.RequestsPerSec > 0 {
			sumFromLBStats += s.RequestsPerSec
			errRateFromLBStats += s.ErrorRate
			countWithRPS++
		}
	}
	if sumFromLBStats > 0 && httpTPS.RequestsPerSec == 0 {
		httpTPS.RequestsPerSec = math.Round(sumFromLBStats*100) / 100
		if countWithRPS > 0 && httpTPS.ErrorRate == 0 {
			httpTPS.ErrorRate = math.Round((errRateFromLBStats/float64(countWithRPS))*100) / 100
		}
	}

	// Map normalized and raw service names to LB stats
	statMap := make(map[string]domainLB.ServiceRequestStats, len(lbStats)*2)
	for _, s := range lbStats {
		norm := normalizeServiceNameForLB(s.ServiceName)
		statMap[norm] = s
		statMap[strings.ToLower(strings.TrimSpace(s.ServiceName))] = s
	}

	// Calculate total Rx traffic across all container services for fallback proportional estimation
	var totalRxTraffic int64
	for _, s := range services {
		if s.RxBytesPerSec > 0 {
			totalRxTraffic += s.RxBytesPerSec
		}
	}

	for i := range services {
		sName := services[i].ServiceName
		norm := normalizeServiceNameForLB(sName)

		stat, matched := findMatchingLBStat(sName, norm, statMap)
		if matched {
			services[i].RequestsPerSec = stat.RequestsPerSec
			services[i].ErrorRate = stat.ErrorRate
			services[i].AvgLatencyMs = stat.AvgLatencyMs
			services[i].MaxLatencyMs = stat.MaxLatencyMs
		}

		// 1. Ingress gateway mapping (tiki_traefik / traefik)
		if isTraefikService(sName) {
			services[i].RequestsPerSec = httpTPS.RequestsPerSec
			services[i].ErrorRate = httpTPS.ErrorRate
			services[i].AvgLatencyMs = httpTPS.AvgLatencyMs
			services[i].MaxLatencyMs = httpTPS.MaxLatencyMs
			continue
		}

		// 2. Database service mapping (postgres_db / db)
		if isDatabaseService(sName) && services[i].RequestsPerSec == 0 {
			if dbTPS.TransactionsPerSec > 0 {
				services[i].RequestsPerSec = math.Round(dbTPS.TransactionsPerSec*100) / 100
			}
			continue
		}

		// 3. Messaging service mapping (nats)
		if isMessagingService(sName) && services[i].RequestsPerSec == 0 {
			msgRate := msgTPS.InMsgsPerSec + msgTPS.OutMsgsPerSec
			if msgRate > 0 {
				services[i].RequestsPerSec = math.Round(msgRate*100) / 100
			}
			continue
		}

		// 4. Proportional fallback for backend services with active network traffic
		if services[i].RequestsPerSec == 0 && httpTPS.RequestsPerSec > 0 {
			if totalRxTraffic > 0 && services[i].RxBytesPerSec > 0 {
				ratio := float64(services[i].RxBytesPerSec) / float64(totalRxTraffic)
				services[i].RequestsPerSec = math.Round(httpTPS.RequestsPerSec*ratio*100) / 100
				if httpTPS.ErrorRate > 0 {
					services[i].ErrorRate = httpTPS.ErrorRate
				}
				if httpTPS.AvgLatencyMs > 0 {
					services[i].AvgLatencyMs = httpTPS.AvgLatencyMs
				}
				if httpTPS.MaxLatencyMs > 0 {
					services[i].MaxLatencyMs = httpTPS.MaxLatencyMs
				}
			}
		}
	}

	return services
}

// findMatchingLBStat searches for matching load balancer statistics using exact, normalized, alias, and fuzzy matching.
func findMatchingLBStat(sName string, norm string, statMap map[string]domainLB.ServiceRequestStats) (domainLB.ServiceRequestStats, bool) {
	if stat, ok := statMap[norm]; ok {
		return stat, true
	}
	rawLower := strings.ToLower(strings.TrimSpace(sName))
	if stat, ok := statMap[rawLower]; ok {
		return stat, true
	}

	var aliases []string
	switch {
	case isTraefikService(sName):
		aliases = []string{"traefik", "web", "websecure", "gateway", "web@file", "gateway@file", "websecure@file", "traefik@file"}
	case norm == "web" || strings.Contains(rawLower, "web"):
		aliases = []string{"web", "web@file", "websecure", "tiki_web", "frontend"}
	case norm == "gateway" || strings.Contains(rawLower, "gateway"):
		aliases = []string{"gateway", "gateway@file", "tiki_gateway", "api-gateway"}
	case isCacheService(sName):
		aliases = []string{"redis", "tiki_redis", "redis-cache"}
	case isDatabaseService(sName):
		aliases = []string{"db", "postgres_db", "database", "postgres", "postgresql", "psql"}
	case isMessagingService(sName):
		aliases = []string{"nats", "tiki_nats", "message_broker", "broker"}
	}

	for _, alias := range aliases {
		if stat, ok := statMap[alias]; ok {
			return stat, true
		}
		if stat, ok := statMap[normalizeServiceNameForLB(alias)]; ok {
			return stat, true
		}
	}

	for k, stat := range statMap {
		if norm != "" && k != "" && (strings.Contains(norm, k) || strings.Contains(k, norm)) {
			return stat, true
		}
	}

	return domainLB.ServiceRequestStats{}, false
}
