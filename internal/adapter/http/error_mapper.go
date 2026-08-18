package http

import (
	"errors"
	"net/http"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// mapDomainErrorToHTTP converts domain errors to appropriate HTTP status codes.
// Production environments should NEVER leak raw internal error details.
func mapDomainErrorToHTTP(err error) (int, string) {
	var domErr *domainerrors.DomainError
	if errors.As(err, &domErr) {
		switch domErr.Code {
		case domainerrors.CodeNotFound:
			return http.StatusNotFound, domErr.Message
		case domainerrors.CodeValidation:
			return http.StatusBadRequest, domErr.Message
		case domainerrors.CodeConflict:
			return http.StatusConflict, domErr.Message
		case domainerrors.CodeUnauthorized:
			return http.StatusUnauthorized, domErr.Message
		case domainerrors.CodeForbidden:
			return http.StatusForbidden, domErr.Message
		case domainerrors.CodeTimeout:
			return http.StatusGatewayTimeout, domErr.Message
		case domainerrors.CodeBinaryNotFound:
			return http.StatusNotFound, domErr.Message
		case domainerrors.CodeInternal:
			return http.StatusInternalServerError, "Internal server error"
		}
	}
	return http.StatusInternalServerError, "Internal server error"
}
