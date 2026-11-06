package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestCheckLogAccessRBAC_SensitiveOSSourcesBlockedForAll(t *testing.T) {
	roles := []string{
		tenancy.PlatformAdminRole,
		"tenant_admin",
		"operator",
		"developer",
		"viewer",
		"",
	}

	sensitiveServices := []string{
		"auth",
		"auth.log",
		"authpriv",
		"secure",
		"sshd",
		"ssh",
		"sudo",
		"su",
		"polkit",
		"polkitd",
		"shadow",
		"pam",
		"systemd-logind",
		"kern",
		"kernel",
		"dmesg",
		"audit",
		"auditd",
		"user@1000.service",
		"session-1.scope",
	}

	for _, role := range roles {
		ctx := tenancy.WithUserRole(context.Background(), role)
		for _, svc := range sensitiveServices {
			code, msg := CheckLogAccessRBAC(ctx, "", svc)
			if code != http.StatusForbidden {
				t.Errorf("role %q, service %q: expected 403 Forbidden, got %d (msg: %s)", role, svc, code, msg)
			}
		}
	}
}

func TestCheckLogAccessRBAC_HostJournalctlAccess(t *testing.T) {
	t.Run("Platform admin allowed for journalctl and aliases", func(t *testing.T) {
		ctx := tenancy.WithUserRole(context.Background(), tenancy.PlatformAdminRole)
		for _, svc := range []string{"journalctl", "systemd-journal"} {
			code, msg := CheckLogAccessRBAC(ctx, "", svc)
			if code != 0 {
				t.Errorf("service %q: expected 0 allowed, got %d (msg: %s)", svc, code, msg)
			}
		}
	})

	t.Run("Non-platform admin blocked with 403 for journalctl and aliases", func(t *testing.T) {
		for _, role := range []string{"tenant_admin", "operator", "developer", "viewer", ""} {
			ctx := tenancy.WithUserRole(context.Background(), role)
			for _, svc := range []string{"journalctl", "systemd-journal"} {
				code, msg := CheckLogAccessRBAC(ctx, "", svc)
				if code != http.StatusForbidden {
					t.Errorf("role %q, service %q: expected 403, got %d (msg: %s)", role, svc, code, msg)
				}
			}
		}
	})
}

func TestCheckLogAccessRBAC_HostNamespaceAccess(t *testing.T) {
	t.Run("Platform admin allowed for __host__ namespace", func(t *testing.T) {
		ctx := tenancy.WithUserRole(context.Background(), tenancy.PlatformAdminRole)
		code, msg := CheckLogAccessRBAC(ctx, logging.HostNamespace, "kubelet")
		if code != 0 {
			t.Errorf("expected 0 allowed, got %d (msg: %s)", code, msg)
		}
	})

	t.Run("Non-platform admin blocked with 403 for __host__ namespace", func(t *testing.T) {
		for _, role := range []string{"tenant_admin", "operator", "developer", "viewer", ""} {
			ctx := tenancy.WithUserRole(context.Background(), role)
			code, msg := CheckLogAccessRBAC(ctx, logging.HostNamespace, "my-app")
			if code != http.StatusForbidden {
				t.Errorf("role %q: expected 403, got %d (msg: %s)", role, code, msg)
			}
		}
	})
}

func TestCheckLogAccessRBAC_HostDaemonServices(t *testing.T) {
	daemons := []string{
		"kubelet",
		"kubelet.service",
		"containerd",
		"dockerd",
		"cron",
		"systemd",
		"dbus",
	}

	t.Run("Platform admin allowed for host daemons", func(t *testing.T) {
		ctx := tenancy.WithUserRole(context.Background(), tenancy.PlatformAdminRole)
		for _, svc := range daemons {
			code, msg := CheckLogAccessRBAC(ctx, "", svc)
			if code != 0 {
				t.Errorf("service %q: expected 0 allowed, got %d (msg: %s)", svc, code, msg)
			}
		}
	})

	t.Run("Non-platform admin blocked with 403 for host daemons", func(t *testing.T) {
		for _, role := range []string{"tenant_admin", "operator", "developer", "viewer", ""} {
			ctx := tenancy.WithUserRole(context.Background(), role)
			for _, svc := range daemons {
				code, msg := CheckLogAccessRBAC(ctx, "", svc)
				if code != http.StatusForbidden {
					t.Errorf("role %q, service %q: expected 403, got %d (msg: %s)", role, svc, code, msg)
				}
			}
		}
	})
}

func TestCheckLogAccessRBAC_ApplicationContainersAllowedForAll(t *testing.T) {
	apps := []string{
		"db",
		"my-nginx",
		"tiki_redis",
		"order-api",
		"cart-svc",
		"nats",
		"",
	}

	roles := []string{
		tenancy.PlatformAdminRole,
		"tenant_admin",
		"operator",
		"developer",
		"viewer",
	}

	for _, role := range roles {
		ctx := tenancy.WithUserRole(context.Background(), role)
		for _, app := range apps {
			code, msg := CheckLogAccessRBAC(ctx, "default", app)
			if code != 0 {
				t.Errorf("role %q, app %q: expected 0 allowed, got %d (msg: %s)", role, app, code, msg)
			}
		}
	}
}
