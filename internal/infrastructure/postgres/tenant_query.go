package postgres

import (
	"context"
	"fmt"
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
	"agent_tasks":           true, // TODO: needs migration to add tenant_id
	"agent_subtasks":        true, // TODO: needs migration to add tenant_id
	"agent_executions":      true, // TODO: needs migration to add tenant_id
	"agent_project_state":   true, // TODO: needs migration to add tenant_id
	"audit_findings":        true, // TODO: needs migration to add tenant_id
	"audit_runs":            true, // TODO: needs migration to add tenant_id
	"audit_logs":            true, // TODO: needs migration to add tenant_id
	"automation_rules":      true, // TODO: needs migration to add tenant_id
	"automation_executions": true, // TODO: needs migration to add tenant_id
	"backup_history":        true, // TODO: needs migration to add tenant_id
	"change_requests":       true, // TODO: needs migration to add tenant_id
	"maintenance_windows":   true, // TODO: needs migration to add tenant_id
	"compliance_frameworks": true, // TODO: needs migration to add tenant_id
	"compliance_violations": true, // TODO: needs migration to add tenant_id
	"correlated_events":     true, // TODO: needs migration to add tenant_id
	"cluster_costs":         true, // TODO: needs migration to add tenant_id
	"namespace_costs":       true, // TODO: needs migration to add tenant_id
	"resource_waste":        true, // TODO: needs migration to add tenant_id
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

type queryRewriter struct {
	tenantID        string
	args            []interface{}
	cteAliases      map[string]bool
	subqueryAliases map[string]bool
}

func newQueryRewriter(tenantID string, args []interface{}) *queryRewriter {
	return &queryRewriter{
		tenantID:        tenantID,
		args:            args,
		cteAliases:      make(map[string]bool),
		subqueryAliases: make(map[string]bool),
	}
}

func (r *queryRewriter) rewrite(tokens []Token) []Token {
	if len(tokens) == 0 {
		return tokens
	}

	// Step 1: Handle WITH (CTEs)
	ctePrefix, mainQueryTokens := r.handleCTEs(tokens)

	// Step 2: Handle top-level UNION
	rewrittenMain := r.handleUnions(mainQueryTokens)

	if len(ctePrefix) == 0 {
		return rewrittenMain
	}

	var result []Token
	result = append(result, ctePrefix...)
	result = append(result, rewrittenMain...)
	return result
}

func (r *queryRewriter) handleCTEs(tokens []Token) ([]Token, []Token) {
	idx := firstNonWhitespaceToken(tokens, 0)
	if idx >= len(tokens) || tokens[idx].Type != TokenKeyword || strings.ToUpper(tokens[idx].Value) != "WITH" {
		return nil, tokens
	}

	mainStart := len(tokens)
	i := idx + 1
	for i < len(tokens) {
		i = firstNonWhitespaceToken(tokens, i)
		if i >= len(tokens) {
			break
		}

		if tokens[i].Type == TokenKeyword && isMainQueryStart(tokens[i].Value) {
			mainStart = i
			break
		}

		if tokens[i].Type == TokenIdentifier || (tokens[i].Type == TokenKeyword && !isMainQueryStart(tokens[i].Value)) {
			cteName := strings.ToLower(tokens[i].Value)
			r.cteAliases[cteName] = true
			i++

			i = firstNonWhitespaceToken(tokens, i)
			if i < len(tokens) && tokens[i].Type == TokenKeyword && strings.ToUpper(tokens[i].Value) == "AS" {
				i++
				i = firstNonWhitespaceToken(tokens, i)
				if i < len(tokens) && tokens[i].Value == "(" {
					closeIdx := findMatchingParen(tokens, i)
					if closeIdx > i {
						innerTokens := tokens[i+1 : closeIdx]
						rewrittenInner := r.rewrite(innerTokens)

						var newTokens []Token
						newTokens = append(newTokens, tokens[:i+1]...)
						newTokens = append(newTokens, rewrittenInner...)
						newTokens = append(newTokens, tokens[closeIdx:]...)
						tokens = newTokens

						diff := len(rewrittenInner) - len(innerTokens)
						i = closeIdx + diff + 1
					}
				}
			}
		}

		i = firstNonWhitespaceToken(tokens, i)
		if i < len(tokens) && tokens[i].Value == "," {
			i++
			continue
		}
		if i < len(tokens) && tokens[i].Type == TokenKeyword && isMainQueryStart(tokens[i].Value) {
			mainStart = i
			break
		}
		i++
	}

	if mainStart < len(tokens) {
		return tokens[:mainStart], tokens[mainStart:]
	}

	return tokens, nil
}

func isMainQueryStart(val string) bool {
	u := strings.ToUpper(val)
	return u == "SELECT" || u == "INSERT" || u == "UPDATE" || u == "DELETE"
}

func (r *queryRewriter) handleUnions(tokens []Token) []Token {
	depth := 0
	unionIndices := []int{}

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Value == "(" {
			depth++
		} else if tok.Value == ")" {
			if depth > 0 {
				depth--
			}
		} else if depth == 0 && tok.Type == TokenKeyword && strings.ToUpper(tok.Value) == "UNION" {
			unionIndices = append(unionIndices, i)
		}
	}

	if len(unionIndices) == 0 {
		return r.rewriteSingleSelect(tokens)
	}

	var result []Token
	start := 0

	for _, uIdx := range unionIndices {
		segment := tokens[start:uIdx]
		rewrittenSegment := r.rewriteSingleSelect(segment)
		result = append(result, rewrittenSegment...)

		endUnion := uIdx + 1
		if endUnion < len(tokens) && tokens[endUnion].Type == TokenWhitespace {
			endUnion++
		}
		if endUnion < len(tokens) && tokens[endUnion].Type == TokenKeyword && strings.ToUpper(tokens[endUnion].Value) == "ALL" {
			endUnion++
		}
		result = append(result, tokens[uIdx:endUnion]...)
		start = endUnion
	}

	if start < len(tokens) {
		lastSegment := tokens[start:]
		rewrittenLast := r.rewriteSingleSelect(lastSegment)
		result = append(result, rewrittenLast...)
	}

	return result
}

