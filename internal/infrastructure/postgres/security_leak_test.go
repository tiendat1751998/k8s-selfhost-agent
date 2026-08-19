package postgres_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
)

func init() {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	}
}

func TestFleetCluster_EntityJSONNeverSerializesEncryptedToken(t *testing.T) {
	sensitiveKubeconfig := "sensitive-admin-kubeconfig-data-token-12345"
	c := fleet.Cluster{
		ID:             "cluster-1",
		Name:           "prod-k8s",
		Group:          "production",
		Region:         "us-east-1",
		Provider:       "aws",
		Status:         "active",
		EncryptedToken: sensitiveKubeconfig,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("failed to marshal cluster: %v", err)
	}

	str := string(data)
	if strings.Contains(str, sensitiveKubeconfig) {
		t.Fatalf("CRITICAL SECURITY LEAK: sensitive token %q found in marshaled Cluster JSON: %s", sensitiveKubeconfig, str)
	}
	if strings.Contains(str, "encrypted_token") {
		t.Fatalf("encrypted_token tag was not ignored (json:\"-\") in marshaled JSON: %s", str)
	}
}

func TestBackupStorage_EntityJSONNeverSerializesCredentials(t *testing.T) {
	secretKey := "super-secret-aws-access-key-999"
	s := backup.BackupStorage{
		ID:       "storage-1",
		TenantID: "tenant-a",
		Name:     "s3-backup",
		Type:     "s3",
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "my-bucket",
		Credentials: map[string]string{
			"access_key": "AKIAIOSFODNN7EXAMPLE",
			"secret_key": secretKey,
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("failed to marshal backup storage: %v", err)
	}

	str := string(data)
	if strings.Contains(str, secretKey) {
		t.Fatalf("CRITICAL SECURITY LEAK: secret key %q found in marshaled BackupStorage JSON: %s", secretKey, str)
	}
	if strings.Contains(str, "credentials") {
		t.Fatalf("credentials tag was not ignored (json:\"-\") in marshaled JSON: %s", str)
	}
}
