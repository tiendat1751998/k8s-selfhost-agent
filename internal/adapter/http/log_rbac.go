package http

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// isPlatformAdmin checks if the caller in the context holds the platform_admin role.
func isPlatformAdmin(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return tenancy.UserRoleFromContext(ctx) == tenancy.PlatformAdminRole
}

// CheckLogAccessRBAC validates that the requested namespace and service comply with RBAC and security policy:
// 1. Blocks sensitive OS log sources (auth, sudo, sshd, kern, etc.) with 403 Forbidden for all callers.
// 2. Blocks host journalctl and host daemon logs for non-platform_admin callers with 403 Forbidden.
// Returns HTTP status code (0 if allowed) and human-readable error message.
func CheckLogAccessRBAC(ctx context.Context, namespace, service string) (int, string) {
	service = strings.TrimSpace(service)
	namespace = strings.TrimSpace(namespace)

	// Block sensitive OS logs unconditionally for ALL users including platform_admin
	if service != "" && domainLogging.IsSensitiveOSLogSource(service, "", "") {
		return http.StatusForbidden, "forbidden: access to sensitive OS log source is blocked"
	}

	// Platform admin has full access to host system daemons and journalctl (excluding sensitive OS sources above)
	if isPlatformAdmin(ctx) {
		return 0, ""
	}

	// Non-platform_admin: strictly block journalctl alias
	if domainLogging.IsJournalctlAlias(service) {
		return http.StatusForbidden, "forbidden: access to host journalctl logs requires platform_admin role"
	}

	// Non-platform_admin: strictly block host namespace
	if strings.EqualFold(namespace, domainLogging.HostNamespace) {
		return http.StatusForbidden, "forbidden: access to host logs requires platform_admin role"
	}

	// Non-platform_admin: strictly block host system services
	if service != "" && domainLogging.IsHostSystemLog(namespace, service) {
		return http.StatusForbidden, "forbidden: access to host daemon logs requires platform_admin role"
	}

	return 0, ""
}

// SanitizeServicesCatalog filters services and byNode maps according to RBAC:
// 1. Strips 100% of IsSensitiveOSLogSource and individual host system units (e.g. kubelet.service, cron.service).
// 2. Retains pure application / container / pod services.
// 3. For platform_admin only: appends "journalctl" once to targets/services and once per node in byNode.
// 4. For non-platform_admin: ensures "journalctl" is excluded. If tenant has 0 nodes, returns empty slice and empty map.
func SanitizeServicesCatalog(ctx context.Context, allServices []string, byNode map[string][]string) ([]string, map[string][]string) {
	admin := isPlatformAdmin(ctx)

	cleanedByNode := make(map[string][]string, len(byNode))
	serviceSet := make(map[string]struct{})

	for node, svcs := range byNode {
		nodeCleaned := make([]string, 0, len(svcs))
		for _, s := range svcs {
			s = strings.TrimSpace(s)
			if s == "" || domainLogging.IsSensitiveOSLogSource(s, "", "") || domainLogging.IsHostSystemLog("", s) {
				continue
			}
			if domainLogging.IsJournalctlAlias(s) {
				continue
			}
			nodeCleaned = append(nodeCleaned, s)
			serviceSet[s] = struct{}{}
		}
		if admin {
			nodeCleaned = append(nodeCleaned, "journalctl")
			serviceSet["journalctl"] = struct{}{}
		}
		sort.Strings(nodeCleaned)
		cleanedByNode[node] = nodeCleaned
	}

	if len(byNode) == 0 && len(allServices) > 0 {
		for _, s := range allServices {
			s = strings.TrimSpace(s)
			if s == "" || domainLogging.IsSensitiveOSLogSource(s, "", "") || domainLogging.IsHostSystemLog("", s) || domainLogging.IsJournalctlAlias(s) {
				continue
			}
			serviceSet[s] = struct{}{}
		}
		if admin {
			serviceSet["journalctl"] = struct{}{}
		}
	}

	resultServices := make([]string, 0, len(serviceSet))
	for s := range serviceSet {
		resultServices = append(resultServices, s)
	}
	sort.Strings(resultServices)

	return resultServices, cleanedByNode
}

func parseStreamLogLine(line string) (time.Time, string) {
	if idx := strings.IndexByte(line, ' '); idx >= 19 && idx <= 35 {
		rawTS := line[:idx]
		if t, err := time.Parse(time.RFC3339Nano, rawTS); err == nil {
			return t.UTC(), strings.TrimSpace(line[idx+1:])
		}
		if t, err := time.Parse(time.RFC3339, rawTS); err == nil {
			return t.UTC(), strings.TrimSpace(line[idx+1:])
		}
	}
	return time.Now().UTC(), line
}