type tableRef struct {
	tableName  string
	tableAlias string
}

func (r *queryRewriter) rewriteSingleSelect(tokens []Token) []Token {
	if len(tokens) == 0 {
		return tokens
	}

	tokens = r.rewriteSubqueries(tokens)

	tenantTables := r.findAllTenantTables(tokens)
	if len(tenantTables) == 0 {
		return tokens
	}

	qualifiers := r.buildTenantQualifiers(tenantTables)

	hasWhere, whereIdx := r.findTopLevelWhere(tokens)
	insertIdx := r.findFilterInsertionPoint(tokens, hasWhere, whereIdx)

	var filterTokens []Token
	needLeadingSpace := true
	if insertIdx > 0 && tokens[insertIdx-1].Type == TokenWhitespace {
		needLeadingSpace = false
	}

	needTrailingSpace := false
	if insertIdx < len(tokens) && tokens[insertIdx].Type != TokenWhitespace {
		needTrailingSpace = true
	}

	if needLeadingSpace {
		filterTokens = append(filterTokens, Token{Type: TokenWhitespace, Value: " "})
	}

	for i, qualifier := range qualifiers {
		r.args = append(r.args, r.tenantID)
		paramIdx := len(r.args)

		if i == 0 && !hasWhere {
			filterTokens = append(filterTokens,
				Token{Type: TokenKeyword, Value: "WHERE"},
				Token{Type: TokenWhitespace, Value: " "},
				Token{Type: TokenIdentifier, Value: qualifier},
				Token{Type: TokenWhitespace, Value: " "},
				Token{Type: TokenOperator, Value: "="},
				Token{Type: TokenWhitespace, Value: " "},
				Token{Type: TokenPlaceholder, Value: fmt.Sprintf("$%d", paramIdx)},
			)
		} else {
			filterTokens = append(filterTokens,
				Token{Type: TokenKeyword, Value: "AND"},
				Token{Type: TokenWhitespace, Value: " "},
				Token{Type: TokenIdentifier, Value: qualifier},
				Token{Type: TokenWhitespace, Value: " "},
				Token{Type: TokenOperator, Value: "="},
				Token{Type: TokenWhitespace, Value: " "},
				Token{Type: TokenPlaceholder, Value: fmt.Sprintf("$%d", paramIdx)},
			)
		}

		if i < len(qualifiers)-1 {
			filterTokens = append(filterTokens, Token{Type: TokenWhitespace, Value: " "})
		}
	}

	if needTrailingSpace {
		filterTokens = append(filterTokens, Token{Type: TokenWhitespace, Value: " "})
	}

	var out []Token
	out = append(out, tokens[:insertIdx]...)
	out = append(out, filterTokens...)
	out = append(out, tokens[insertIdx:]...)
	return out
}

