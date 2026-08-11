package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestChallenger_ComprehensiveInputValidation stress tests all request structs and decodeJSON behavior.
func TestChallenger_ComprehensiveInputValidation(t *testing.T) {
	tests := []struct {
		name           string
		inputJSON      string
		validateFn     func() error
		expectedFields []string
		expectErr      bool
	}{
		// 1. Agent Task Request
		{
			name:      "createTaskRequest - empty payload",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createTaskRequest{}
				return req.Validate()
			},
			expectedFields: []string{"title", "phase", "module"},
			expectErr:      true,
		},
		{
			name:      "createTaskRequest - whitespace fields",
			inputJSON: `{"title": "  ", "phase": "   ", "module": " "}`,
			validateFn: func() error {
				req := &createTaskRequest{Title: "  ", Phase: "   ", Module: " "}
				return req.Validate()
			},
			expectedFields: []string{"title", "phase", "module"},
			expectErr:      true,
		},
		{
			name:      "createTaskRequest - valid",
			inputJSON: `{"title": "Fix bug", "phase": "P1", "module": "auth"}`,
			validateFn: func() error {
				req := &createTaskRequest{Title: "Fix bug", Phase: "P1", Module: "auth"}
				return req.Validate()
			},
			expectErr: false,
		},

		// 2. AI Provider Request
		{
			name:      "addProviderRequest - invalid type & missing fields",
			inputJSON: `{"name": "", "type": "unknown_ai", "endpoint": "", "model": ""}`,
			validateFn: func() error {
				req := &addProviderRequest{Name: "", Type: "unknown_ai", Endpoint: "", Model: ""}
				return req.Validate()
			},
			expectedFields: []string{"name", "type", "endpoint", "model"},
			expectErr:      true,
		},
		{
			name:      "addProviderRequest - valid ollama",
			inputJSON: `{"name": "local", "type": "ollama", "endpoint": "http://localhost:11434", "model": "llama3"}`,
			validateFn: func() error {
				req := &addProviderRequest{Name: "local", Type: "ollama", Endpoint: "http://localhost:11434", Model: "llama3"}
				return req.Validate()
			},
			expectErr: false,
		},

		// 3. AI Test Prompt Request
		{
			name:      "testPromptRequest - missing prompt",
			inputJSON: `{"provider": "ollama"}`,
			validateFn: func() error {
				req := &testPromptRequest{Provider: "ollama"}
				return req.Validate()
			},
			expectedFields: []string{"prompt"},
			expectErr:      true,
		},

		// 4. Auth Login Request
		{
			name:      "loginRequest - missing credentials",
			inputJSON: `{"email": "", "password": ""}`,
			validateFn: func() error {
				req := &loginRequest{}
				return req.Validate()
			},
			expectedFields: []string{"email", "password"},
			expectErr:      true,
		},

		// 5. Automation Create Rule Request
		{
			name:      "createRuleRequest - missing required fields",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createRuleRequest{}
				return req.Validate()
			},
			expectedFields: []string{"name", "trigger_type", "action_type"},
			expectErr:      true,
		},

		// 6. Automation Update Rule Request
		{
			name:      "updateRuleRequest - missing required fields",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &updateRuleRequest{}
				return req.Validate()
			},
			expectedFields: []string{"name", "trigger_type", "action_type"},
			expectErr:      true,
		},

		// 7. Backup Trigger Recovery Request
		{
			name:      "triggerRecoveryRequest - missing target",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &triggerRecoveryRequest{}
				return req.Validate()
			},
			expectedFields: []string{"target"},
			expectErr:      true,
		},

		// 8. Capacity Forecast Request
		{
			name:      "recordForecastRequest - missing fields",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &recordForecastRequest{}
				return req.Validate()
			},
			expectedFields: []string{"cluster", "resource_type"},
			expectErr:      true,
		},

		// 9. Change Management Request
		{
			name:      "createChangeRequest - empty fields",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createChangeRequest{}
				return req.Validate()
			},
			expectedFields: []string{"title", "requester", "cluster", "namespace"},
			expectErr:      true,
		},

		// 10. Correlation Request
		{
			name:      "createCorrelatedRequest - empty fields",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createCorrelatedRequest{}
				return req.Validate()
			},
			expectedFields: []string{"title", "root_cause", "severity"},
			expectErr:      true,
		},

		// 11. Deployment Scale Request (Out-of-bounds number testing)
		{
			name:      "scaleDeploymentRequest - negative replicas",
			inputJSON: `{"name": "app", "namespace": "default", "replicas": -10}`,
			validateFn: func() error {
				req := &scaleDeploymentRequest{Name: "app", Namespace: "default", Replicas: -10}
				return req.Validate()
			},
			expectedFields: []string{"replicas"},
			expectErr:      true,
		},
		{
			name:      "scaleDeploymentRequest - missing name & namespace",
			inputJSON: `{"replicas": 2}`,
			validateFn: func() error {
				req := &scaleDeploymentRequest{Replicas: 2}
				return req.Validate()
			},
			expectedFields: []string{"name", "namespace"},
			expectErr:      true,
		},

		// 12. Deployment Restart Request
		{
			name:      "restartDeploymentRequest - missing name",
			inputJSON: `{"namespace": "default"}`,
			validateFn: func() error {
				req := &restartDeploymentRequest{Namespace: "default"}
				return req.Validate()
			},
			expectedFields: []string{"name"},
			expectErr:      true,
		},

		// 13. Deployment Delete Request
		{
			name:      "deleteDeploymentRequest - missing namespace",
			inputJSON: `{"name": "app"}`,
			validateFn: func() error {
				req := &deleteDeploymentRequest{Name: "app"}
				return req.Validate()
			},
			expectedFields: []string{"namespace"},
			expectErr:      true,
		},

		// 14. Deployment Create Request
		{
			name:      "createDeploymentRequest - missing image and negative replicas",
			inputJSON: `{"name": "web", "namespace": "prod", "replicas": -1}`,
			validateFn: func() error {
				req := &createDeploymentRequest{}
				req.Name = "web"
				req.Namespace = "prod"
				req.Replicas = -1
				return req.Validate()
			},
			expectedFields: []string{"image", "replicas"},
			expectErr:      true,
		},

		// 15. Docker Scale Service Request
		{
			name:      "scaleServiceRequest - negative replicas",
			inputJSON: `{"replicas": -5}`,
			validateFn: func() error {
				req := &scaleServiceRequest{Replicas: -5}
				return req.Validate()
			},
			expectedFields: []string{"replicas"},
			expectErr:      true,
		},

		// 16. Docker Toggle Container Request
		{
			name:      "toggleContainerRequest - invalid action string",
			inputJSON: `{"action": "restart"}`,
			validateFn: func() error {
				req := &toggleContainerRequest{Action: "restart"}
				return req.Validate()
			},
			expectedFields: []string{"action"},
			expectErr:      true,
		},

		// 17. Drift Record Request
		{
			name:      "createDriftRequest - missing resource",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createDriftRequest{}
				return req.Validate()
			},
			expectedFields: []string{"cluster", "resource", "resource_kind"},
			expectErr:      true,
		},

		// 18. Explorer Sync Request
		{
			name:      "syncResourceRequest - empty payload",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &syncResourceRequest{}
				return req.Validate()
			},
			expectedFields: []string{"kind", "name", "cluster"},
			expectErr:      true,
		},

		// 19. Fleet Register Cluster Request
		{
			name:      "registerClusterRequest - missing region",
			inputJSON: `{"name": "c1", "provider": "aws"}`,
			validateFn: func() error {
				req := &registerClusterRequest{}
				req.Name = "c1"
				req.Provider = "aws"
				return req.Validate()
			},
			expectedFields: []string{"region"},
			expectErr:      true,
		},

		// 20. GitOps Create PR Request
		{
			name:      "createPRRequest - missing incident_id and title",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createPRRequest{}
				return req.Validate()
			},
			expectedFields: []string{"incident_id"},
			expectErr:      true,
		},

		// 21. Health Center Ping Request
		{
			name:      "pingComponentRequest - missing status",
			inputJSON: `{"component": "db"}`,
			validateFn: func() error {
				req := &pingComponentRequest{Component: "db"}
				return req.Validate()
			},
			expectedFields: []string{"status"},
			expectErr:      true,
		},

		// 22. Notification Channel Request
		{
			name:      "createChannelRequest - empty payload",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createChannelRequest{}
				return req.Validate()
			},
			expectedFields: []string{"name", "type"},
			expectErr:      true,
		},

		// 23. Promotion Create Request
		{
			name:      "createPromotionRequest - missing environment targets",
			inputJSON: `{"service": "auth", "version": "v1.0"}`,
			validateFn: func() error {
				req := &createPromotionRequest{}
				req.Service = "auth"
				req.Version = "v1.0"
				return req.Validate()
			},
			expectedFields: []string{"from_env", "to_env"},
			expectErr:      true,
		},

		// 24. Reporting Generate Request
		{
			name:      "generateReportRequest - missing type",
			inputJSON: `{"title": "Monthly Report"}`,
			validateFn: func() error {
				req := &generateReportRequest{}
				req.Title = "Monthly Report"
				return req.Validate()
			},
			expectedFields: []string{"type"},
			expectErr:      true,
		},

		// 25. Runbook Create Request
		{
			name:      "createRunbookRequest - missing category",
			inputJSON: `{"title": "Restart Pods"}`,
			validateFn: func() error {
				req := &createRunbookRequest{}
				req.Title = "Restart Pods"
				return req.Validate()
			},
			expectedFields: []string{"category"},
			expectErr:      true,
		},

		// 26. Runbook Update Request
		{
			name:      "updateRunbookRequest - missing title",
			inputJSON: `{"category": "k8s"}`,
			validateFn: func() error {
				req := &updateRunbookRequest{}
				req.Category = "k8s"
				return req.Validate()
			},
			expectedFields: []string{"title"},
			expectErr:      true,
		},

		// 27. Tagging Create Request
		{
			name:      "createTagRequest - missing value",
			inputJSON: `{"key": "env"}`,
			validateFn: func() error {
				req := &createTagRequest{}
				req.Key = "env"
				return req.Validate()
			},
			expectedFields: []string{"value"},
			expectErr:      true,
		},

		// 28. Tagging Resource Request
		{
			name:      "tagResourceRequest - missing tag_id",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &tagResourceRequest{}
				return req.Validate()
			},
			expectedFields: []string{"tag_id"},
			expectErr:      true,
		},

		// 29. Tenancy Create Organization Request
		{
			name:      "createOrganizationRequest - missing ID and Name",
			inputJSON: `{}`,
			validateFn: func() error {
				req := &createOrganizationRequest{}
				return req.Validate()
			},
			expectedFields: []string{"id", "name"},
			expectErr:      true,
		},

		// 30. Tenancy Create Project Request
		{
			name:      "createProjectRequest - missing OrgID",
			inputJSON: `{"id": "p1", "name": "Project 1"}`,
			validateFn: func() error {
				req := &createProjectRequest{}
				req.ID = "p1"
				req.Name = "Project 1"
				return req.Validate()
			},
			expectedFields: []string{"org_id"},
			expectErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.validateFn()
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected validation error, got nil")
				}
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Fatalf("expected *ValidationError, got %T", err)
				}
				for _, f := range tc.expectedFields {
					if _, exists := ve.Fields[f]; !exists {
						t.Errorf("expected field error for '%s' in %v", f, ve.Fields)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("expected no validation error, got: %v", err)
				}
			}
		})
	}
}

