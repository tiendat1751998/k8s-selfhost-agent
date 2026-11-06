package http

import (
	"context"
	"net/http"
	"sort"
	"strings"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
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

