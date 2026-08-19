package scaffold_test

import (
	"context"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/catalog"
	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
	usecaseScaffold "github.com/datdt/k8sselfhost/internal/usecase/scaffold"
)

func TestEngine_RenderBuiltinTemplates(t *testing.T) {
	engine := usecaseScaffold.NewEngine()
	builtins := scaffold.GetBuiltinTemplates()

	for _, tmpl := range builtins {
		t.Run(tmpl.Name, func(t *testing.T) {
			res, err := engine.RenderTemplate(&tmpl, map[string]string{})
			if err != nil {
				t.Fatalf("unexpected error rendering template %s: %v", tmpl.Name, err)
			}
			if res == nil {
				t.Fatalf("expected non-nil result for template %s", tmpl.Name)
			}
			if res.RenderedYAML == "" {
				t.Errorf("expected non-empty RenderedYAML for %s", tmpl.Name)
			}
			if res.RenderedCompose == "" {
				t.Errorf("expected non-empty RenderedCompose for %s", tmpl.Name)
			}
			if res.RenderedHelm == "" {
				t.Errorf("expected non-empty RenderedHelm for %s", tmpl.Name)
			}

			// Ensure template placeholders are replaced
			if strings.Contains(res.RenderedYAML, "{{.") {
				t.Errorf("RenderedYAML still contains unrendered template tags: %s", res.RenderedYAML)
			}
			if strings.Contains(res.RenderedCompose, "{{.") {
				t.Errorf("RenderedCompose still contains unrendered template tags: %s", res.RenderedCompose)
			}
			if strings.Contains(res.RenderedHelm, "{{.") {
				t.Errorf("RenderedHelm still contains unrendered template tags: %s", res.RenderedHelm)
			}
		})
	}
}

