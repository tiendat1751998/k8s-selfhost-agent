package cloud_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/cloud"
)

func TestProviderTypeConstants(t *testing.T) {
	tests := []struct {
		provider cloud.ProviderType
		expected string
	}{
		{cloud.ProviderAWS, "aws"},
		{cloud.ProviderGCP, "gcp"},
		{cloud.ProviderAzure, "azure"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("expected ProviderType %q, got %q", tt.expected, string(tt.provider))
			}
		})
	}
}

func TestCloudAccount_JSONExcludesEncryptedCreds(t *testing.T) {
	now := time.Now().UTC()
	account := cloud.CloudAccount{
		ID:             "acc-test-123",
		Name:           "Production AWS",
		Provider:       cloud.ProviderAWS,
		EncryptedCreds: "super-secret-encrypted-credentials-blob-12345",
		Region:         "us-east-1",
		Status:         "active",
		TenantID:       "tenant-abc",
		LastSyncAt:     &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	bytes, err := json.Marshal(account)
	if err != nil {
		t.Fatalf("failed to marshal CloudAccount: %v", err)
	}

	jsonStr := string(bytes)

	// EncryptedCreds must NOT be present in JSON
	if strings.Contains(jsonStr, "super-secret-encrypted-credentials-blob-12345") {
		t.Errorf("sensitive EncryptedCreds was serialized into JSON: %s", jsonStr)
	}
	if strings.Contains(jsonStr, "EncryptedCreds") || strings.Contains(jsonStr, "encrypted_creds") {
		t.Errorf("EncryptedCreds key found in JSON: %s", jsonStr)
	}

	// Required public fields must be present
	var unmarshaled map[string]interface{}
	if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal CloudAccount JSON: %v", err)
	}

	if unmarshaled["id"] != "acc-test-123" {
		t.Errorf("expected id 'acc-test-123', got %v", unmarshaled["id"])
	}
	if unmarshaled["name"] != "Production AWS" {
		t.Errorf("expected name 'Production AWS', got %v", unmarshaled["name"])
	}
	if unmarshaled["provider"] != "aws" {
		t.Errorf("expected provider 'aws', got %v", unmarshaled["provider"])
	}
	if unmarshaled["region"] != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got %v", unmarshaled["region"])
	}
	if unmarshaled["status"] != "active" {
		t.Errorf("expected status 'active', got %v", unmarshaled["status"])
	}
	if unmarshaled["tenant_id"] != "tenant-abc" {
		t.Errorf("expected tenant_id 'tenant-abc', got %v", unmarshaled["tenant_id"])
	}
}

func TestCloudCluster_JSONSerialization(t *testing.T) {
	now := time.Now().UTC()
	cluster := cloud.CloudCluster{
		ID:       "cls-eks-01",
		Name:     "prod-cluster",
		Provider: cloud.ProviderAWS,
		Region:   "us-west-2",
		Status:   "ACTIVE",
		Version:  "1.28",
		Endpoint: "https://123456789.gr7.us-west-2.eks.amazonaws.com",
		NodeGroups: []cloud.CloudNodeGroup{
			{
				ID:           "ng-01",
				Name:         "worker-nodes",
				ClusterName:  "prod-cluster",
				Status:       "ACTIVE",
				InstanceType: "m5.large",
				MinSize:      2,
				MaxSize:      10,
				DesiredSize:  3,
				CurrentSize:  3,
				Labels: map[string]string{
					"environment": "production",
				},
			},
		},
		Tags: map[string]string{
			"Owner": "DevOps",
		},
		CreatedAt: &now,
	}

	bytes, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("failed to marshal CloudCluster: %v", err)
	}

	var parsed cloud.CloudCluster
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal CloudCluster: %v", err)
	}

	if parsed.ID != "cls-eks-01" {
		t.Errorf("expected cluster ID 'cls-eks-01', got %s", parsed.ID)
	}
	if parsed.Name != "prod-cluster" {
		t.Errorf("expected cluster Name 'prod-cluster', got %s", parsed.Name)
	}
	if parsed.Provider != cloud.ProviderAWS {
		t.Errorf("expected cluster Provider 'aws', got %s", parsed.Provider)
	}
	if parsed.Region != "us-west-2" {
		t.Errorf("expected cluster Region 'us-west-2', got %s", parsed.Region)
	}
	if parsed.Status != "ACTIVE" {
		t.Errorf("expected cluster Status 'ACTIVE', got %s", parsed.Status)
	}
	if parsed.Version != "1.28" {
		t.Errorf("expected cluster Version '1.28', got %s", parsed.Version)
	}
	if parsed.Endpoint != "https://123456789.gr7.us-west-2.eks.amazonaws.com" {
		t.Errorf("expected cluster Endpoint, got %s", parsed.Endpoint)
	}
	if len(parsed.NodeGroups) != 1 {
		t.Fatalf("expected 1 node group, got %d", len(parsed.NodeGroups))
	}
	ng := parsed.NodeGroups[0]
	if ng.Name != "worker-nodes" || ng.DesiredSize != 3 || ng.MinSize != 2 || ng.MaxSize != 10 {
		t.Errorf("node group fields mismatch: %+v", ng)
	}
	if parsed.Tags["Owner"] != "DevOps" {
		t.Errorf("expected tag Owner: DevOps, got %v", parsed.Tags["Owner"])
	}
}

func TestNewCloudAccount(t *testing.T) {
	account := cloud.NewCloudAccount("GCP Test", cloud.ProviderGCP, "enc-creds", "us-central1", "tenant-1")

	if account.Name != "GCP Test" {
		t.Errorf("expected Name 'GCP Test', got %s", account.Name)
	}
	if account.Provider != cloud.ProviderGCP {
		t.Errorf("expected Provider 'gcp', got %s", account.Provider)
	}
	if account.EncryptedCreds != "enc-creds" {
		t.Errorf("expected EncryptedCreds 'enc-creds', got %s", account.EncryptedCreds)
	}
	if account.Region != "us-central1" {
		t.Errorf("expected Region 'us-central1', got %s", account.Region)
	}
	if account.Status != string(cloud.AccountStatusActive) {
		t.Errorf("expected Status 'active', got %s", account.Status)
	}
	if account.TenantID != "tenant-1" {
		t.Errorf("expected TenantID 'tenant-1', got %s", account.TenantID)
	}
	if account.CreatedAt.IsZero() || account.UpdatedAt.IsZero() {
		t.Error("expected timestamps to be set")
	}
}
