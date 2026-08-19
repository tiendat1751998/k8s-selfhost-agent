package cloud

import (
	"fmt"

	domain "github.com/datdt/k8sselfhost/internal/domain/cloud"
)

// ProviderFactory creates cloud.Provider instances from account credentials.
type ProviderFactory struct{}

// NewProviderFactory creates a new ProviderFactory.
func NewProviderFactory() *ProviderFactory { return &ProviderFactory{} }

// CreateProvider returns a Provider for the given account type and decrypted credentials.
func (f *ProviderFactory) CreateProvider(providerType domain.ProviderType, decryptedCreds, region string) (domain.Provider, error) {
	switch providerType {
	case domain.ProviderAWS:
		return NewPlaceholderProvider("AWS EKS"), nil
	case domain.ProviderGCP:
		return NewPlaceholderProvider("GCP GKE"), nil
	case domain.ProviderAzure:
		return NewPlaceholderProvider("Azure AKS"), nil
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", providerType)
	}
}

// GetProvider returns a Provider for the given provider type (implements adapthttp.CloudProviderFactory).
func (f *ProviderFactory) GetProvider(providerType domain.ProviderType) (domain.Provider, error) {
	return f.CreateProvider(providerType, "", "")
}
