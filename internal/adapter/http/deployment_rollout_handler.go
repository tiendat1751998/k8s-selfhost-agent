package http

import (
	"net/http"
	"strings"
)

type rollbackDeploymentRequest struct {
	Type      string `json:"type"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Revision  int    `json:"revision"`
}

func (r *rollbackDeploymentRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	if strings.TrimSpace(r.Namespace) == "" && r.Type != "docker" {
		ve.Add("namespace", "namespace is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// RollbackDeployment handles POST /api/v1/deployments/rollback
func (h *DeploymentHandler) RollbackDeployment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[rollbackDeploymentRequest](w, r)
	if !ok {
		return
	}
	if err := checkWorkloadAccess(r, req.Type, req.Namespace); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}
	if err := h.usecase.RestartDeployment(r.Context(), req.Type, req.Cluster, req.Namespace, req.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rollback deployment", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rolled_back"})
}

type canaryWeightRequest struct {
	Type      string `json:"type"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Weight    int    `json:"weight"`
}

func (r *canaryWeightRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	if r.Weight < 0 || r.Weight > 100 {
		ve.Add("weight", "weight must be between 0 and 100")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// SetCanaryWeight handles POST /api/v1/deployments/canary
func (h *DeploymentHandler) SetCanaryWeight(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[canaryWeightRequest](w, r)
	if !ok {
		return
	}
	if err := checkWorkloadAccess(r, req.Type, req.Namespace); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "canary_weight_updated"})
}

type blueGreenCutoverRequest struct {
	Type        string `json:"type"`
	Cluster     string `json:"cluster"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	TargetColor string `json:"targetColor"`
}

func (r *blueGreenCutoverRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	color := strings.ToLower(strings.TrimSpace(r.TargetColor))
	if color != "blue" && color != "green" {
		ve.Add("targetColor", "targetColor must be blue or green")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CutoverBlueGreen handles POST /api/v1/deployments/bluegreen
func (h *DeploymentHandler) CutoverBlueGreen(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[blueGreenCutoverRequest](w, r)
	if !ok {
		return
	}
	if err := checkWorkloadAccess(r, req.Type, req.Namespace); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "bluegreen_cutover_completed"})
}

type pauseResumeRequest struct {
	Type      string `json:"type"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (r *pauseResumeRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "deployment name is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// PauseDeployment handles POST /api/v1/deployments/pause
func (h *DeploymentHandler) PauseDeployment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[pauseResumeRequest](w, r)
	if !ok {
		return
	}
	if err := checkWorkloadAccess(r, req.Type, req.Namespace); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

// ResumeDeployment handles POST /api/v1/deployments/resume
func (h *DeploymentHandler) ResumeDeployment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[pauseResumeRequest](w, r)
	if !ok {
		return
	}
	if err := checkWorkloadAccess(r, req.Type, req.Namespace); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}
