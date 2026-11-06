package logging

import "strings"

// HostNamespace is the canonical reserved namespace stamped on all host OS and journald entries.
const HostNamespace = "__host__"

// JournalctlTarget is the consolidated platform_admin target alias for systemd logs.
const JournalctlTarget = "journalctl"

// sensitiveOSIdentifiers contains normalized (lowercase, un-suffixed) names of auth/kernel security sources.
var sensitiveOSIdentifiers = map[string]struct{}{
	"auth":           {},
	"authpriv":       {},
	"secure":         {},
	"sshd":           {},
	"ssh":            {},
	"sudo":           {},
	"su":             {},
	"polkit":         {},
	"polkitd":        {},
	"shadow":         {},
	"gshadow":        {},
	"pam":            {},
	"systemd-logind": {},
	"kern":           {},
	"kernel":         {},
	"dmesg":          {},
	"btmp":           {},
	"wtmp":           {},
	"lastlog":        {},
	"faillog":        {},
	"auditd":         {},
	"audit":          {},
}

// knownHostOSIdentifiers contains normalized (lowercase) names of host OS, init, and hypervisor daemons.
var knownHostOSIdentifiers = map[string]struct{}{
	"systemd":                 {},
	"systemd-udevd":           {},
	"systemd-journald":        {},
	"systemd-networkd":        {},
	"systemd-resolved":        {},
	"systemd-timesyncd":       {},
	"journald":                {},
	"syslog":                  {},
	"rsyslog":                 {},
	"rsyslogd":                {},
	"daemon":                  {},
	"messages":                {},
	"cron":                    {},
	"crond":                   {},
	"crontab":                 {},
	"atd":                     {},
	"dbus":                    {},
	"dbus-daemon":             {},
	"dbus-broker":             {},
	"modemmanager":            {},
	"networkmanager":          {},
	"wpa_supplicant":          {},
	"multipath":               {},
	"multipathd":              {},
	"vmware-vmsvc-root":       {},
	"vmware-vmtoolsd-root":    {},
	"vmtoolsd":                {},
	"qemu-ga":                 {},
	"packagekit":              {},
	"packagekitd":             {},
	"unattended-upgrades":     {},
	"apt-daily":               {},
	"apt-daily-upgrade":       {},
	"chrony":                  {},
	"chronyd":                 {},
	"ntpd":                    {},
	"dhclient":                {},
	"dhcpcd":                  {},
	"agetty":                  {},
	"irqbalance":              {},
	"acpid":                   {},
	"udev":                    {},
	"lvm2":                    {},
	"mdadm":                   {},
	"snapd":                   {},
	"kubelet":                 {},
	"dockerd":                 {},
	"dockerd-rootless":        {},
	"dockerd-rootless.sh":     {},
	"containerd":              {},
	"containerd-shim":         {},
	"containerd-shim-runc-v2": {},
	"runc":                    {},
	"crio":                    {},
	"etcd":                    {},
	"fwupd":                   {},
	"networkd-dispatcher":     {},
	"avahi-daemon":            {},
	"accounts-daemon":         {},
	"rtkit-daemon":            {},
	"udisksd":                 {},
	"upowerd":                 {},
	"lxcfs":                   {},
	"vmware-vmsvc":            {},
	"kube-apiserver":          {},
	"kube-controller-manager": {},
	"kube-scheduler":          {},
	"kube-proxy":              {},
}

// unitSuffixes holds systemd unit type extensions.
var unitSuffixes = []string{
	".service", ".socket", ".timer", ".target", ".scope",
	".slice", ".mount", ".path", ".automount", ".swap",
}

// IsSensitiveOSLogSource returns true (DENY FOR ALL USERS) for auth, PAM, kernel, or audit logs.
func IsSensitiveOSLogSource(service, unit, facility string) bool {
	f := strings.TrimSpace(facility)
	if f == "4" || f == "10" || strings.EqualFold(f, "auth") || strings.EqualFold(f, "authpriv") {
		return true
	}

	for _, candidate := range []string{service, unit} {
		norm := strings.TrimSpace(strings.ToLower(candidate))
		if norm == "" {
			continue
		}
		if strings.HasPrefix(norm, "user@") {
			return true
		}
		if strings.HasPrefix(norm, "session-") && strings.HasSuffix(norm, ".scope") {
			return true
		}

		clean := strings.TrimSuffix(norm, ".service")
		clean = strings.TrimSuffix(clean, ".log")
		if _, ok := sensitiveOSIdentifiers[clean]; ok {
			return true
		}
		if _, ok := sensitiveOSIdentifiers[norm]; ok {
			return true
		}
	}
	return false
}

// IsJournalctlAlias returns true for "journalctl" or "systemd-journal".
func IsJournalctlAlias(service string) bool {
	s := strings.TrimSpace(strings.ToLower(service))
	return s == "journalctl" || s == "systemd-journal"
}

// IsHostSystemLog returns true when a log entry originates from the host OS, journald, or system daemons.
func IsHostSystemLog(namespace, service string) bool {
	ns := strings.TrimSpace(namespace)
	svc := strings.TrimSpace(strings.ToLower(service))

	if ns == HostNamespace || IsJournalctlAlias(svc) || IsSensitiveOSLogSource(svc, "", "") {
		return true
	}

	for _, suf := range unitSuffixes {
		if strings.HasSuffix(svc, suf) {
			return true
		}
	}

	// For traefik: only host system log when explicitly in HostNamespace or ending with .service (handled above)
	if svc == "traefik" {
		return ns == HostNamespace
	}

	base := strings.TrimSuffix(svc, ".service")
	if _, ok := knownHostOSIdentifiers[base]; ok {
		return true
	}
	if _, ok := knownHostOSIdentifiers[svc]; ok {
		return true
	}

	return false
}
