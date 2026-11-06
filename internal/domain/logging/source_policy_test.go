package logging

import "testing"

func TestIsSensitiveOSLogSource(t *testing.T) {
	tests := []struct {
		service  string
		unit     string
		facility string
		expected bool
	}{
		// Syslog facilities (4, 10, auth, authpriv)
		{"", "", "4", true},
		{"", "", "10", true},
		{"", "", "auth", true},
		{"", "", "AUTH", true},
		{"", "", "authpriv", true},
		{"", "", "AUTHPRIV", true},
		{"", "", "1", false},
		{"", "", "daemon", false},

		// Sensitive services/units/identifiers (case-insensitive, with/without .service / .log)
		{"auth", "", "", true},
		{"AUTH", "", "", true},
		{"auth.log", "", "", true},
		{"auth.service", "", "", true},
		{"authpriv", "", "", true},
		{"secure", "", "", true},
		{"secure.log", "", "", true},
		{"sshd", "", "", true},
		{"sshd.service", "", "", true},
		{"ssh", "", "", true},
		{"sudo", "", "", true},
		{"sudo.service", "", "", true},
		{"su", "", "", true},
		{"polkit", "", "", true},
		{"polkitd", "", "", true},
		{"shadow", "", "", true},
		{"gshadow", "", "", true},
		{"pam", "", "", true},
		{"systemd-logind", "", "", true},
		{"systemd-logind.service", "", "", true},
		{"kern", "", "", true},
		{"kern.log", "", "", true},
		{"kernel", "", "", true},
		{"dmesg", "", "", true},
		{"btmp", "", "", true},
		{"wtmp", "", "", true},
		{"lastlog", "", "", true},
		{"faillog", "", "", true},
		{"auditd", "", "", true},
		{"audit", "", "", true},
		{"audit.log", "", "", true},

		// user@* and session-*.scope
		{"user@1000.service", "", "", true},
		{"USER@0.SERVICE", "", "", true},
		{"session-1.scope", "", "", true},
		{"session-c2.scope", "", "", true},

		// Check unit parameter
		{"", "sshd.service", "", true},
		{"", "sudo", "", true},
		{"", "user@1000.service", "", true},

		// Normal application services
		{"my-nginx", "", "", false},
		{"db", "", "", false},
		{"tiki_redis", "", "", false},
		{"nats", "", "", false},
		{"kubelet", "", "", false}, // kubelet is host OS log, but NOT sensitive OS auth source
		{"cron", "", "", false},    // cron is host OS log, but NOT sensitive OS auth source
	}

	for _, tc := range tests {
		t.Run(tc.service+"/"+tc.unit+"/"+tc.facility, func(t *testing.T) {
			got := IsSensitiveOSLogSource(tc.service, tc.unit, tc.facility)
			if got != tc.expected {
				t.Fatalf("IsSensitiveOSLogSource(%q, %q, %q) = %v; want %v", tc.service, tc.unit, tc.facility, got, tc.expected)
			}
		})
	}
}

func TestIsJournalctlAlias(t *testing.T) {
	if !IsJournalctlAlias("journalctl") {
		t.Errorf("expected journalctl to be alias")
	}
	if !IsJournalctlAlias("JOURNALCTL") {
		t.Errorf("expected JOURNALCTL to be alias")
	}
	if !IsJournalctlAlias("systemd-journal") {
		t.Errorf("expected systemd-journal to be alias")
	}
	if !IsJournalctlAlias("  journalctl  ") {
		t.Errorf("expected whitespace-trimmed journalctl to be alias")
	}
	if IsJournalctlAlias("kubelet") {
		t.Errorf("expected kubelet NOT to be alias")
	}
	if IsJournalctlAlias("") {
		t.Errorf("expected empty string NOT to be alias")
	}
}

func TestIsHostSystemLog(t *testing.T) {
	tests := []struct {
		namespace string
		service   string
		expected  bool
	}{
		// __host__ namespace always host log
		{"__host__", "my-app", true},
		{"__host__", "anything", true},

		// journalctl aliases
		{"", "journalctl", true},
		{"default", "journalctl", true},
		{"", "systemd-journal", true},

		// Sensitive OS log sources are also host logs
		{"", "auth", true},
		{"", "sshd.service", true},

		// Unit suffixes (.service, .socket, .timer, .target, .scope, .slice, .mount, .path, .automount, .swap)
		{"", "kubelet.service", true},
		{"", "docker.socket", true},
		{"", "logrotate.timer", true},
		{"", "multi-user.target", true},
		{"", "init.scope", true},
		{"", "system.slice", true},
		{"", "var-log.mount", true},
		{"", "systemd-ask-password-wall.path", true},
		{"", "proc-sys-fs-binfmt_misc.automount", true},
		{"", "dev-swap.swap", true},

		// Known host OS daemons without suffix
		{"", "systemd", true},
		{"", "systemd-udevd", true},
		{"", "systemd-journald", true},
		{"", "syslog", true},
		{"", "rsyslog", true},
		{"", "rsyslogd", true},
		{"", "cron", true},
		{"", "crond", true},
		{"", "dbus", true},
		{"", "NetworkManager", true},
		{"", "multipathd", true},
		{"", "kubelet", true},
		{"", "containerd", true},
		{"", "dockerd", true},
		{"", "etcd", true},
		{"", "kube-apiserver", true},
		{"", "kube-proxy", true},

		// traefik handling: systemd unit vs container workload
		{"__host__", "traefik", true},
		{"", "traefik.service", true},
		{"default", "traefik", false},
		{"", "traefik", false},

		// Real Docker / Pod container applications
		{"", "db", false},
		{"", "my-nginx", false},
		{"default", "my-nginx", false},
		{"prod", "tiki_redis", false},
		{"", "nats", false},
	}

	for _, tc := range tests {
		t.Run(tc.namespace+"/"+tc.service, func(t *testing.T) {
			got := IsHostSystemLog(tc.namespace, tc.service)
			if got != tc.expected {
				t.Fatalf("IsHostSystemLog(%q, %q) = %v; want %v", tc.namespace, tc.service, got, tc.expected)
			}
		})
	}
}
