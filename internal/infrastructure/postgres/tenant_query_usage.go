package postgres

import (
	"fmt"
	"strings"
)


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
