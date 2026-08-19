package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/backup"
	backupUsecase "github.com/datdt/k8sselfhost/internal/usecase/backup"
)

type BackupHandler struct {
	usecase *backupUsecase.Usecase
}

func NewBackupHandler(usecase *backupUsecase.Usecase) *BackupHandler {
	return &BackupHandler{usecase: usecase}
}

func (h *BackupHandler) RegisterRoutes(r chi.Router) {
	r.Post("/storages", h.CreateStorage)
	r.Get("/storages", h.ListStorages)
	r.Post("/policies", h.CreatePolicy)
	r.Get("/policies", h.ListPolicies)
	r.Post("/jobs", h.TriggerBackup)
	r.Get("/jobs", h.ListJobs)
	r.Post("/restores", h.TriggerRestore)
	r.Get("/restores", h.ListRestores)
}

type createStorageRequest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Endpoint    string            `json:"endpoint"`
	Bucket      string            `json:"bucket"`
	Credentials map[string]string `json:"credentials"`
}

func (r *createStorageRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Type) == "" {
		ve.Add("type", "type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func normalizeTenantID(r *http.Request) string {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if tenantID == "" || tenantID == "default" {
		return "default-tenant"
	}
	return tenantID
}

func (h *BackupHandler) CreateStorage(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createStorageRequest](w, r)
	if !ok {
		return
	}

	tenantID := normalizeTenantID(r)

	storage := &backup.BackupStorage{
		TenantID:    tenantID,
		Name:        req.Name,
		Type:        req.Type,
		Endpoint:    req.Endpoint,
		Bucket:      req.Bucket,
		Credentials: req.Credentials,
	}

	if err := h.usecase.CreateStorage(r.Context(), storage); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup storage", err)
		return
	}
	storage.Credentials = nil
	writeJSON(w, http.StatusCreated, storage)
}

func (h *BackupHandler) ListStorages(w http.ResponseWriter, r *http.Request) {
	tenantID := normalizeTenantID(r)

	storages, err := h.usecase.ListStorages(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backup storages", err)
		return
	}
	for _, s := range storages {
		if s != nil {
			s.Credentials = nil
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": storages})
}

type createPolicyRequest struct {
	Name           string `json:"name"`
	DBType         string `json:"db_type"`
	DBHost         string `json:"db_host"`
	DBPort         int    `json:"db_port"`
	DBName         string `json:"db_name"`
	StorageID      string `json:"storage_id"`
	Schedule       string `json:"schedule"`
	RetentionCount int    `json:"retention_count"`
	BackupType     string `json:"backup_type"`
	Enabled        bool   `json:"enabled"`
}

func (r *createPolicyRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.StorageID) == "" {
		ve.Add("storage_id", "storage_id is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *BackupHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createPolicyRequest](w, r)
	if !ok {
		return
	}

	tenantID := normalizeTenantID(r)

	policy := &backup.BackupPolicy{
		TenantID:       tenantID,
		Name:           req.Name,
		DBType:         req.DBType,
		DBHost:         req.DBHost,
		DBPort:         req.DBPort,
		DBName:         req.DBName,
		StorageID:      req.StorageID,
		Schedule:       req.Schedule,
		RetentionCount: req.RetentionCount,
		BackupType:     req.BackupType,
		Enabled:        req.Enabled,
	}

	if err := h.usecase.CreatePolicy(r.Context(), policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup policy", err)
		return
	}
	writeJSON(w, http.StatusCreated, policy)
}

func (h *BackupHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	tenantID := normalizeTenantID(r)

	policies, err := h.usecase.ListPolicies(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backup policies", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": policies})
}

type triggerBackupRequest struct {
	PolicyID    string `json:"policy_id"`
	BackupType  string `json:"backup_type"`
	StoragePath string `json:"storage_path"`
}

func (r *triggerBackupRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.PolicyID) == "" {
		ve.Add("policy_id", "policy_id is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *BackupHandler) TriggerBackup(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[triggerBackupRequest](w, r)
	if !ok {
		return
	}

	tenantID := normalizeTenantID(r)

	job := &backup.BackupJob{
		TenantID:    tenantID,
		PolicyID:    req.PolicyID,
		BackupType:  req.BackupType,
		StoragePath: req.StoragePath,
		Status:      "running",
	}

	if err := h.usecase.TriggerBackup(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger backup", err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (h *BackupHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	tenantID := normalizeTenantID(r)

	jobs, err := h.usecase.ListJobs(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backup jobs", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": jobs})
}

type triggerRestoreRequest struct {
	BackupJobID  string `json:"backup_job_id"`
	TargetDBHost string `json:"target_db_host"`
	TargetDBName string `json:"target_db_name"`
}

func (r *triggerRestoreRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.BackupJobID) == "" {
		ve.Add("backup_job_id", "backup_job_id is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (h *BackupHandler) TriggerRestore(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[triggerRestoreRequest](w, r)
	if !ok {
		return
	}

	tenantID := normalizeTenantID(r)

	restore := &backup.RestoreJob{
		TenantID:     tenantID,
		BackupJobID:  req.BackupJobID,
		TargetDBHost: req.TargetDBHost,
		TargetDBName: req.TargetDBName,
		Status:       "running",
	}

	if err := h.usecase.TriggerRestore(r.Context(), restore); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger restore", err)
		return
	}
	writeJSON(w, http.StatusCreated, restore)
}

func (h *BackupHandler) ListRestores(w http.ResponseWriter, r *http.Request) {
	tenantID := normalizeTenantID(r)

	restores, err := h.usecase.ListRestores(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list restores", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": restores})
}
