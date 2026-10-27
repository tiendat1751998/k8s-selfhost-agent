package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// matchAgentAuthToken checks if an incoming request to /agents/register has a valid AGENT_AUTH_TOKEN.
// Accepts "Authorization: Bearer <token>" or "X-Agent-Token: <token>".
func matchAgentAuthToken(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	path := r.URL.Path
	if !strings.HasSuffix(path, "/agents/register") && path != "/agents/register" {
		return false
	}

	expectedToken := os.Getenv("AGENT_AUTH_TOKEN")
	if expectedToken == "" {
		return false
	}

	var callerToken string
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		prefix, rest, found := strings.Cut(authHeader, " ")
		if found && strings.EqualFold(prefix, "Bearer") {
			callerToken = rest
		}
	}
	if callerToken == "" {
		callerToken = r.Header.Get("X-Agent-Token")
	}

	if callerToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(callerToken), []byte(expectedToken)) == 1
}
