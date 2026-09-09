package postgres

import (
	"strings"
)


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
