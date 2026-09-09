package internal_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"gopkg.in/yaml.v3"
)

func findRepoRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root containing go.mod")
		}
		dir = parent
	}
}

func TestDockerfilesHardening(t *testing.T) {
	root := findRepoRoot(t)
	dockerfiles := []string{
		filepath.Join(root, "deployments", "docker", "Dockerfile"),
		filepath.Join(root, "deployments", "docker", "Dockerfile.agent"),
		filepath.Join(root, "Dockerfile"),
		filepath.Join(root, "Dockerfile.agent"),
		filepath.Join(root, "orchestrator-agent", "Dockerfile"),
	}

	for _, dfPath := range dockerfiles {
		t.Run(filepath.Base(dfPath), func(t *testing.T) {
			content, err := os.ReadFile(dfPath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", dfPath, err)
			}
			str := string(content)

			// 1. Must define non-root user
			if !strings.Contains(str, "USER ") {
				t.Errorf("%s: missing USER instruction", dfPath)
			}

			// 2. Must define HEALTHCHECK
			if !strings.Contains(str, "HEALTHCHECK ") {
				t.Errorf("%s: missing HEALTHCHECK instruction", dfPath)
			}

			// 3. For Go Dockerfiles, verify multi-stage build
			if strings.Contains(str, "golang:") {
				if !strings.Contains(str, "AS builder") {
					t.Errorf("%s: Go build should use multi-stage 'AS builder'", dfPath)
				}
				if !strings.Contains(str, "FROM gcr.io/distroless/static-debian12:nonroot") {
					t.Errorf("%s: runtime stage should use distroless nonroot static image", dfPath)
				}
			}
		})
	}
}

func TestDockerComposeHardening(t *testing.T) {
	root := findRepoRoot(t)
	composePath := filepath.Join(root, "deployments", "docker", "docker-compose.yml")

	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read compose file: %v", err)
	}

	var compose struct {
		Services map[string]struct {
			Restart     string `yaml:"restart"`
			Healthcheck struct {
				Test []string `yaml:"test"`
			} `yaml:"healthcheck"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("invalid YAML in compose file: %v", err)
	}

	expectedServices := []string{"app", "postgres", "redis", "nats"}
	for _, svc := range expectedServices {
		s, ok := compose.Services[svc]
		if !ok {
			t.Errorf("missing service %s in docker-compose.yml", svc)
			continue
		}
		if s.Restart == "" {
			t.Errorf("service %s missing restart policy", svc)
		}
		if len(s.Healthcheck.Test) == 0 {
			t.Errorf("service %s missing healthcheck definition", svc)
		}
	}
}

func TestKubernetesDeploymentsHardening(t *testing.T) {
	root := findRepoRoot(t)
	manifests := []string{
		filepath.Join(root, "deployments", "k8s", "deployment.yaml"),
		filepath.Join(root, "deploy", "logging", "fluentbit-config.yaml"),
		filepath.Join(root, "deploy", "logging", "loki-stack.yaml"),
		filepath.Join(root, "deploy", "logging", "vector-daemonset.yaml"),
		filepath.Join(root, "deploy", "traffic", "ingress-nginx.yaml"),
	}

	for _, mPath := range manifests {
		t.Run(filepath.Base(mPath), func(t *testing.T) {
			content, err := os.ReadFile(mPath)
			if err != nil {
				t.Fatalf("failed to read manifest %s: %v", mPath, err)
			}
			str := string(content)

			// Verify CPU and Memory limits/requests
			if !strings.Contains(str, "resources:") || !strings.Contains(str, "limits:") || !strings.Contains(str, "requests:") {
				t.Errorf("%s: missing explicit resource requests and limits", mPath)
			}

			// Verify probes
			if !strings.Contains(str, "livenessProbe:") {
				t.Errorf("%s: missing livenessProbe", mPath)
			}
			if !strings.Contains(str, "readinessProbe:") {
				t.Errorf("%s: missing readinessProbe", mPath)
			}

			// Verify securityContext
			if !strings.Contains(str, "allowPrivilegeEscalation: false") {
				t.Errorf("%s: missing allowPrivilegeEscalation: false", mPath)
			}
			if !strings.Contains(str, "runAsNonRoot: true") {
				t.Errorf("%s: missing runAsNonRoot: true", mPath)
			}
		})
	}
}

func TestHelmChartHardening(t *testing.T) {
	root := findRepoRoot(t)
	valuesPath := filepath.Join(root, "deployments", "helm", "k8sselfhost", "values.yaml")
	deploymentPath := filepath.Join(root, "deployments", "helm", "k8sselfhost", "templates", "deployment.yaml")

	valData, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("failed to read helm values: %v", err)
	}

	valStr := string(valData)
	if !strings.Contains(valStr, "runAsNonRoot: true") {
		t.Errorf("helm values.yaml missing runAsNonRoot: true")
	}
	if !strings.Contains(valStr, "allowPrivilegeEscalation: false") {
		t.Errorf("helm values.yaml missing allowPrivilegeEscalation: false")
	}
	if !strings.Contains(valStr, "readOnlyRootFilesystem: true") {
		t.Errorf("helm values.yaml missing readOnlyRootFilesystem: true")
	}

	deployData, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatalf("failed to read helm deployment: %v", err)
	}

	deployStr := string(deployData)
	if !strings.Contains(deployStr, "livenessProbe:") || !strings.Contains(deployStr, "readinessProbe:") {
		t.Errorf("helm deployment template missing livenessProbe or readinessProbe")
	}
	if !strings.Contains(deployStr, "timeoutSeconds:") {
		t.Errorf("helm deployment template missing explicit timeoutSeconds")
	}
}
