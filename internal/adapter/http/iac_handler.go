package http

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
)

// IaCHandler provides HTTP endpoints for Infrastructure as Code (Terraform & Ansible).
type IaCHandler struct {
	terraform *iac.TerraformRunner
	ansible   *iac.AnsibleRunner
}

// NewIaCHandler creates a new IaCHandler.
func NewIaCHandler(terraform *iac.TerraformRunner, ansible *iac.AnsibleRunner) *IaCHandler {
	return &IaCHandler{
		terraform: terraform,
		ansible:   ansible,
	}
}

// RegisterRoutes registers IaC endpoints on the given Chi router.
func (h *IaCHandler) RegisterRoutes(r chi.Router) {
	r.Get("/status", h.GetStatus)

	r.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Group(func(mut chi.Router) {
		mut.Post("/terraform/plan", h.TerraformPlan)
		mut.Post("/terraform/apply", h.TerraformApply)
		mut.Post("/ansible/playbook", h.AnsiblePlaybook)
	})
}

// GetStatus handles GET /api/v1/iac/status
func (h *IaCHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":              "ready",
		"terraform_configured": h.terraform != nil,
		"ansible_configured":   h.ansible != nil,
	})
}

// TerraformPlan handles POST /api/v1/iac/terraform/plan
func (h *IaCHandler) TerraformPlan(w http.ResponseWriter, r *http.Request) {
	if h.terraform == nil {
		writeError(w, http.StatusServiceUnavailable, "Terraform runner is not configured", nil)
		return
	}

	var opts iac.TerraformRunOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	var out bytes.Buffer
	res, err := h.terraform.Plan(r.Context(), opts, &out)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "terraform plan failed", err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// TerraformApply handles POST /api/v1/iac/terraform/apply
func (h *IaCHandler) TerraformApply(w http.ResponseWriter, r *http.Request) {
	if h.terraform == nil {
		writeError(w, http.StatusServiceUnavailable, "Terraform runner is not configured", nil)
		return
	}

	var opts iac.TerraformRunOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	var out bytes.Buffer
	res, err := h.terraform.Apply(r.Context(), opts, &out)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "terraform apply failed", err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// AnsiblePlaybook handles POST /api/v1/iac/ansible/playbook
func (h *IaCHandler) AnsiblePlaybook(w http.ResponseWriter, r *http.Request) {
	if h.ansible == nil {
		writeError(w, http.StatusServiceUnavailable, "Ansible runner is not configured", nil)
		return
	}

	var opts iac.AnsiblePlaybookOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	var out bytes.Buffer
	res, err := h.ansible.RunPlaybook(r.Context(), opts, &out)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ansible playbook execution failed", err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}