func (r *queryRewriter) rewriteSubqueries(tokens []Token) []Token {
	depth := 0
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Value == "(" {
			depth++
			closeIdx := findMatchingParen(tokens, i)
			if closeIdx > i {
				innerTokens := tokens[i+1 : closeIdx]
				if containsQueryKeyword(innerTokens) {
					rewrittenInner := r.rewrite(innerTokens)

					nextIdx := firstNonWhitespaceToken(tokens, closeIdx+1)
					if nextIdx < len(tokens) {
						if tokens[nextIdx].Type == TokenKeyword && strings.ToUpper(tokens[nextIdx].Value) == "AS" {
							aliasIdx := firstNonWhitespaceToken(tokens, nextIdx+1)
							if aliasIdx < len(tokens) && tokens[aliasIdx].Type == TokenIdentifier {
								r.subqueryAliases[strings.ToLower(tokens[aliasIdx].Value)] = true
							}
						} else if tokens[nextIdx].Type == TokenIdentifier {
							r.subqueryAliases[strings.ToLower(tokens[nextIdx].Value)] = true
						}
					}

					var newTokens []Token
					newTokens = append(newTokens, tokens[:i+1]...)
					newTokens = append(newTokens, rewrittenInner...)
					newTokens = append(newTokens, tokens[closeIdx:]...)
					tokens = newTokens

					diff := len(rewrittenInner) - len(innerTokens)
					i = closeIdx + diff
				}
			}
		} else if tokens[i].Value == ")" {
			if depth > 0 {
				depth--
			}
		}
	}
	return tokens
}

func containsQueryKeyword(tokens []Token) bool {
	for _, tok := range tokens {
		if tok.Type == TokenKeyword && (strings.ToUpper(tok.Value) == "SELECT" || strings.ToUpper(tok.Value) == "WITH") {
			return true
		}
	}
	return false
}

func (r *queryRewriter) findAllTenantTables(tokens []Token) []tableRef {
	var tenantTables []tableRef
	depth := 0
	n := len(tokens)

	for i := 0; i < n; i++ {
		tok := tokens[i]
		if tok.Value == "(" {
			depth++
			continue
		}
		if tok.Value == ")" {
			if depth > 0 {
				depth--
			}
			continue
		}

		if depth != 0 {
			continue
		}

		if tok.Type != TokenKeyword {
			continue
		}

		kw := strings.ToUpper(tok.Value)
		if kw == "FROM" || kw == "JOIN" || kw == "USING" {
			i = r.parseTableReferences(tokens, i+1, &tenantTables)
		} else if kw == "UPDATE" {
			i = r.parseSingleTableReference(tokens, i+1, &tenantTables)
		}
	}

	return r.deduplicateTableRefs(tenantTables)
}

