package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// MetricsAuthMiddleware enforces authentication for the /metrics endpoint.
// It requires an Authorization: Bearer <token> header matching METRICS_TOKEN
// (if METRICS_TOKEN is set) or a valid JWT bearer token.
// If METRICS_TOKEN is unset and no valid JWT is provided, requests return 401 Unauthorized.
func MetricsAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		var token string
		if authHeader != "" {
			prefix, rest, found := strings.Cut(authHeader, " ")
			if found && prefix == "Bearer" {
				token = rest
			}
		}

		if token == "" {
			http.Error(w, "unauthorized: missing or invalid authorization header", http.StatusUnauthorized)
			return
		}

		expectedToken := os.Getenv("METRICS_TOKEN")
		if expectedToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		// Also permit valid JWT tokens
		if _, err := ValidateJWT(token); err == nil {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "unauthorized: invalid metrics token", http.StatusUnauthorized)
	})
}
