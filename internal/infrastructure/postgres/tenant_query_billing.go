package postgres

import (
	"strings"
)


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