func (r *queryRewriter) parseSingleTableReference(tokens []Token, start int, tenantTables *[]tableRef) int {
	j := firstNonWhitespaceToken(tokens, start)
	if j >= len(tokens) {
		return start
	}

	if tokens[j].Value == "(" {
		return j
	}

	if (tokens[j].Type == TokenIdentifier || tokens[j].Type == TokenKeyword) && !isSQLKeyword(tokens[j].Value) {
		tableName := tokens[j].Value
		tableAlias := ""
		nextIdx := j

		k := firstNonWhitespaceToken(tokens, j+1)
		if k < len(tokens) {
			if tokens[k].Type == TokenKeyword && strings.ToUpper(tokens[k].Value) == "AS" {
				k2 := firstNonWhitespaceToken(tokens, k+1)
				if k2 < len(tokens) && tokens[k2].Type == TokenIdentifier && !isSQLKeyword(tokens[k2].Value) {
					tableAlias = tokens[k2].Value
					nextIdx = k2
				}
			} else if tokens[k].Type == TokenIdentifier && !isSQLKeyword(tokens[k].Value) {
				tableAlias = tokens[k].Value
				nextIdx = k
			}
		}

		if r.isTenantTable(tableName) {
			*tenantTables = append(*tenantTables, tableRef{
				tableName:  tableName,
				tableAlias: tableAlias,
			})
		}
		return nextIdx
	}
	return start
}

func (r *queryRewriter) parseTableReferences(tokens []Token, start int, tenantTables *[]tableRef) int {
	i := start
	for i < len(tokens) {
		i = firstNonWhitespaceToken(tokens, i)
		if i >= len(tokens) {
			break
		}

		if tokens[i].Value == "(" {
			closeParen := findMatchingParen(tokens, i)
			if closeParen > i {
				i = closeParen
				next := firstNonWhitespaceToken(tokens, closeParen+1)
				if next < len(tokens) {
					if tokens[next].Type == TokenKeyword && strings.ToUpper(tokens[next].Value) == "AS" {
						next2 := firstNonWhitespaceToken(tokens, next+1)
						if next2 < len(tokens) && tokens[next2].Type == TokenIdentifier {
							i = next2
						} else {
							i = next
						}
					} else if tokens[next].Type == TokenIdentifier && !isSQLKeyword(tokens[next].Value) {
						i = next
					}
				}
				i++
			} else {
				break
			}
			continue
		}

		if (tokens[i].Type == TokenIdentifier || tokens[i].Type == TokenKeyword) && !isSQLKeyword(tokens[i].Value) {
			tableName := tokens[i].Value
			tableAlias := ""

			k := firstNonWhitespaceToken(tokens, i+1)
			if k < len(tokens) {
				if tokens[k].Type == TokenKeyword && strings.ToUpper(tokens[k].Value) == "AS" {
					k2 := firstNonWhitespaceToken(tokens, k+1)
					if k2 < len(tokens) && tokens[k2].Type == TokenIdentifier && !isSQLKeyword(tokens[k2].Value) {
						tableAlias = tokens[k2].Value
						i = k2
					} else {
						i = k
					}
				} else if tokens[k].Type == TokenIdentifier && !isSQLKeyword(tokens[k].Value) {
					tableAlias = tokens[k].Value
					i = k
				}
			}

			if r.isTenantTable(tableName) {
				*tenantTables = append(*tenantTables, tableRef{
					tableName:  tableName,
					tableAlias: tableAlias,
				})
			}
		}

		next := firstNonWhitespaceToken(tokens, i+1)
		if next < len(tokens) && tokens[next].Value == "," {
			i = next + 1
			continue
		}

		break
	}
	return i
}

func (r *queryRewriter) isTenantTable(tableName string) bool {
	if tableName == "" {
		return false
	}
	lowerT := strings.ToLower(tableName)
	return !r.cteAliases[lowerT] && !r.subqueryAliases[lowerT] && !nonTenantTables[lowerT]
}