func TestEngine_RenderWithCustomVariables(t *testing.T) {
	engine := usecaseScaffold.NewEngine()
	tmpl := &scaffold.Template{
		ID:   "custom-test",
		Name: "Custom App",
		Variables: []scaffold.TemplateVariable{
			{Name: "app_name", Default: "default-app", Required: true},
			{Name: "port", Default: "8080", Required: true},
		},
		ManifestYAML:  "name: {{.app_name}}\nport: {{.port}}",
		HelmValues:    "name: {{.AppName}}\nport: {{.Port}}",
		DockerCompose: "service: {{.app_name}}\nport: {{.port}}",
	}

	res, err := engine.RenderTemplate(tmpl, map[string]string{
		"app_name": "my-payment-api",
		"port":     "9090",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res.RenderedYAML, "name: my-payment-api") || !strings.Contains(res.RenderedYAML, "port: 9090") {
		t.Errorf("RenderedYAML incorrect: %s", res.RenderedYAML)
	}
	if !strings.Contains(res.RenderedHelm, "name: my-payment-api") || !strings.Contains(res.RenderedHelm, "port: 9090") {
		t.Errorf("RenderedHelm incorrect: %s", res.RenderedHelm)
	}
	if !strings.Contains(res.RenderedCompose, "service: my-payment-api") || !strings.Contains(res.RenderedCompose, "port: 9090") {
		t.Errorf("RenderedCompose incorrect: %s", res.RenderedCompose)
	}
}

func TestEngine_MissingRequiredVariable(t *testing.T) {
	engine := usecaseScaffold.NewEngine()
	tmpl := &scaffold.Template{
		ID:   "req-test",
		Name: "Required Test",
		Variables: []scaffold.TemplateVariable{
			{Name: "mandatory_var", Required: true},
		},
		ManifestYAML: "val: {{.mandatory_var}}",
	}

	_, err := engine.RenderTemplate(tmpl, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing required variable, got nil")
	}
	if !strings.Contains(err.Error(), "mandatory_var") {
		t.Errorf("expected error message to mention 'mandatory_var', got %v", err)
	}
}

type mockCatalogRepo struct {
	entries map[string]*catalog.ServiceEntry
}

func (m *mockCatalogRepo) Create(ctx context.Context, entry *catalog.ServiceEntry) error {
	if m.entries == nil {
		m.entries = make(map[string]*catalog.ServiceEntry)
	}
	m.entries[entry.ID] = entry
	return nil
}
func (m *mockCatalogRepo) GetByID(ctx context.Context, id string) (*catalog.ServiceEntry, error) {
	return m.entries[id], nil
}
func (m *mockCatalogRepo) List(ctx context.Context, tenantID string, filter catalog.ListFilter) ([]catalog.ServiceEntry, error) {
	var list []catalog.ServiceEntry
	for _, e := range m.entries {
		list = append(list, *e)
	}
	return list, nil
}
func (m *mockCatalogRepo) Update(ctx context.Context, entry *catalog.ServiceEntry) error {
	m.entries[entry.ID] = entry
	return nil
}
func (m *mockCatalogRepo) Delete(ctx context.Context, id string) error {
	delete(m.entries, id)
	return nil
}
func (m *mockCatalogRepo) Stats(ctx context.Context, tenantID string) (*catalog.CatalogStats, error) {
	return &catalog.CatalogStats{Total: len(m.entries)}, nil
}

type mockScaffoldRepo struct {
	templates map[string]*scaffold.Template
}

func (m *mockScaffoldRepo) Create(ctx context.Context, t *scaffold.Template) error {
	if m.templates == nil {
		m.templates = make(map[string]*scaffold.Template)
	}
	m.templates[t.ID] = t
	return nil
}
func (m *mockScaffoldRepo) GetByID(ctx context.Context, id string) (*scaffold.Template, error) {
	return m.templates[id], nil
}
func (m *mockScaffoldRepo) List(ctx context.Context, tenantID string, filter scaffold.ListFilter) ([]scaffold.Template, error) {
	var list []scaffold.Template
	for _, t := range m.templates {
		list = append(list, *t)
	}
	return list, nil
}
func (m *mockScaffoldRepo) Update(ctx context.Context, t *scaffold.Template) error {
	m.templates[t.ID] = t
	return nil
}
func (m *mockScaffoldRepo) Delete(ctx context.Context, id string) error {
	delete(m.templates, id)
	return nil
}

func TestUsecase_RenderAndRegisterCatalog(t *testing.T) {
	scaffoldRepo := &mockScaffoldRepo{templates: make(map[string]*scaffold.Template)}
	catalogRepo := &mockCatalogRepo{entries: make(map[string]*catalog.ServiceEntry)}
	engine := usecaseScaffold.NewEngine()
	service := usecaseScaffold.NewService(scaffoldRepo, catalogRepo, engine)

	ctx := context.Background()

	tmpl := &scaffold.Template{
		ID:          "tpl-go",
		Name:        "Go API Service",
		Category:    "api",
		Framework:   "go-chi",
		TenantID:    "tenant-1",
		Variables: []scaffold.TemplateVariable{
			{Name: "app_name", Default: "orders-api", Required: true},
			{Name: "port", Default: "8080", Required: true},
		},
		ManifestYAML: "apiVersion: v1\nkind: Service\nmetadata:\n  name: {{.app_name}}",
	}
	_ = scaffoldRepo.Create(ctx, tmpl)

	res, err := service.Render(ctx, usecaseScaffold.RenderRequest{
		TemplateID: "tpl-go",
		Variables: map[string]string{
			"app_name": "orders-api",
		},
		RegisterCatalog: true,
		OwnerTeam:       "orders-team",
		OwnerEmail:      "orders@example.com",
		TenantID:        "tenant-1",
	})

	if err != nil {
		t.Fatalf("unexpected service render error: %v", err)
	}
	if res.RenderedYAML == "" {
		t.Fatalf("expected rendered YAML")
	}
	if res.CatalogEntryID == "" {
		t.Fatalf("expected catalog entry ID to be generated")
	}

	entry, err := catalogRepo.GetByID(ctx, res.CatalogEntryID)
	if err != nil || entry == nil {
		t.Fatalf("expected service entry to be found in catalog repo")
	}
	if entry.Name != "orders-api" {
		t.Errorf("expected entry name 'orders-api', got %q", entry.Name)
	}
	if entry.OwnerTeam != "orders-team" {
		t.Errorf("expected owner team 'orders-team', got %q", entry.OwnerTeam)
	}
}
