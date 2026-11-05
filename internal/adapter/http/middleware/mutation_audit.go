package middleware

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// AuditRecorder defines the action recording contract matching audit.Repository.
type AuditRecorder interface {
	RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error
}

var readOnlyPostKeywords = []string{
	"/validate",
	"/test",
	"/verify",
	"/discover",
	"/render",
	"/cluster-search",
	"/search",
	"/check-",
	"/audit-",
	"/pod-preflight",
	"/pfc-deadlock-check",
	"/nccl-progress",
	"/kv-cache-check",
	"/resolve-digest",
	"/fips/scan",
}

// IsReadOnlyPostPath determines if a POST request path is an idempotent query, validation, or check.
func IsReadOnlyPostPath(path string) bool {
	lower := strings.ToLower(path)
	for _, kw := range readOnlyPostKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

type mutationResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *mutationResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *mutationResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *mutationResponseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *mutationResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hijacker.Hijack()
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
				strings.HasPrefix(r.URL.Path, "/api/v1/logs") ||
				IsReadOnlyPostPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			rw := &mutationResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			if tenancy.IsAuditRecorded(r.Context()) {
				return
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
