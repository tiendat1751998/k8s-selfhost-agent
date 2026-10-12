package security

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// CosignVerifier verifies cryptographic container image signatures via Cosign.
type CosignVerifier struct {
	binaryPath string
}

// NewCosignVerifier constructs a CosignVerifier with environment or default binary.
func NewCosignVerifier() *CosignVerifier {
	bin := os.Getenv("COSIGN_BINARY")
	if bin == "" {
		bin = "cosign"
	}
	return &CosignVerifier{binaryPath: bin}
}

// NewCosignVerifierWithBinary constructs a CosignVerifier with an explicit binary path.
func NewCosignVerifierWithBinary(binary string) *CosignVerifier {
	if binary == "" {
		binary = "cosign"
	}
	return &CosignVerifier{binaryPath: binary}
}

// VerifyImage validates that the given image is signed with the public key.
// It executes: cosign verify --key <publicKeyPath> <image>
// Under the Fail-Close invariant, any execution failure or verification failure returns false and an error.
func (v *CosignVerifier) VerifyImage(ctx context.Context, image string, publicKeyPath string) (bool, error) {
	if image == "" {
		return false, errors.NewValidation("image", "image reference cannot be empty")
	}
	if publicKeyPath == "" {
		return false, errors.NewValidation("publicKeyPath", "public key path cannot be empty")
	}

	cmd := exec.CommandContext(ctx, v.binaryPath, "verify", "--key", publicKeyPath, image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))
		return false, fmt.Errorf("cosign verification failed for image %s: %w (output: %s)", image, err, outStr)
	}

	return true, nil
}
