package postgres_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestSecurity_SQLQueries scans database repository implementation files under internal/infrastructure/postgres
// to detect if SQL queries are constructed dynamically using unsafe Sprintf format or string concatenations.
func TestSecurity_SQLQueries(t *testing.T) {
	root := "."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		fileAST, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Errorf("Failed to parse file %s: %v", path, err)
			return nil
		}

		ast.Inspect(fileAST, func(n ast.Node) bool {
			// Check string concatenations using '+' operator
			if binExpr, ok := n.(*ast.BinaryExpr); ok && binExpr.Op == token.ADD {
				if isUnsafeSQLConcatenation(binExpr.X, binExpr.Y) {
					t.Errorf("Violation in %s: Unsafe SQL string concatenation using '+' operator", path)
				}
			}

			// Check Sprintf format expressions
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" && sel.Sel.Name == "Sprintf" {
						if isUnsafeSQLFormat(call) {
							t.Errorf("Violation in %s: Unsafe dynamic SQL formatting in Sprintf", path)
						}
					}
				}
			}
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk directory: %v", err)
	}
}

// Check if an AST expression is a string literal containing SQL keywords
func isSQLStringLiteral(expr ast.Expr) (string, bool) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		val := strings.ToLower(lit.Value)
		if strings.Contains(val, "select") || strings.Contains(val, "insert") ||
			strings.Contains(val, "update") || strings.Contains(val, "delete") ||
			strings.Contains(val, "where") || strings.Contains(val, "and") ||
			strings.Contains(val, "join") {
			return lit.Value, true
		}
	}
	return "", false
}

func isUnsafeSQLConcatenation(left, right ast.Expr) bool {
	_, leftIsSQL := isSQLStringLiteral(left)
	_, rightIsSQL := isSQLStringLiteral(right)

	if leftIsSQL {
		return !isSafeSQLExpr(right)
	}
	if rightIsSQL {
		return !isSafeSQLExpr(left)
	}
	return false
}

func isSafeSQLExpr(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING || e.Kind == token.INT
	case *ast.Ident:
		name := strings.ToLower(e.Name)
		return name == "query" || name == "basequery" || name == "countquery" ||
			name == "selectquery" || name == "whereclause" || name == "limit" ||
			name == "offset" || name == "idx" || name == "argidx" || name == "qb" ||
			name == "sb" || name == "sql" || name == "err"
	case *ast.BinaryExpr:
		return isSafeSQLExpr(e.X) && isSafeSQLExpr(e.Y)
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" && sel.Sel.Name == "Sprintf" {
				return !isUnsafeSQLFormat(e)
			}
		}
	}
	return false
}

func isUnsafeSQLFormat(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	formatStr, isSQL := isSQLStringLiteral(call.Args[0])
	if !isSQL {
		return false
	}

	for _, arg := range call.Args[1:] {
		if !isSafeSQLExpr(arg) {
			if strings.Contains(formatStr, "%s") || strings.Contains(formatStr, "%v") {
				return true
			}
		}
	}
	return false
}
