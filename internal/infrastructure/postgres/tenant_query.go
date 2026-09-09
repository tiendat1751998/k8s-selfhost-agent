package postgres

import (
	"context"
	"strings"
	"unicode"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type TokenType int

const (
	TokenKeyword TokenType = iota
	TokenIdentifier
	TokenStringLiteral
	TokenComment
	TokenNumber
	TokenPlaceholder
	TokenOperator
	TokenPunctuation
	TokenWhitespace
)

type Token struct {
	Type  TokenType
	Value string
}

var nonTenantTables = map[string]bool{
	// Tables without tenant_id column in database schema:
	// TODO: Add migrations to add tenant_id column to these tables for full tenant isolation:
	"backup_history":        true, // TODO: needs migration to add tenant_id
	"change_requests":       true, // TODO: needs migration to add tenant_id
	"maintenance_windows":   true, // TODO: needs migration to add tenant_id
	"compliance_frameworks": true, // TODO: needs migration to add tenant_id
	"compliance_violations": true, // TODO: needs migration to add tenant_id
	"correlated_events":     true, // TODO: needs migration to add tenant_id
	"cluster_costs":         true, // TODO: needs migration to add tenant_id
	"namespace_costs":       true, // TODO: needs migration to add tenant_id
	"resource_waste":        true, // TODO: needs migration to add tenant_id
	"capacity_forecasts":    true, // TODO: needs migration to add tenant_id
	"drift_records":         true, // TODO: needs migration to add tenant_id
	"notifications":        true, // TODO: needs migration to add tenant_id
	"slo_definitions":       true, // TODO: needs migration to add tenant_id
	"slo_snapshots":         true, // TODO: needs migration to add tenant_id
	"promotions":            true, // TODO: needs migration to add tenant_id
	"reports":               true, // TODO: needs migration to add tenant_id
	"reporting":             true, // TODO: needs migration to add tenant_id
	"tags":                  true, // TODO: needs migration to add tenant_id
	"resource_tags":         true, // TODO: needs migration to add tenant_id
	"organizations":         true, // Root tenant table (id is tenant ID)
	"projects":              true, // TODO: scoped by org_id; needs migration or mapping to tenant_id
	"tenant_members":        true, // TODO: scoped by org_id; needs migration or mapping to tenant_id
	"rbac_matrix":           true, // Global RBAC definitions
	"timeline_events":       true, // TODO: needs migration to add tenant_id
	"users":                 true, // Global user accounts
}

// BuildTenantQuery appends tenant filtering to a query if the user is not platform_admin.
func BuildTenantQuery(ctx context.Context, query string, args ...interface{}) (string, []interface{}) {
	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole == "platform_admin" {
		return query, args
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	tokens := tokenizeSQL(query)
	rewriter := newQueryRewriter(tenantID, args)
	rewrittenTokens := rewriter.rewrite(tokens)
	return stringifyTokens(rewrittenTokens), rewriter.args
}

func tokenizeSQL(query string) []Token {
	var tokens []Token
	runes := []rune(query)
	n := len(runes)
	i := 0

	for i < n {
		r := runes[i]

		// 1. Whitespace
		if unicode.IsSpace(r) {
			start := i
			for i < n && unicode.IsSpace(runes[i]) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenWhitespace, Value: string(runes[start:i])})
			continue
		}

		// 2. Single line comment --
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			start := i
			i += 2
			for i < n && runes[i] != '\n' {
				i++
			}
			tokens = append(tokens, Token{Type: TokenComment, Value: string(runes[start:i])})
			continue
		}

		// 3. Block comment /* ... */
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			start := i
			i += 2
			for i < n {
				if runes[i] == '*' && i+1 < n && runes[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokenComment, Value: string(runes[start:i])})
			continue
		}

		// 4. Single-quoted String literal '...'
		if r == '\'' {
			start := i
			i++
			for i < n {
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokenStringLiteral, Value: string(runes[start:i])})
			continue
		}

		// 5. Dollar-quoted String literal $$...$$ or $tag$...$tag$
		if r == '$' && isDollarQuoteStart(runes, i) {
			tag := extractDollarTag(runes, i)
			start := i
			i += len(tag)
			for i < n {
				if runes[i] == '$' && hasPrefixRunes(runes[i:], []rune(tag)) {
					i += len(tag)
					break
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokenStringLiteral, Value: string(runes[start:i])})
			continue
		}

		// 6. Parameter placeholder $1, $2...
		if r == '$' && i+1 < n && unicode.IsDigit(runes[i+1]) {
			start := i
			i++
			for i < n && unicode.IsDigit(runes[i]) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenPlaceholder, Value: string(runes[start:i])})
			continue
		}

		// 7. Quoted Identifier "..." or `...`
		if r == '"' || r == '`' {
			quote := r
			start := i
			i++
			for i < n {
				if runes[i] == quote {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokenIdentifier, Value: string(runes[start:i])})
			continue
		}

		// 8. Unquoted Identifiers or Keywords
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			val := string(runes[start:i])
			if isSQLKeyword(val) {
				tokens = append(tokens, Token{Type: TokenKeyword, Value: val})
			} else {
				tokens = append(tokens, Token{Type: TokenIdentifier, Value: val})
			}
			continue
		}

		// 9. Numbers
		if unicode.IsDigit(r) {
			start := i
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: string(runes[start:i])})
			continue
		}

		// 10. Operators and Punctuation
		if r == '(' || r == ')' || r == ',' || r == ';' || r == '.' {
			tokens = append(tokens, Token{Type: TokenPunctuation, Value: string(r)})
			i++
			continue
		}

		start := i
		i++
		if i < n {
			twoChar := string(runes[start : i+1])
			if twoChar == "!=" || twoChar == "<>" || twoChar == "<=" || twoChar == ">=" || twoChar == "||" {
				i++
				tokens = append(tokens, Token{Type: TokenOperator, Value: twoChar})
				continue
			}
		}
		tokens = append(tokens, Token{Type: TokenOperator, Value: string(runes[start:i])})
	}

	return tokens
}

