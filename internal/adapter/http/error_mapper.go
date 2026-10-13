package http

import (
	"errors"
	"net/http"

	domaincrypto "github.com/datdt/k8sselfhost/internal/domain/crypto"
	"github.com/datdt/k8sselfhost/internal/domain/k8sresilience"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// mapDomainErrorToHTTP converts domain errors to appropriate HTTP status codes.
// Production environments should NEVER leak raw internal error details.
func mapDomainErrorToHTTP(err error) (int, string) {
	if errors.Is(err, infraK8s.ErrK8sUnavailable) || errors.Is(err, domainerrors.ErrK8sUnavailable) {
		return http.StatusServiceUnavailable, "Kubernetes cluster not connected or unconfigured"
	}
	if errors.Is(err, k8sresilience.ErrK8sWebhookDeadlock) {
		return http.StatusInternalServerError, err.Error()
	}
	if errors.Is(err, k8sresilience.ErrK8sPLEGTimeout) {
		return http.StatusGatewayTimeout, err.Error()
	}
	if errors.Is(err, k8sresilience.ErrFinalizerReapFailed) {
		return http.StatusInternalServerError, err.Error()
	}
	if errors.Is(err, k8sresilience.ErrWebhookNotFound) || errors.Is(err, k8sresilience.ErrNodeNotFound) {
		return http.StatusNotFound, err.Error()
	}
	if errors.Is(err, domaincrypto.ErrOCSPStapleFetchFailed) {
		return http.StatusBadGateway, err.Error()
	}
	if errors.Is(err, domaincrypto.ErrHSMPoolSaturated) {
		return http.StatusServiceUnavailable, err.Error()
	}
	if errors.Is(err, domaincrypto.ErrFIPSNonCompliantHash) {
		return http.StatusUnprocessableEntity, err.Error()
	}
	if errors.Is(err, domaincrypto.ErrKMSDEKGraceExpired) {
		return http.StatusInternalServerError, err.Error()
	}
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
		case domainerrors.CodeK8sUnavailable:
			return http.StatusServiceUnavailable, domErr.Message
		case "ERR_STORAGE_SHRINK_UNSUPPORTED":
			return http.StatusBadRequest, domErr.Message
		case "ERR_STORAGE_CLASS_NO_EXPANSION":
			return http.StatusUnprocessableEntity, domErr.Message
		case "ERR_VOLUME_ATTACHMENT_LOCKED":
			return http.StatusConflict, domErr.Message
		case "ERR_DIND_NOT_ALLOWED":
			return http.StatusForbidden, domErr.Message
		case "ERR_ADMISSION_MUTATION_FAILED":
			return http.StatusUnprocessableEntity, domErr.Message
		case domainerrors.CodeInternal:
			return http.StatusInternalServerError, "Internal server error"
		}
	}
	return http.StatusInternalServerError, "Internal server error"
}
