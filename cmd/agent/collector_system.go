package main

import (
	"math"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (c *SystemCollector) collectUptime() int64 {
	uptimePath := filepath.Join(c.procPath, "uptime")
	data, err := os.ReadFile(uptimePath)
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if sec, err := strconv.ParseFloat(fields[0], 64); err == nil && sec >= 0 {
				return int64(sec)
			}
		}
	}
	return int64(time.Since(c.startTime).Seconds())
}

func (c *SystemCollector) collectLoadAvg() [3]float64 {
	loadPath := filepath.Join(c.procPath, "loadavg")
	data, err := os.ReadFile(loadPath)
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			l1, e1 := strconv.ParseFloat(fields[0], 64)
			l5, e2 := strconv.ParseFloat(fields[1], 64)
			l15, e3 := strconv.ParseFloat(fields[2], 64)
			if e1 == nil && e2 == nil && e3 == nil {
				return [3]float64{
					math.Round(l1*100) / 100,
					math.Round(l5*100) / 100,
					math.Round(l15*100) / 100,
				}
			}
		}
	}
	return [3]float64{0.0, 0.0, 0.0}
}

func (c *SystemCollector) collectProcesses() int {
	entries, err := os.ReadDir(c.procPath)
	if err == nil {
		var count int
		for _, entry := range entries {
			if entry.IsDir() {
				name := entry.Name()
				isNumeric := true
				for _, r := range name {
					if !unicode.IsDigit(r) {
						isNumeric = false
						break
					}
				}
				if isNumeric && len(name) > 0 {
					count++
				}
			}
		}
		if count > 0 {
			return count
		}
	}

	loadPath := filepath.Join(c.procPath, "loadavg")
	if data, err := os.ReadFile(loadPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 4 {
			threadsPart := fields[3]
			parts := strings.Split(threadsPart, "/")
			if len(parts) == 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
					return n
				}
			}
		}
	}

	return 1
}

func resolveUser(uidStr string) string {
	uidStr = strings.TrimSpace(uidStr)
	if uidStr == "" {
		return "unknown"
	}
	if uidStr == "0" {
		return "root"
	}
	if uidStr == "65534" {
		return "nobody"
	}
	if u, err := user.LookupId(uidStr); err == nil && u.Username != "" {
		return u.Username
	}
	return uidStr
}

func parseState(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "(") && strings.Contains(s, ")") {
		start := strings.Index(s, "(")
		end := strings.Index(s, ")")
		if end > start+1 {
			return strings.ToLower(strings.TrimSpace(s[start+1 : end]))
		}
	}
	if len(s) == 0 {
		return "unknown"
	}
	switch s[0] {
	case 'R':
		return "running"
	case 'S':
		return "sleeping"
	case 'D':
		return "disk sleep"
	case 'Z':
		return "zombie"
	case 'T':
		return "stopped"
	case 't':
		return "tracing stop"
	case 'X', 'x':
		return "dead"
	case 'K':
		return "wakekill"
	case 'W':
		return "waking"
	case 'P':
		return "parked"
	case 'I':
		return "idle"
	default:
		return strings.ToLower(s)
	}
}

// collectOSInfo retrieves the operating system distribution and kernel version.
func (c *SystemCollector) collectOSInfo() (string, string) {
	distro := c.readOSDistro()
	kernel := c.readKernelVersion()

	if distro == "" {
		switch runtime.GOOS {
		case "windows":
			distro = "Windows"
		case "darwin":
			distro = "macOS"
		default:
			distro = "Linux"
		}
	}

	if kernel == "" {
		switch runtime.GOOS {
		case "windows":
			kernel = runtime.GOARCH
		case "darwin":
			kernel = runtime.GOARCH
		default:
			kernel = "unknown"
		}
	}

	return distro, kernel
}

