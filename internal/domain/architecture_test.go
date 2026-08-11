package domain_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchitecture_Imports asserts that no domain modules import adapter or infrastructure layers.
func TestArchitecture_Imports(t *testing.T) {
	root := "."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		fileAST, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("Failed to parse file %s: %v", path, err)
			return nil
		}

		for _, imp := range fileAST.Imports {
			val := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(val, "github.com/datdt/k8sselfhost/internal/infrastructure") ||
				strings.Contains(val, "github.com/datdt/k8sselfhost/internal/adapter") ||
				strings.Contains(val, "github.com/datdt/k8sselfhost/internal/usecase") {
				t.Errorf("Violation: domain layer file %s imports disallowed package %s", path, val)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}
}
