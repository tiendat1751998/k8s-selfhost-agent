package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestMapDomainErrorToHTTP(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode int
		expectedMsg  string
	}{
		{
			name:         "NotFound error",
			err:          domainerrors.NewNotFound("cluster", "c-123"),
			expectedCode: http.StatusNotFound,
			expectedMsg:  "cluster with id 'c-123' not found",
		},
		{
			name:         "Validation error",
			err:          domainerrors.NewValidation("email", "invalid format"),
			expectedCode: http.StatusBadRequest,
			expectedMsg:  "validation failed for field 'email': invalid format",
		},
		{
			name:         "Conflict error",
			err:          domainerrors.NewConflict("cluster", "already exists"),
			expectedCode: http.StatusConflict,
			expectedMsg:  "conflict on cluster: already exists",
		},
		{
			name:         "Unauthorized error",
			err:          domainerrors.NewUnauthorized("invalid credentials"),
			expectedCode: http.StatusUnauthorized,
			expectedMsg:  "invalid credentials",
		},
		{
			name:         "Forbidden error",
			err:          domainerrors.NewForbidden("access denied"),
			expectedCode: http.StatusForbidden,
			expectedMsg:  "access denied",
		},
		{
			name:         "Timeout error",
			err:          domainerrors.NewTimeout("database query"),
			expectedCode: http.StatusGatewayTimeout,
			expectedMsg:  "operation 'database query' timed out",
		},
		{
			name:         "Internal domain error",
			err:          domainerrors.NewInternal("database connection died", errors.New("connection reset")),
			expectedCode: http.StatusInternalServerError,
			expectedMsg:  "Internal server error",
		},
		{
			name:         "Generic standard error",
			err:          errors.New("raw postgres sql syntax error: table not found"),
			expectedCode: http.StatusInternalServerError,
			expectedMsg:  "Internal server error",
		},
		{
			name:         "K8sUnavailable domain error",
			err:          domainerrors.NewK8sUnavailable("", nil),
			expectedCode: http.StatusServiceUnavailable,
			expectedMsg:  "Kubernetes cluster not connected or unconfigured",
		},
		{
			name:         "K8sUnavailable sentinel error",
			err:          domainerrors.ErrK8sUnavailable,
			expectedCode: http.StatusServiceUnavailable,
			expectedMsg:  "Kubernetes cluster not connected or unconfigured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg := mapDomainErrorToHTTP(tt.err)
			if code != tt.expectedCode {
				t.Errorf("expected status code %d, got %d", tt.expectedCode, code)
			}
			if msg != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, msg)
			}
		})
	}
}

func TestWriteError_NoLeak(t *testing.T) {
	w := httptest.NewRecorder()
	rawErr := errors.New("sensitive postgres sql syntax error near SELECT * FROM users WHERE password_hash = ...")
	writeError(w, http.StatusInternalServerError, "failed to query user", rawErr)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	body := w.Body.String()
	// Ensure raw error is NEVER leaked in the response body
	if domainerrors.Wrap(rawErr, "").Error() != "" && (w.Body.String() == "" || errors.Is(rawErr, rawErr)) {
		if string(body) != "{\"error\":\"failed to query user\"}\n" {
			t.Errorf("unexpected response body: %s", body)
		}
	}
}

func TestWriteError_DomainErrorAutoMapping(t *testing.T) {
	w := httptest.NewRecorder()
	domErr := domainerrors.NewNotFound("cluster", "c-999")
	writeError(w, http.StatusInternalServerError, "", domErr)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 from domain error mapping, got %d", w.Code)
	}
	expected := "{\"error\":\"cluster with id 'c-999' not found\"}\n"
	if w.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, w.Body.String())
	}
}
