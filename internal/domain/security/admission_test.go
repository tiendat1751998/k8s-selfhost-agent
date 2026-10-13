package security

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestDomainSecurity_ErrorsAndTypes(t *testing.T) {
	assert.Equal(t, "ERR_DIND_NOT_ALLOWED", CodeDinDNotAllowed)
	assert.Equal(t, "ERR_ADMISSION_MUTATION_FAILED", CodeAdmissionPolicyViolation)

	var domErr *domainerrors.DomainError
	assert.True(t, errors.As(ErrDinDNotAllowed, &domErr))
	assert.Equal(t, CodeDinDNotAllowed, domErr.Code)

	assert.True(t, errors.As(ErrAdmissionPolicyViolation, &domErr))
	assert.Equal(t, CodeAdmissionPolicyViolation, domErr.Code)

	spec := RootlessBuildSpec{
		BuildID:     "build-123",
		ContextURL:  "git://github.com/org/repo.git",
		Dockerfile:  "Dockerfile",
		Destination: "harbor.internal/app:v1.0.0",
	}
	assert.Equal(t, "build-123", spec.BuildID)
}