func (c *SystemCollector) readOSDistro() string {
	paths := []string{c.osReleasePath, "/usr/lib/os-release", "/etc/lsb-release"}
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		var name, version, prettyName, distribDesc string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			switch key {
			case "PRETTY_NAME":
				prettyName = val
			case "DISTRIB_DESCRIPTION":
				distribDesc = val
			case "NAME":
				name = val
			case "VERSION":
				version = val
			case "VERSION_ID":
				if version == "" {
					version = val
				}
			}
		}
		if prettyName != "" {
			return prettyName
		}
		if distribDesc != "" {
			return distribDesc
		}
		if name != "" && version != "" {
			return name + " " + version
		}
		if name != "" {
			return name
		}
	}

	// Fallback to /etc/redhat-release
	if data, err := os.ReadFile("/etc/redhat-release"); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			return content
		}
	}

	// Fallback to /etc/debian_version
	if data, err := os.ReadFile("/etc/debian_version"); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			return "Debian " + content
		}
	}

	return ""
}

func (c *SystemCollector) readKernelVersion() string {
	candidates := []string{
		filepath.Join(c.procPath, "sys", "kernel", "osrelease"),
		"/proc/sys/kernel/osrelease",
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if data, err := os.ReadFile(p); err == nil {
			k := strings.TrimSpace(string(data))
			if k != "" {
				return k
			}
		}
	}

	verCandidates := []string{
		filepath.Join(c.procPath, "version"),
		"/proc/version",
	}
	for _, p := range verCandidates {
		if p == "" {
			continue
		}
		if data, err := os.ReadFile(p); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= 3 && fields[0] == "Linux" && fields[1] == "version" {
				return fields[2]
			}
		}
	}

	return ""
}

var dockerEnvPath = "/.dockerenv"

// detectRuntimeEnvironment identifies whether the agent is running in Kubernetes, Docker, or bare metal.
func detectRuntimeEnvironment() string {
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return "kubernetes"
	}
	if _, err := os.Stat(dockerEnvPath); err == nil {
		return "docker"
	}
	return "bare-metal"
}

// detectHostRoleAndServices inspects processes to classify host role ("database", "compute", or "k8s")
// and returns any detected database services.
func detectHostRoleAndServices(topProcesses []ProcessMetric) (string, []string) {
	detectedMap := make(map[string]struct{})
	hasK8s := false
	hasCompute := false

	for _, p := range topProcesses {
		name := strings.ToLower(p.Name)
		cmd := strings.ToLower(p.CommandLine)

		// Check for database engines: postgres, mysqld, mariadbd, mongod, redis-server, clickhouse
		if strings.Contains(name, "postgres") || strings.Contains(cmd, "postgres") {
			detectedMap["postgres"] = struct{}{}
		}
		if strings.Contains(name, "mysqld") || strings.Contains(cmd, "mysqld") ||
			strings.Contains(name, "mysql") || strings.Contains(cmd, "mysql") {
			detectedMap["mysql"] = struct{}{}
		}
		if strings.Contains(name, "mariadbd") || strings.Contains(cmd, "mariadbd") ||
			strings.Contains(name, "mariadb") || strings.Contains(cmd, "mariadb") {
			detectedMap["mariadb"] = struct{}{}
		}
		if strings.Contains(name, "mongod") || strings.Contains(cmd, "mongod") ||
			strings.Contains(name, "mongodb") || strings.Contains(cmd, "mongodb") {
			detectedMap["mongodb"] = struct{}{}
		}
		if strings.Contains(name, "redis-server") || strings.Contains(cmd, "redis-server") ||
			strings.Contains(name, "redis") || strings.Contains(cmd, "redis") {
			detectedMap["redis"] = struct{}{}
		}
		if strings.Contains(name, "clickhouse") || strings.Contains(cmd, "clickhouse") {
			detectedMap["clickhouse"] = struct{}{}
		}

		// Check for k8s engines
		if strings.Contains(name, "kubelet") || strings.Contains(cmd, "kubelet") ||
			strings.Contains(name, "k3s") || strings.Contains(cmd, "k3s") {
			hasK8s = true
		}

		// Check for compute engines
		if strings.Contains(name, "containerd") || strings.Contains(cmd, "containerd") ||
			strings.Contains(name, "dockerd") || strings.Contains(cmd, "dockerd") {
			hasCompute = true
		}
	}

	if len(detectedMap) > 0 {
		services := make([]string, 0, len(detectedMap))
		for s := range detectedMap {
			services = append(services, s)
		}
		sort.Strings(services)
		return "database", services
	}

	if hasK8s {
		return "k8s", []string{}
	}

	if hasCompute {
		return "compute", []string{}
	}

	return "compute", []string{}
}