func isDollarQuoteStart(runes []rune, i int) bool {
	n := len(runes)
	if i+1 < n && unicode.IsDigit(runes[i+1]) {
		return false
	}
	for j := i + 1; j < n; j++ {
		if runes[j] == '$' {
			return true
		}
		if !unicode.IsLetter(runes[j]) && !unicode.IsDigit(runes[j]) && runes[j] != '_' {
			return false
		}
	}
	return false
}

func extractDollarTag(runes []rune, i int) string {
	n := len(runes)
	for j := i + 1; j < n; j++ {
		if runes[j] == '$' {
			return string(runes[i : j+1])
		}
	}
	return "$$"
}

func hasPrefixRunes(runes []rune, prefix []rune) bool {
	if len(runes) < len(prefix) {
		return false
	}
	for i := range prefix {
		if runes[i] != prefix[i] {
			return false
		}
	}
	return true
}

func isSQLKeyword(val string) bool {
	switch strings.ToUpper(val) {
	case "SELECT", "FROM", "WHERE", "UNION", "ALL", "JOIN", "INNER", "LEFT", "RIGHT",
		"FULL", "OUTER", "CROSS", "ON", "WITH", "AS", "ORDER", "BY", "GROUP", "LIMIT",
		"OFFSET", "FOR", "UPDATE", "AND", "OR", "INSERT", "INTO", "VALUES", "SET",
		"DELETE", "HAVING", "USING", "LATERAL", "RETURNING":
		return true
	default:
		return false
	}
}

func stringifyTokens(tokens []Token) string {
	var sb strings.Builder
	for _, tok := range tokens {
		sb.WriteString(tok.Value)
	}
	return sb.String()
}
