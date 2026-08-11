package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidationError(t *testing.T) {
	ve := NewValidationError("validation failed")
	if ve.HasErrors() {
		t.Errorf("expected no errors initially")
	}

	ve.Add("name", "name is required")
	ve.Add("replicas", "replicas must be >= 0")

	if !ve.HasErrors() {
		t.Errorf("expected errors after adding fields")
	}

	if len(ve.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(ve.Fields))
	}

	errMsg := ve.Error()
	if errMsg == "" {
		t.Errorf("expected non-empty error message")
	}
}

type testValidatable struct {
	Name     string `json:"name"`
	Replicas int    `json:"replicas"`
}

func (r *testValidatable) Validate() error {
	ve := NewValidationError("validation failed")
	if r.Name == "" {
		ve.Add("name", "name is required")
	}
	if r.Replicas < 0 {
		ve.Add("replicas", "replicas must be >= 0")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func TestDecodeJSON_ValidationSuccess(t *testing.T) {
	body := `{"name": "nginx", "replicas": 3}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	decoded, ok := decodeJSON[testValidatable](w, req)
	if !ok {
		t.Fatalf("expected decodeJSON to succeed")
	}
	if decoded.Name != "nginx" || decoded.Replicas != 3 {
		t.Errorf("unexpected decoded struct: %+v", decoded)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDecodeJSON_ValidationFailure(t *testing.T) {
	body := `{"name": "", "replicas": -1}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	decoded, ok := decodeJSON[testValidatable](w, req)
	if ok {
		t.Fatalf("expected decodeJSON to fail validation")
	}
	if decoded != nil {
		t.Errorf("expected nil decoded struct on failure")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp["error"] != "validation failed" {
		t.Errorf("expected error 'validation failed', got %v", resp["error"])
	}

	details, ok := resp["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected details map in response, got %v", resp["details"])
	}

	if details["name"] != "name is required" {
		t.Errorf("expected name error in details, got %v", details["name"])
	}
	if details["replicas"] != "replicas must be >= 0" {
		t.Errorf("expected replicas error in details, got %v", details["replicas"])
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	_, ok := decodeJSON[testValidatable](w, req)
	if ok {
		t.Fatalf("expected decodeJSON to fail on malformed JSON")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandlerRequestValidations(t *testing.T) {
	t.Run("createTaskRequest", func(t *testing.T) {
		req := &createTaskRequest{}
		err := req.Validate()
		if err == nil {
			t.Fatalf("expected validation error for empty createTaskRequest")
		}
		ve := err.(*ValidationError)
		if _, ok := ve.Fields["title"]; !ok {
			t.Errorf("expected field 'title' in validation error")
		}
	})

	t.Run("addProviderRequest", func(t *testing.T) {
		req := &addProviderRequest{Name: "p1", Type: "invalid_type"}
		err := req.Validate()
		if err == nil {
			t.Fatalf("expected validation error for invalid provider type")
		}
		ve := err.(*ValidationError)
		if _, ok := ve.Fields["type"]; !ok {
			t.Errorf("expected field 'type' in validation error")
		}
	})

	t.Run("scaleDeploymentRequest", func(t *testing.T) {
		req := &scaleDeploymentRequest{Name: "dep1", Namespace: "default", Replicas: -5}
		err := req.Validate()
		if err == nil {
			t.Fatalf("expected validation error for negative replicas")
		}
		ve := err.(*ValidationError)
		if _, ok := ve.Fields["replicas"]; !ok {
			t.Errorf("expected field 'replicas' in validation error")
		}
	})

	t.Run("toggleContainerRequest", func(t *testing.T) {
		req := &toggleContainerRequest{Action: "invalid_action"}
		err := req.Validate()
		if err == nil {
			t.Fatalf("expected validation error for invalid action")
		}
		ve := err.(*ValidationError)
		if _, ok := ve.Fields["action"]; !ok {
			t.Errorf("expected field 'action' in validation error")
		}
	})
}