func extractStreamServiceTag(msg string) (string, string) {
	trimmed := strings.TrimSpace(msg)
	if !strings.HasPrefix(trimmed, "[") {
		return "", msg
	}
	endIdx := strings.IndexByte(trimmed, ']')
	if endIdx <= 1 {
		return "", msg
	}
	candidate := trimmed[1:endIdx]
	upper := strings.ToUpper(candidate)
	if upper == "INFO" || upper == "WARN" || upper == "WARNING" || upper == "ERROR" || upper == "DEBUG" || upper == "FATAL" || upper == "TRACE" || upper == "PANIC" {
		return "", msg
	}
	rest := strings.TrimSpace(trimmed[endIdx+1:])
	if strings.HasPrefix(rest, "[") {
		if lvlEnd := strings.IndexByte(rest, ']'); lvlEnd > 1 {
			lvl := strings.ToUpper(rest[1:lvlEnd])
			if lvl == "INFO" || lvl == "WARN" || lvl == "WARNING" || lvl == "ERROR" || lvl == "DEBUG" || lvl == "FATAL" || lvl == "TRACE" || lvl == "PANIC" {
				rest = strings.TrimSpace(rest[lvlEnd+1:])
			}
		}
	}
	if rest == "" {
		rest = msg
	}
	return candidate, rest
}

func detectStreamLogLevel(msg string) string {
	if inferred := domainLogging.InferSeverityFromMessage(msg); inferred != "" {
		return strings.ToUpper(string(inferred))
	}
	u := strings.ToUpper(msg)
	switch {
	case strings.Contains(u, "ERROR") || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC"):
		return "ERROR"
	case strings.Contains(u, "WARN"):
		return "WARN"
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE"):
		return "DEBUG"
	default:
		return "INFO"
	}
}

// processRemoteStreamLine parses a raw line received from a remote agent, applies security/isolation filters,
// redacts sensitive tokens, and returns a LogEntry to ingest, or nil if the line should be dropped.
func processRemoteStreamLine(
	line, targetService, namespace, node, tenantID string,
	isAdmin bool,
	maskFunc func(string) string,
) *logging.LogEntry {
	entryTS, msg := parseStreamLogLine(line)
	extractedSvc, rest := extractStreamServiceTag(msg)

	svc := targetService
	cleanMsg := msg
	if extractedSvc != "" {
		if svc == "" {
			svc = extractedSvc
		}
		cleanMsg = rest
	}

	// Always drop sensitive OS log sources immediately (auth, sudo, sshd, etc.)
	if domainLogging.IsSensitiveOSLogSource(extractedSvc, "", "") || domainLogging.IsSensitiveOSLogSource(svc, "", "") {
		return nil
	}

	entryTenantID := tenantID
	entryNS := namespace

	if domainLogging.IsJournalctlAlias(targetService) {
		if extractedSvc != "" {
			svc = extractedSvc
		} else {
			svc = "journalctl"
		}
		// Drop non-host container lines (e.g. [nginx], [db])
		if extractedSvc != "" && !domainLogging.IsHostSystemLog("", extractedSvc) {
			return nil
		}
		entryNS = domainLogging.HostNamespace
		if domainLogging.IsSensitiveOSLogSource(svc, "", "") {
			return nil
		}
	} else if strings.TrimSpace(targetService) == "" {
		// All Apps: drop host daemon lines so they never leak over the wire
		if domainLogging.IsHostSystemLog(entryNS, svc) {
			return nil
		}
	} else {
		// Specific container requested: drop unexpected host system lines
		if domainLogging.IsHostSystemLog(entryNS, svc) {
			return nil
		}
	}

	if domainLogging.IsHostSystemLog(entryNS, svc) {
		if entryNS == "" {
			entryNS = domainLogging.HostNamespace
		}
		if !isAdmin {
			entryTenantID = ""
		}
	}

	if maskFunc != nil {
		cleanMsg = maskFunc(cleanMsg)
	}

	return &logging.LogEntry{
		Timestamp: entryTS,
		TenantID:  entryTenantID,
		Namespace: entryNS,
		Pod:       svc,
		Container: svc,
		Service:   svc,
		Node:      node,
		Stream:    "stdout",
		Level:     detectStreamLogLevel(msg),
		Message:   cleanMsg,
	}
}


