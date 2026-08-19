package scaffold_test

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
)

func TestBuiltinTemplates_ValidAndComplete(t *testing.T) {
	templates := scaffold.GetBuiltinTemplates()
	if len(templates) != 3 {
		t.Fatalf("expected 3 built-in templates, got %d", len(templates))
	}

	expectedIDs := map[string]bool{
		scaffold.BuiltinIDGoAPI:      true,
		scaffold.BuiltinIDNodeWeb:    true,
		scaffold.BuiltinIDPostgresDB: true,
	}

	for _, tmpl := range templates {
		if !expectedIDs[tmpl.ID] {
			t.Errorf("unexpected template ID: %s", tmpl.ID)
		}
		if tmpl.Name == "" {
			t.Errorf("template %s has empty Name", tmpl.ID)
		}
		if tmpl.Description == "" {
			t.Errorf("template %s has empty Description", tmpl.ID)
		}
		if tmpl.Category == "" {
			t.Errorf("template %s has empty Category", tmpl.ID)
		}
		if tmpl.Framework == "" {
			t.Errorf("template %s has empty Framework", tmpl.ID)
		}
		if !tmpl.BuiltIn {
			t.Errorf("template %s should have BuiltIn=true", tmpl.ID)
		}
		if tmpl.ManifestYAML == "" {
			t.Errorf("template %s has empty ManifestYAML", tmpl.ID)
		}
		if tmpl.HelmValues == "" {
			t.Errorf("template %s has empty HelmValues", tmpl.ID)
		}
		if tmpl.DockerCompose == "" {
			t.Errorf("template %s has empty DockerCompose", tmpl.ID)
		}
		if len(tmpl.Variables) == 0 {
			t.Errorf("template %s has no Variables defined", tmpl.ID)
		}
		for _, v := range tmpl.Variables {
			if v.Name == "" {
				t.Errorf("template %s has variable with empty name", tmpl.ID)
			}
			if v.Label == "" {
				t.Errorf("template %s variable %s has empty label", tmpl.ID, v.Name)
			}
			if v.Type == "" {
				t.Errorf("template %s variable %s has empty type", tmpl.ID, v.Name)
			}
		}
	}
}
