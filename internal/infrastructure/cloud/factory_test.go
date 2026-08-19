package cloud_test

import (
	"context"
	"errors"
	"testing"

	domain "github.com/datdt/k8sselfhost/internal/domain/cloud"
	infra "github.com/datdt/k8sselfhost/internal/infrastructure/cloud"
)

func TestFactory_AWS_ReturnsProvider(t *testing.T) {
	factory := infra.NewProviderFactory()
	provider, err := factory.CreateProvider(domain.ProviderAWS, "", "us-east-1")
	if err != nil {
		t.Fatalf("expected no error for AWS provider, got: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider for AWS")
	}

	_, ok := provider.(domain.Provider)
	if !ok {
		t.Fatalf("expected provider to implement domain.Provider")
	}
}

func TestFactory_GCP_ReturnsProvider(t *testing.T) {
	factory := infra.NewProviderFactory()
	provider, err := factory.CreateProvider(domain.ProviderGCP, "", "us-central1")
	if err != nil {
		t.Fatalf("expected no error for GCP provider, got: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider for GCP")
	}

	_, ok := provider.(domain.Provider)
	if !ok {
		t.Fatalf("expected provider to implement domain.Provider")
	}
}

func TestFactory_Azure_ReturnsProvider(t *testing.T) {
	factory := infra.NewProviderFactory()
	provider, err := factory.CreateProvider(domain.ProviderAzure, "", "eastus")
	if err != nil {
		t.Fatalf("expected no error for Azure provider, got: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider for Azure")
	}

	_, ok := provider.(domain.Provider)
	if !ok {
		t.Fatalf("expected provider to implement domain.Provider")
	}
}

func TestFactory_Unknown_ReturnsError(t *testing.T) {
	factory := infra.NewProviderFactory()
	provider, err := factory.CreateProvider(domain.ProviderType("digitalocean"), "", "nyc1")
	if err == nil {
		t.Fatal("expected error for unknown provider type, got nil")
	}
	if provider != nil {
		t.Fatalf("expected nil provider for unknown provider type, got: %v", provider)
	}
}

func TestFactory_GetProvider(t *testing.T) {
	factory := infra.NewProviderFactory()
	provider, err := factory.GetProvider(domain.ProviderAWS)
	if err != nil {
		t.Fatalf("expected no error for GetProvider AWS, got: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider for GetProvider AWS")
	}
}

func TestPlaceholder_ValidateCredentials_ReturnsNotConfigured(t *testing.T) {
	provider := infra.NewPlaceholderProvider("AWS EKS")
	account := &domain.CloudAccount{
		ID:       "acc-123",
		Provider: domain.ProviderAWS,
		Region:   "us-east-1",
	}

	err := provider.ValidateCredentials(context.Background(), account)
	if err == nil {
		t.Fatal("expected error from ValidateCredentials, got nil")
	}
	if !errors.Is(err, infra.ErrProviderNotConfigured) {
		t.Fatalf("expected errors.Is(err, ErrProviderNotConfigured) to be true, got: %v", err)
	}
}

func TestPlaceholder_ListClusters_ReturnsNotConfigured(t *testing.T) {
	provider := infra.NewPlaceholderProvider("AWS EKS")
	account := &domain.CloudAccount{
		ID:       "acc-123",
		Provider: domain.ProviderAWS,
		Region:   "us-east-1",
	}

	clusters, err := provider.ListClusters(context.Background(), account)
	if err == nil {
		t.Fatal("expected error from ListClusters, got nil")
	}
	if clusters != nil {
		t.Fatalf("expected nil clusters, got: %v", clusters)
	}
	if !errors.Is(err, infra.ErrProviderNotConfigured) {
		t.Fatalf("expected errors.Is(err, ErrProviderNotConfigured) to be true, got: %v", err)
	}
}

func TestPlaceholder_AllMethods_ImplementInterface(t *testing.T) {
	var _ domain.Provider = (*infra.PlaceholderProvider)(nil)

	provider := infra.NewPlaceholderProvider("GCP GKE")
	account := &domain.CloudAccount{
		ID:       "acc-456",
		Provider: domain.ProviderGCP,
		Region:   "us-central1",
	}
	ctx := context.Background()

	if _, err := provider.GetKubeconfig(ctx, account, "test-cluster"); !errors.Is(err, infra.ErrProviderNotConfigured) {
		t.Fatalf("GetKubeconfig: expected ErrProviderNotConfigured, got %v", err)
	}

	if _, err := provider.ListNodeGroups(ctx, account, "test-cluster"); !errors.Is(err, infra.ErrProviderNotConfigured) {
		t.Fatalf("ListNodeGroups: expected ErrProviderNotConfigured, got %v", err)
	}

	if err := provider.ScaleNodeGroup(ctx, account, "test-cluster", "default-pool", 3); !errors.Is(err, infra.ErrProviderNotConfigured) {
		t.Fatalf("ScaleNodeGroup: expected ErrProviderNotConfigured, got %v", err)
	}
}
