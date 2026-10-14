// Package middleware provides HTTP middleware for logging, metrics, and security.
package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

var (
	tracer          = otel.Tracer("k8sselfhost/http")
	meter           = otel.Meter("k8sselfhost/http")
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
	requestCounter  int64
)

// RequestCounter counts total incoming HTTP requests in-memory.
func RequestCounter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCounter, 1)
		next.ServeHTTP(w, r)
	})
}

// GetRequestCount returns the total number of HTTP requests recorded.
func GetRequestCount() int64 {
	return atomic.LoadInt64(&requestCounter)
}

func init() {
	var err error
	requestCount, err = meter.Int64Counter("http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		logger.Get().Fatal("failed to create http_requests_total counter", zap.Error(err))
	}

	requestDuration, err = meter.Float64Histogram("http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		logger.Get().Fatal("failed to create http_request_duration_seconds histogram", zap.Error(err))
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hijacker.Hijack()
}

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// RequestBodyLimit limits the size of incoming HTTP request bodies.
func RequestBodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxBytes <= (1<<20) && strings.HasPrefix(r.URL.Path, "/api/v1/logs") {
				next.ServeHTTP(w, r)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// Metrics records request count and duration as Prometheus metrics.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		attrs := []attribute.KeyValue{
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
			attribute.Int("status", rw.statusCode),
		}

		requestCount.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		requestDuration.Record(r.Context(), duration, metric.WithAttributes(attrs...))
	})
}

var sensitiveQueryKeys = map[string]struct{}{
	"token":         {},
	"trace_token":   {},
	"authorization": {},
	"key":           {},
	"password":      {},
	"secret":        {},
	"api_key":       {},
	"apikey":        {},
	"access_token":  {},
	"refresh_token": {},
	"auth_token":    {},
	"private_key":   {},
}

func isSensitiveQueryKey(key string) bool {
	clean := strings.ToLower(strings.TrimSpace(key))
	clean = strings.ReplaceAll(clean, "-", "_")
	_, exists := sensitiveQueryKeys[clean]
	return exists
}

// SanitizeURL returns a string representation of the URL with sensitive query parameters redacted.
// It does not modify the input URL.
func SanitizeURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	if u.RawQuery == "" {
		return u.String()
	}

	uCopy := *u
	query := uCopy.Query()
	modified := false

	for k := range query {
		if isSensitiveQueryKey(k) {
			query.Set(k, "[REDACTED]")
			modified = true
		}
	}

	if modified {
		uCopy.RawQuery = query.Encode()
	}
	return uCopy.String()
}

// Tracing adds OpenTelemetry tracing spans to HTTP requests.
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", SanitizeURL(r.URL)),
			attribute.String("http.user_agent", r.UserAgent()),
		)

		rw := newResponseWriter(w)
		next.ServeHTTP(rw, r.WithContext(ctx))

		span.SetAttributes(attribute.Int("http.status_code", rw.statusCode))
	})
}

// StructuredLogger logs HTTP requests using zap structured logging.
func StructuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		logger.Get().Info("http request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.statusCode),
			zap.Duration("duration", time.Since(start)),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)
	})
}

var (
	corsCacheMu   sync.RWMutex
	corsCachedEnv string
	corsCachedMap map[string]struct{}
)

func getCachedAllowedOrigins(env string) map[string]struct{} {
	corsCacheMu.RLock()
	if env == corsCachedEnv && corsCachedMap != nil {
		m := corsCachedMap
		corsCacheMu.RUnlock()
		return m
	}
	corsCacheMu.RUnlock()

	corsCacheMu.Lock()
	defer corsCacheMu.Unlock()
	if env == corsCachedEnv && corsCachedMap != nil {
		return corsCachedMap
	}

	newMap := make(map[string]struct{})
	for _, o := range strings.Split(env, ",") {
		o = strings.ToLower(strings.TrimRight(strings.TrimSpace(o), "/"))
		if o == "" || o == "*" {
			continue
		}
		newMap[o] = struct{}{}
	}
	corsCachedEnv = env
	corsCachedMap = newMap
	return newMap
}

// IsOriginAllowed checks whether the specified origin is permitted by CORS_ALLOWED_ORIGINS.
// By default, no cross-origin requests are allowed unless explicitly configured in CORS_ALLOWED_ORIGINS.
// Wildcards ('*') are rejected to prevent unauthenticated/credentialed reflection attacks.
func IsOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	allowedOriginsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
	if strings.TrimSpace(allowedOriginsEnv) == "" {
		return false
	}

	normOrigin := strings.ToLower(strings.TrimRight(strings.TrimSpace(origin), "/"))
	allowedMap := getCachedAllowedOrigins(allowedOriginsEnv)
	_, ok := allowedMap[normOrigin]
	return ok
}

// CORS adds Cross-Origin Resource Sharing headers.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" && IsOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept")
		}

		if r.Method == http.MethodOptions {
			if origin != "" && !IsOriginAllowed(origin) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders adds modern OWASP security headers to responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self' ws: wss:; img-src 'self' data:; frame-ancestors 'none'; object-src 'none'; base-uri 'self';")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		next.ServeHTTP(w, r)
	})
}
