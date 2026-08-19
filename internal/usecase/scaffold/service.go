package scaffold

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/catalog"
	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
)

// RenderRequest specifies template rendering parameters.
type RenderRequest struct {
	TemplateID      string            `json:"template_id"`
	CustomTemplate  *scaffold.Template `json:"custom_template,omitempty"`
	Variables       map[string]string `json:"variables"`
	RegisterCatalog bool              `json:"register_catalog,omitempty"`
	OwnerTeam       string            `json:"owner_team,omitempty"`
	OwnerEmail      string            `json:"owner_email,omitempty"`
	TenantID        string            `json:"tenant_id,omitempty"`
}

// RenderResponse contains rendered YAML, Compose, Helm manifests and optional catalog entry ID.
type RenderResponse struct {
	RenderedYAML    string `json:"rendered_yaml"`
	RenderedCompose string `json:"rendered_compose"`
	RenderedHelm    string `json:"rendered_helm"`
	CatalogEntryID  string `json:"catalog_entry_id,omitempty"`
}

// Service provides high-level business logic for scaffold templates.
type Service interface {
	ListTemplates(ctx context.Context, tenantID string, filter scaffold.ListFilter) ([]scaffold.Template, error)
	GetTemplate(ctx context.Context, id string) (*scaffold.Template, error)
	CreateTemplate(ctx context.Context, t *scaffold.Template) error
	UpdateTemplate(ctx context.Context, t *scaffold.Template) error
	DeleteTemplate(ctx context.Context, id string) error
	Render(ctx context.Context, req RenderRequest) (*RenderResponse, error)
}

type serviceImpl struct {
	repo        scaffold.Repository
	catalogRepo catalog.Repository
	engine      Engine
}

// NewService creates a new scaffold service.
func NewService(repo scaffold.Repository, catalogRepo catalog.Repository, engine Engine) Service {
	if engine == nil {
		engine = NewEngine()
	}
	return &serviceImpl{
		repo:        repo,
		catalogRepo: catalogRepo,
		engine:      engine,
	}
}

func (s *serviceImpl) ListTemplates(ctx context.Context, tenantID string, filter scaffold.ListFilter) ([]scaffold.Template, error) {
	return s.repo.List(ctx, tenantID, filter)
}

func (s *serviceImpl) GetTemplate(ctx context.Context, id string) (*scaffold.Template, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *serviceImpl) CreateTemplate(ctx context.Context, t *scaffold.Template) error {
	if t.ID == "" {
		t.ID = "tpl-" + uuid.NewString()[:8]
	}
	if t.Variables == nil {
		t.Variables = []scaffold.TemplateVariable{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	return s.repo.Create(ctx, t)
}

func (s *serviceImpl) UpdateTemplate(ctx context.Context, t *scaffold.Template) error {
	t.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, t)
}

func (s *serviceImpl) DeleteTemplate(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *serviceImpl) Render(ctx context.Context, req RenderRequest) (*RenderResponse, error) {
	var targetTemplate *scaffold.Template

	if req.CustomTemplate != nil {
		targetTemplate = req.CustomTemplate
	} else if req.TemplateID != "" {
		tmpl, err := s.repo.GetByID(ctx, req.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("fetching template %s: %w", req.TemplateID, err)
		}
		if tmpl == nil {
			return nil, fmt.Errorf("template %q not found", req.TemplateID)
		}
		targetTemplate = tmpl
	} else {
		return nil, fmt.Errorf("either template_id or custom_template must be provided")
	}

	result, err := s.engine.RenderTemplate(targetTemplate, req.Variables)
	if err != nil {
		return nil, err
	}

	resp := &RenderResponse{
		RenderedYAML:    result.RenderedYAML,
		RenderedCompose: result.RenderedCompose,
		RenderedHelm:    result.RenderedHelm,
	}

	// Register in service catalog if requested and catalogRepo is configured
	if req.RegisterCatalog && s.catalogRepo != nil {
		appName := req.Variables["app_name"]
		if appName == "" {
			appName = req.Variables["db_name"]
		}
		if appName == "" {
			appName = req.Variables["AppName"]
		}
		if appName == "" {
			appName = targetTemplate.Name
		}

		serviceType := catalog.TypeService
		switch targetTemplate.Category {
		case scaffold.CategoryAPI:
			serviceType = catalog.TypeAPI
		case scaffold.CategoryDatabase:
			serviceType = catalog.TypeDatabase
		case scaffold.CategoryWeb:
			serviceType = catalog.TypeFrontend
		case scaffold.CategoryWorker:
			serviceType = catalog.TypeWorker
		}

		namespace := req.Variables["namespace"]
		if namespace == "" {
			namespace = "default"
		}

		tenantID := req.TenantID
		if tenantID == "" {
			tenantID = targetTemplate.TenantID
		}
		if tenantID == "" {
			tenantID = "default-tenant"
		}

		catalogEntry := &catalog.ServiceEntry{
			ID:          "srv-" + uuid.NewString()[:8],
			Name:        appName,
			Description: fmt.Sprintf("Scaffolded from template %s (%s)", targetTemplate.Name, targetTemplate.Framework),
			Type:        serviceType,
			Lifecycle:   catalog.LifecycleDevelopment,
			OwnerTeam:   req.OwnerTeam,
			OwnerEmail:  req.OwnerEmail,
			Tags:        targetTemplate.Tags,
			Annotations: map[string]string{
				"k8s.io/namespace":        namespace,
				"scaffolder/template-id":  targetTemplate.ID,
				"scaffolder/framework":    targetTemplate.Framework,
				"scaffolder/generated-at": time.Now().UTC().Format(time.RFC3339),
			},
			TenantID: tenantID,
		}

		if err := s.catalogRepo.Create(ctx, catalogEntry); err != nil {
			// Log error but still return rendered templates if catalog entry creation had non-fatal constraint
			// (or return error if critical)
			// Return catalog error with context
			return nil, fmt.Errorf("registering scaffolded service in catalog: %w", err)
		}

		resp.CatalogEntryID = catalogEntry.ID
	}

	return resp, nil
}