func (r *queryRewriter) deduplicateTableRefs(refs []tableRef) []tableRef {
	var result []tableRef
	seen := make(map[string]bool)

	for _, ref := range refs {
		key := strings.ToLower(ref.tableName) + ":" + strings.ToLower(ref.tableAlias)
		if !seen[key] {
			seen[key] = true
			result = append(result, ref)
		}
	}
	return result
}

func (r *queryRewriter) buildTenantQualifiers(tenantTables []tableRef) []string {
	qualifiers := make([]string, len(tenantTables))
	isMultiTable := len(tenantTables) > 1

	for i, ref := range tenantTables {
		if ref.tableAlias != "" {
			qualifiers[i] = ref.tableAlias + ".tenant_id"
		} else if isMultiTable {
			qualifiers[i] = ref.tableName + ".tenant_id"
		} else {
			qualifiers[i] = "tenant_id"
		}
	}
	return qualifiers
}

func (r *queryRewriter) findTopLevelWhere(tokens []Token) (bool, int) {
	depth := 0
	for i, tok := range tokens {
		if tok.Value == "(" {
			depth++
		} else if tok.Value == ")" {
			if depth > 0 {
				depth--
			}
		} else if depth == 0 && tok.Type == TokenKeyword && strings.ToUpper(tok.Value) == "WHERE" {
			return true, i
		}
	}
	return false, -1
}

func (r *queryRewriter) findFilterInsertionPoint(tokens []Token, hasWhere bool, whereIdx int) int {
	depth := 0

	if hasWhere {
		for i := whereIdx + 1; i < len(tokens); i++ {
			tok := tokens[i]
			if tok.Value == "(" {
				depth++
			} else if tok.Value == ")" {
				if depth > 0 {
					depth--
				}
			} else if depth == 0 {
				if tok.Type == TokenComment {
					return i
				}
				if tok.Type == TokenKeyword {
					u := strings.ToUpper(tok.Value)
					if u == "ORDER" || u == "GROUP" || u == "LIMIT" || u == "OFFSET" || u == "FOR" || u == "HAVING" || u == "UNION" || u == "RETURNING" {
						return i
					}
				}
			}
		}
		return len(tokens)
	}

	fromIdx := -1
	for i, tok := range tokens {
		if tok.Value == "(" {
			depth++
		} else if tok.Value == ")" {
			if depth > 0 {
				depth--
			}
		} else if depth == 0 && tok.Type == TokenKeyword {
			u := strings.ToUpper(tok.Value)
			if u == "FROM" || u == "UPDATE" {
				fromIdx = i
				break
			}
		}
	}

	startSearch := 0
	if fromIdx != -1 {
		startSearch = fromIdx + 1
	}

	depth = 0
	for i := startSearch; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Value == "(" {
			depth++
		} else if tok.Value == ")" {
			if depth > 0 {
				depth--
			}
		} else if depth == 0 {
			if tok.Type == TokenComment {
				return i
			}
			if tok.Type == TokenKeyword {
				u := strings.ToUpper(tok.Value)
				if u == "ORDER" || u == "GROUP" || u == "LIMIT" || u == "OFFSET" || u == "FOR" || u == "HAVING" || u == "UNION" || u == "RETURNING" {
					return i
				}
			}
		}
	}

	return len(tokens)
}

func firstNonWhitespaceToken(tokens []Token, start int) int {
	for i := start; i < len(tokens); i++ {
		if tokens[i].Type != TokenWhitespace && tokens[i].Type != TokenComment {
			return i
		}
	}
	return len(tokens)
}

func findMatchingParen(tokens []Token, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(tokens); i++ {
		if tokens[i].Value == "(" {
			depth++
		} else if tokens[i].Value == ")" {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return openIdx
}
