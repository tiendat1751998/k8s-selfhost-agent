package compliance

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DetectRuntimeFindings inspects live daemons, exposed ports, and runtime security posture across the fleet.
func DetectRuntimeFindings(ctx context.Context, sc *FleetComplianceScanner, hosts []ComputeHostRow) []AuditFindingRecord {
	now := time.Now().UTC()
	var findings []AuditFindingRecord

	for _, h := range hosts {
		hostIP := sc.resolveHostIP(h)
		token := sc.resolveAuthToken(h)
		baseURL := sc.resolveBaseURL(h)

		// 1. Process table & container runtime health
		var health agentHealthData
		if err := sc.fetchAgentJSON(ctx, baseURL+"/k8s/health", token, &health); err == nil {
			if health.ZombieCount > 0 {
				findings = append(findings, AuditFindingRecord{
					ID:          uuid.NewSHA1(uuid.NameSpaceOID, []byte("RUNTIME-PROC-ZOMBIE-"+h.Name)).String(),
					Category:    "process_table",
					Severity:    "medium",
					Description: fmt.Sprintf("[RUNTIME-PROC-ZOMBIE] %d defunct zombie process(es) detected in host process table on %s", health.ZombieCount, h.Name),
					Remediation: fmt.Sprintf("Trigger automated zombie reap via node agent :9100/remediate or execute 'kill -s SIGCHLD <parent_pid>' on %s.", h.Name),
					Status:      "open",
					DetectedAt:  now,
				})
			}
			if !health.ContainerdHealthy && !health.DockerHealthy {
				findings = append(findings, AuditFindingRecord{
					ID:          uuid.NewSHA1(uuid.NameSpaceOID, []byte("RUNTIME-CR-DOWN-"+h.Name)).String(),
					Category:    "container_runtime",
					Severity:    "critical",
					Description: fmt.Sprintf("[RUNTIME-CR-DOWN] Container runtime daemon (containerd/dockerd) is inactive or unhealthy on %s", h.Name),
					Remediation: fmt.Sprintf("Restart container runtime daemon on %s: 'systemctl restart containerd' or inspect /var/log/containerd.", h.Name),
					Status:      "open",
					DetectedAt:  now,
				})
			}
		}

		// 2. Host storage pressure
		var metrics agentMetricsData
		if err := sc.fetchAgentJSON(ctx, baseURL+"/metrics", token, &metrics); err == nil {
			rootDiskUsed := 0.0
			for _, d := range metrics.Disks {
				if d.MountPoint == "/" {
					rootDiskUsed = d.UsagePercent
					break
				}
			}
			if rootDiskUsed >= 85.0 {
				findings = append(findings, AuditFindingRecord{
					ID:          uuid.NewSHA1(uuid.NameSpaceOID, []byte("RUNTIME-FS-ROOT-"+h.Name)).String(),
					Category:    "runtime_security",
					Severity:    "high",
					Description: fmt.Sprintf("[RUNTIME-FS-ROOT] Root filesystem usage is %.1f%% on %s (exceeds 85%% safe threshold)", rootDiskUsed, h.Name),
					Remediation: fmt.Sprintf("Trigger disk garbage collection via :9100/remediate or execute 'docker system prune -af' / 'crictl rmi --prune' on %s.", h.Name),
					Status:      "open",
					DetectedAt:  now,
				})
			}
		}

		// 3. Kubelet API LAN exposure check (:10250)
		if sc.isPortOpen(ctx, hostIP, sc.kubeletProbePort) {
			findings = append(findings, AuditFindingRecord{
				ID:          uuid.NewSHA1(uuid.NameSpaceOID, []byte("RUNTIME-K8S-10250-"+hostIP)).String(),
				Category:    "kubelet_api",
				Severity:    "medium",
				Description: fmt.Sprintf("[RUNTIME-K8S-10250] Kubelet HTTPS API :10250 exposed on LAN interface %s (%s)", hostIP, h.Name),
				Remediation: fmt.Sprintf("Restrict Kubelet :10250 with iptables/ufw on %s to control plane nodes only, and enforce anonymous-auth=false.", h.Name),
				Status:      "open",
				DetectedAt:  now,
			})
		}

		// 4. Control Plane API server (:6443) reachability on master nodes
		if strings.Contains(h.Name, "master") || hostIP == "10.10.10.60" {
			if !sc.isPortOpen(ctx, hostIP, sc.apiServerProbePort) {
				findings = append(findings, AuditFindingRecord{
					ID:          uuid.NewSHA1(uuid.NameSpaceOID, []byte("RUNTIME-CP-6443-"+hostIP)).String(),
					Category:    "control_plane",
					Severity:    "high",
					Description: fmt.Sprintf("[RUNTIME-CP-6443] Kubernetes API server :6443 unreachable on master node %s (%s)", h.Name, hostIP),
					Remediation: fmt.Sprintf("Inspect kube-apiserver status on %s: 'systemctl status kube-apiserver' or inspect /var/log/pods.", h.Name),
					Status:      "open",
					DetectedAt:  now,
				})
			}
		}

		// 5. Redis service LAN exposure (:6379)
		if hostIP == "10.10.10.133" || sc.isPortOpen(ctx, hostIP, sc.redisProbePort) {
			if sc.isPortOpen(ctx, hostIP, sc.redisProbePort) {
				findings = append(findings, AuditFindingRecord{
					ID:          uuid.NewSHA1(uuid.NameSpaceOID, []byte("RUNTIME-NET-6379-"+hostIP)).String(),
					Category:    "database_network",
					Severity:    "high",
					Description: fmt.Sprintf("[RUNTIME-NET-6379] Redis service port 6379 exposed on LAN interface %s without network isolation or TLS encryption", hostIP),
					Remediation: fmt.Sprintf("Bind Redis daemon on %s to 127.0.0.1 in /etc/redis/redis.conf, enforce requirepass authentication, and apply ufw firewall rules.", h.Name),
					Status:      "open",
					DetectedAt:  now,
				})
			}
		}
	}

	// 6. Global PostgreSQL transport TLS enforcement check
	sslMode := strings.ToLower(strings.TrimSpace(os.Getenv("POSTGRES_SSLMODE")))
	if sslMode == "" {
		sslMode = strings.ToLower(strings.TrimSpace(os.Getenv("K8S_POSTGRES_SSLMODE")))
	}
	if sslMode == "disable" || sslMode == "" {
		findings = append(findings, AuditFindingRecord{
			ID:          uuid.NewSHA1(uuid.NameSpaceOID, []byte("RUNTIME-TLS-5432")).String(),
			Category:    "database_transport",
			Severity:    "medium",
			Description: "[RUNTIME-TLS-5432] PostgreSQL database cluster connection operating without TLS enforcement (sslmode=disable)",
			Remediation: "Set sslmode=require or sslmode=verify-full in K8S_DATABASE_URL and configure ssl=on with valid server.crt/server.key in postgresql.conf.",
			Status:      "open",
			DetectedAt:  now,
		})
	}

	return findings
}
