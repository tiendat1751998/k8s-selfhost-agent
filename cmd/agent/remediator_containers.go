package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Default services permitted for autonomous / manual systemd remediation.
var defaultAllowedServices = map[string]bool{
	"kubelet":           true,
	"containerd":        true,
	"docker":            true,
	"crio":              true,
	"postgresql":        true,
	"postgres":          true,
	"nginx":             true,
	"redis":             true,
	"redis-server":      true,
	"haproxy":           true,
	"keepalived":        true,
	"etcd":              true,
	"patroni":           true,
	"traefik":           true,
	"coredns":           true,
	"nats":              true,
	"minio":             true,
	"clickhouse-server": true,
}

// Critical OS and networking services that must NEVER be restarted.
var criticalServiceDenylist = map[string]bool{
	"sshd":             true,
	"ssh":              true,
	"networking":       true,
	"systemd-networkd": true,
	"NetworkManager":   true,
	"dbus":             true,
	"udev":             true,
	"init":             true,
	"systemd-journald": true,
	"systemd-logind":   true,
}

var (
	validServiceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._@-]+$`)
	validContainerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
)

// AddAllowedServices dynamically expands the allowed service names.
// Supports comma-separated strings. Validates format and enforces the hard denylist.
func (nr *NodeRemediator) AddAllowedServices(services []string) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	if nr.allowedServices == nil {
		nr.allowedServices = make(map[string]bool)
		for k, v := range defaultAllowedServices {
			nr.allowedServices[k] = v
		}
	}

	for _, s := range services {
		for _, part := range strings.Split(s, ",") {
			name := strings.TrimSpace(part)
			if name == "" || len(name) > 64 || strings.HasPrefix(name, "-") {
				continue
			}
			if !validServiceNamePattern.MatchString(name) {
				continue
			}
			if criticalServiceDenylist[name] {
				continue
			}
			nr.allowedServices[name] = true
		}
	}
}

// IsServiceAllowed checks whether a systemd service is safe to restart.
func (nr *NodeRemediator) IsServiceAllowed(service string) bool {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	service = strings.TrimSpace(service)
	if service == "" || len(service) > 64 || strings.HasPrefix(service, "-") {
		return false
	}
	if criticalServiceDenylist[service] {
		return false
	}
	if !validServiceNamePattern.MatchString(service) {
		return false
	}
	if nr.allowedServices != nil {
		return nr.allowedServices[service]
	}
	return defaultAllowedServices[service]
}

func validateContainerTarget(target string) error {
	if len(target) == 0 || len(target) > 128 {
		return fmt.Errorf("container target length must be between 1 and 128 characters")
	}
	if strings.HasPrefix(target, "-") {
		return fmt.Errorf("container target cannot start with hyphen")
	}
	if strings.Contains(target, "..") || strings.Contains(target, "/") || strings.Contains(target, "\\") {
		return fmt.Errorf("container target contains forbidden path characters")
	}
	if !validContainerIDPattern.MatchString(target) {
		return fmt.Errorf("container target %q contains forbidden characters", target)
	}
	return nil
}

func (nr *NodeRemediator) validateRequest(req RemediationRequest) error {
	switch req.Action {
	case "restart_service":
		target := strings.TrimSpace(req.Target)
		if !nr.IsServiceAllowed(target) {
			return fmt.Errorf("unsupported or disallowed service target %q", req.Target)
		}
	case "restart_container":
		target := strings.TrimSpace(req.Target)
		if err := validateContainerTarget(target); err != nil {
			return err
		}
	case "disk_pressure_cleanup", "cni_ipam_cleanup", "reap_zombies":
		// valid
	default:
		return fmt.Errorf("unsupported action %q: only restart_service, restart_container, disk_pressure_cleanup, cni_ipam_cleanup, reap_zombies allowed", req.Action)
	}
	return nil
}

func validateRemediationRequest(req RemediationRequest) error {
	r := NewNodeRemediator()
	return r.validateRequest(req)
}

// executeRestartContainer restarts a Docker or container runtime container.
func (nr *NodeRemediator) executeRestartContainer(ctx context.Context, req RemediationRequest, res *RemediationActionResult) error {
	target := strings.TrimSpace(req.Target)
	if err := validateContainerTarget(target); err != nil {
		return err
	}

	key := "restart_container:" + target
	if err := nr.checkAndRecordCooldown(key); err != nil {
		return err
	}

	out, err := nr.runner.Run(ctx, "docker", "restart", target)
	if err != nil {
		nerdOut, nerdErr := nr.runner.Run(ctx, "nerdctl", "restart", target)
		if nerdErr == nil {
			out = nerdOut
			err = nil
		}
	}
	res.Details = string(out)
	if err != nil {
		return fmt.Errorf("failed to restart container %s: %w (%s)", target, err, string(out))
	}
	return nil
}

// detectUnhealthyContainers queries docker for unhealthy and crash-looping containers.
func detectUnhealthyContainers(ctx context.Context, runner CommandRunner) []UnhealthyContainerInfo {
	if runner == nil {
		runner = &osCommandRunner{}
	}

	results := make([]UnhealthyContainerInfo, 0)
	seen := make(map[string]bool)

	parseLines := func(output []byte) {
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) < 5 {
				continue
			}
			id := strings.TrimSpace(parts[0])
			name := strings.TrimSpace(parts[1])
			name = strings.TrimPrefix(name, "/")
			image := strings.TrimSpace(parts[2])
			state := strings.TrimSpace(parts[3])
			status := strings.TrimSpace(parts[4])

			// Skip k8s pause containers and images
			if strings.HasPrefix(name, "k8s_POD_") || strings.Contains(image, "pause") {
				continue
			}

			// If state is exited, check if it's non-zero
			if state == "exited" || strings.HasPrefix(status, "Exited") {
				if strings.Contains(status, "Exited (0)") {
					continue // Normal exit or one-shot job
				}
			}

			if seen[id] {
				continue
			}
			seen[id] = true

			results = append(results, UnhealthyContainerInfo{
				ID:           id,
				Name:         name,
				Image:        image,
				State:        state,
				Status:       status,
				RestartCount: 0,
			})
		}
	}

	// 1. Containers reporting health=unhealthy
	if out, err := runner.Run(ctx, "docker", "ps", "-a", "--filter", "health=unhealthy", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Status}}"); err == nil {
		parseLines(out)
	}

	// 2. Containers reporting status=exited (non-zero exit)
	if out, err := runner.Run(ctx, "docker", "ps", "-a", "--filter", "status=exited", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Status}}"); err == nil {
		parseLines(out)
	}

	return results
}
