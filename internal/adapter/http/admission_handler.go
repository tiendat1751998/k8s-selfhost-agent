// Package http provides HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	domainsec "github.com/datdt/k8sselfhost/internal/domain/security"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"go.uber.org/zap"
)

// ProblemDetails represents an RFC 7807 problem details object.
type ProblemDetails struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code,omitempty"`
}

// AdmissionHandler handles Kubernetes validating admission webhook requests.
type AdmissionHandler struct {
	enforcer domainsec.PolicyEnforcer
}

// NewAdmissionHandler creates a new AdmissionHandler instance.
func NewAdmissionHandler(enforcer domainsec.PolicyEnforcer) *AdmissionHandler {
	return &AdmissionHandler{
		enforcer: enforcer,
	}
}

// RegisterRoutes mounts admission webhook routes onto the chi router.
func (h *AdmissionHandler) RegisterRoutes(r chi.Router) {
	r.Post("/validate", h.ValidateAdmission)
}

// ValidateAdmission handles POST /api/v1/security/admission/validate.
// It parses admissionv1.AdmissionReview, evaluates pod specifications against
// enterprise zero-trust security policies, and returns the populated AdmissionReview.
func (h *AdmissionHandler) ValidateAdmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeProblem(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only POST requests are supported", "METHOD_NOT_ALLOWED")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Failed to read request body", "READ_ERROR")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Request body cannot be empty", "VALIDATION_ERROR")
		return
	}

	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Failed to decode AdmissionReview JSON: %v", err), "VALIDATION_ERROR")
		return
	}

	if review.Request == nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "AdmissionReview.request cannot be nil", "VALIDATION_ERROR")
		return
	}

	result, err := h.enforcer.ValidatePodSpec(r.Context(), &review)
	if err != nil {
		logger.Get().Error("Admission policy evaluation failed", zap.Error(err))
		h.writeProblem(w, http.StatusBadRequest, "Validation Error", err.Error(), "EVALUATION_ERROR")
		return
	}

	responseReview := admissionv1.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: &admissionv1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: result.Allowed,
			Result: &metav1.Status{
				Code:    result.StatusCode,
				Message: result.Reason,
			},
		},
	}

	// Populate warnings with individual rule violations for client visibility
	if !result.Allowed && len(result.Violations) > 0 {
		warnings := make([]string, 0, len(result.Violations))
		for _, v := range result.Violations {
			warnings = append(warnings, fmt.Sprintf("[%s] %s: %s", v.Rule, v.Field, v.Message))
		}
		responseReview.Response.Warnings = warnings
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(responseReview); err != nil {
		logger.Get().Error("Failed to write admission review response", zap.Error(err))
	}
}

func (h *AdmissionHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code string) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: "/api/v1/security/admission/validate",
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