// TestChallenger_DecodeJSON_HTTPStatus400 verifies that malformed JSON or validation errors produce HTTP 400 Bad Request responses.
func TestChallenger_DecodeJSON_HTTPStatus400(t *testing.T) {
	t.Run("Malformed JSON produces 400 Bad Request", func(t *testing.T) {
		malformedBodies := []string{
			`{invalid json`,
			`{"title": `,
			`[1, 2, 3]`, // JSON array when struct expected
			`"just a string"`,
		}

		for _, b := range malformedBodies {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/tasks", bytes.NewBufferString(b))
			w := httptest.NewRecorder()

			_, ok := decodeJSON[createTaskRequest](w, req)
			if ok {
				t.Errorf("expected decodeJSON to fail for body: %s", b)
			}
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected HTTP 400 Bad Request for body '%s', got %d", b, w.Code)
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to parse response JSON: %v", err)
			}
			if resp["error"] != "invalid request body" {
				t.Errorf("expected error 'invalid request body', got '%v'", resp["error"])
			}
		}
	})

	t.Run("Validation failure produces 400 Bad Request with details map", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/tasks", bytes.NewBufferString(`{}`))
		w := httptest.NewRecorder()

		_, ok := decodeJSON[createTaskRequest](w, req)
		if ok {
			t.Errorf("expected decodeJSON to fail validation")
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected HTTP 400 Bad Request, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to parse response JSON: %v", err)
		}
		if resp["error"] != "validation failed" {
			t.Errorf("expected error 'validation failed', got '%v'", resp["error"])
		}
		details, ok := resp["details"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected details object in response, got %T", resp["details"])
		}
		if len(details) == 0 {
			t.Errorf("expected non-empty details map for invalid fields")
		}
	})
}
