package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// AuditRecorder defines the action recording contract matching audit.Repository.
type AuditRecorder interface {
	RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error
}

var readOnlyPostActionSuffixes = []string{
	"/validate-kubeconfig",
	"/discover",
	"/cluster-search",
	"/logs/search",
	"/test",
	"/health",
	"/validate",
	"/render",
	"/fips/scan",
	"/ocsp/verify-staple",
	"/crypto/verify",
	"/kms/verify",
	"/validate-query",
	"/validate-url",
	"/audit-deps",
	"/validate-order",
	"/audit-symmetry",
	"/validate-path-mtu",
	"/resolve-digest",
	"/validate-ephemeral",
	"/pod-preflight",
	"/pfc-deadlock-check",
	"/verify-regex",
	"/validate-clock-skew",
	"/kv-cache-check",
	"/nccl-progress",
	"/validate-fencing",
	"/check-health-fallback",
}

// IsReadOnlyPostPath determines if a request is an idempotent query, validation, or check.
// It returns true ONLY when method is POST and path ends with one of the explicit read-only action suffixes.
func IsReadOnlyPostPath(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	cleanPath := strings.ToLower(strings.TrimRight(path, "/"))
	for _, suffix := range readOnlyPostActionSuffixes {
		if strings.HasSuffix(cleanPath, suffix) {
			return true
		}
	}
	return false
}

// MutationAudit records state-modifying requests into the central audit repository.
// Idempotent queries, telemetry streaming, log ingestion, and requests already audited are skipped.
func MutationAudit(recorder AuditRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if recorder == nil ||
				r.Method == http.MethodGet ||
				r.Method == http.MethodHead ||
				r.Method == http.MethodOptions ||
				IsStreamingRequest(r) ||
				strings.HasPrefix(r.URL.Path, "/api/v1/logs") {
				next.ServeHTTP(w, r)
				return
			}

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)

			if tenancy.IsAuditRecorded(r.Context()) {
				return
			}

			if r.Method == http.MethodPost && IsReadOnlyPostPath(r.Method, r.URL.Path) {
				isParamSuffix := false
				if rctx := chi.RouteContext(r.Context()); rctx != nil {
					rp := strings.TrimRight(rctx.RoutePattern(), "/")
					if strings.HasSuffix(rp, "}") {
						isParamSuffix = true
					}
				}
				if !isParamSuffix {
					return
				}
			}

			actor := "anonymous"
			if meta := tenancy.RequestMetaFromContext(r.Context()); meta != nil {
				if uid := meta.UserID(); uid != "" {
					actor = uid
				}
			}

			result := "success"
			if rw.statusCode == http.StatusUnauthorized || rw.statusCode == http.StatusForbidden {
				result = "denied"
			} else if rw.statusCode >= http.StatusBadRequest {
				result = "failure"
			}

			details := map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"status": rw.statusCode,
			}

			auditCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
			defer cancel()

			if err := recorder.RecordAction(
				auditCtx,
				actor,
				r.Method,
				"http_endpoint",
				"",
				r.URL.Path,
				result,
				details,
				r.RemoteAddr,
				r.UserAgent(),
			); err != nil {
				zap.L().Warn("failed to record mutation audit log", zap.Error(err), zap.String("path", r.URL.Path))
			}
		})
	}
}
