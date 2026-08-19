package scaffold

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
)

// Engine renders scaffolding templates using Go text/template syntax.
type Engine interface {
	RenderTemplate(tmpl *scaffold.Template, variables map[string]string) (*scaffold.ScaffoldResult, error)
}

type templateEngine struct{}

// NewEngine creates a new template rendering engine.
func NewEngine() Engine {
	return &templateEngine{}
}

// RenderTemplate validates inputs, populates default variables, and renders YAML, Helm, and Compose targets.
func (e *templateEngine) RenderTemplate(tmpl *scaffold.Template, variables map[string]string) (*scaffold.ScaffoldResult, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("template cannot be nil")
	}

	mergedVars := make(map[string]string)
	// 1. Fill default values from template definitions
	for _, v := range tmpl.Variables {
		if v.Default != "" {
			mergedVars[v.Name] = v.Default
		}
	}

	// 2. Override with caller-provided variables
	for k, v := range variables {
		if strings.TrimSpace(v) != "" {
			mergedVars[k] = v
		}
	}

	// 3. Validate required variables
	for _, v := range tmpl.Variables {
		if v.Required {
			val, exists := mergedVars[v.Name]
			if !exists || strings.TrimSpace(val) == "" {
				return nil, fmt.Errorf("variable %q is required for template %q", v.Name, tmpl.Name)
			}
		}
	}

	// 4. Construct template context with casing aliases (e.g., app_name, AppName, appName, APP_NAME)
	data := make(map[string]interface{})
	for k, v := range mergedVars {
		data[k] = v
		data[toPascalCase(k)] = v
		data[toCamelCase(k)] = v
		data[strings.ToUpper(k)] = v
		data[strings.ToLower(k)] = v
	}

	// 5. Render targets
	renderedYAML, err := renderString("manifest_yaml", tmpl.ManifestYAML, data)
	if err != nil {
		return nil, fmt.Errorf("rendering manifest_yaml: %w", err)
	}

	renderedHelm, err := renderString("helm_values", tmpl.HelmValues, data)
	if err != nil {
		return nil, fmt.Errorf("rendering helm_values: %w", err)
	}

	renderedCompose, err := renderString("docker_compose", tmpl.DockerCompose, data)
	if err != nil {
		return nil, fmt.Errorf("rendering docker_compose: %w", err)
	}

	return &scaffold.ScaffoldResult{
		RenderedYAML:    renderedYAML,
		RenderedCompose: renderedCompose,
		RenderedHelm:    renderedHelm,
	}, nil
}

func renderString(name, content string, data map[string]interface{}) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", nil
	}

	funcMap := template.FuncMap{
		"upper":   strings.ToUpper,
		"lower":   strings.ToLower,
		"trim":    strings.TrimSpace,
		"replace": strings.ReplaceAll,
		"default": func(def, val string) string {
			if strings.TrimSpace(val) == "" {
				return def
			}
			return val
		},
		"indent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return pad + strings.ReplaceAll(v, "\n", "\n"+pad)
		},
		"nindent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return "\n" + pad + strings.ReplaceAll(v, "\n", "\n"+pad)
		},
	}

	t, err := template.New(name).Funcs(funcMap).Parse(content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return ""
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}
