package logengine

import (
	"bytes"
	"regexp"
	"strings"
)

// SecretPatternType identifies the category of sensitive credential pattern.
type SecretPatternType string

const (
	PatternConnectionString SecretPatternType = "CONNECTION_STRING"
	PatternBearerToken      SecretPatternType = "BEARER_TOKEN"
	PatternAPIKey           SecretPatternType = "API_KEY"
	PatternPasswordKeyValue SecretPatternType = "PASSWORD_KEY_VALUE"
	PatternPrivateKey       SecretPatternType = "PRIVATE_KEY"
)

// RedactionRule holds a pre-compiled regex and its replacement for stream masking.
type RedactionRule struct {
	Type           SecretPatternType
	Regex          *regexp.Regexp
	Replacement    []byte
	replacementStr string
	triggers       []string
}

// InStreamRedactor performs in-stream detection and masking of sensitive credentials.
type InStreamRedactor struct {
	rules []RedactionRule
}

// NewInStreamRedactor constructs an InStreamRedactor with pre-compiled automata rules.
func NewInStreamRedactor() *InStreamRedactor {
	rules := []RedactionRule{
		{Type: PatternConnectionString, Regex: regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis|amqp):\/\/[^:\s]+:[^@\s]+@`), Replacement: []byte(`$1://[REDACTED_USER]:[REDACTED_SECRET]@`), triggers: []string{"postgres://", "mysql://", "mongodb://", "redis://", "amqp://"}},
		{Type: PatternBearerToken, Regex: regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9_\-\.]{20,}`), Replacement: []byte(`$1[REDACTED_BEARER_TOKEN]`), triggers: []string{"bearer"}},
		{Type: PatternAPIKey, Regex: regexp.MustCompile(`(?i)(sk_live_|ghp_|glpat-|xoxb-)[a-zA-Z0-9_\-]{16,}`), Replacement: []byte(`$1[REDACTED_API_KEY]`), triggers: []string{"sk_live_", "ghp_", "glpat-", "xoxb-"}},
		{Type: PatternPasswordKeyValue, Regex: regexp.MustCompile(`(?i)("(?:password|secret|token|api_key|access_key)"\s*:\s*")[^"]+(")`), Replacement: []byte(`$1[REDACTED_SECRET]$2`), triggers: []string{"\"password\"", "\"secret\"", "\"token\"", "\"api_key\"", "\"access_key\""}},
		{Type: PatternPrivateKey, Regex: regexp.MustCompile(`(?s)-----BEGIN [A-Z ]+PRIVATE KEY-----.*?-----END [A-Z ]+PRIVATE KEY-----`), Replacement: []byte(`[REDACTED_PRIVATE_KEY]`), triggers: []string{"BEGIN"}},
	}

	for i := range rules {
		rules[i].replacementStr = string(rules[i].Replacement)
	}

	return &InStreamRedactor{rules: rules}
}

// MaskBytes replaces sensitive credentials in line []byte with redacted placeholders.
// If no triggers are present (fast path), the original slice is returned with zero allocations.
func (r *InStreamRedactor) MaskBytes(line []byte) []byte {
	if len(line) == 0 {
		return line
	}

	var matchedIndices [8]int
	matchCount := 0

	for i := range r.rules {
		if ruleTriggeredBytes(&r.rules[i], line) {
			matchedIndices[matchCount] = i
			matchCount++
		}
	}

	if matchCount == 0 {
		return line
	}

	out := line
	for k := 0; k < matchCount; k++ {
		rule := &r.rules[matchedIndices[k]]
		var handled bool
		switch rule.Type {
		case PatternConnectionString:
			out, handled = maskConnectionStringBytes(out)
		case PatternPasswordKeyValue:
			out, handled = maskPasswordKeyValueBytes(out)
		case PatternBearerToken:
			out, handled = maskBearerTokenBytes(out)
		case PatternAPIKey:
			out, handled = maskAPIKeyBytes(out)
		case PatternPrivateKey:
			out, handled = maskPrivateKeyBytes(out)
		}
		if handled {
			continue
		}
		if rule.Regex.Match(out) {
			out = rule.Regex.ReplaceAll(out, rule.Replacement)
		}
	}
	return out
}

// MaskString replaces sensitive credentials in string s with redacted placeholders.
// If no triggers are present (fast path), the original string is returned with zero allocations.
func (r *InStreamRedactor) MaskString(s string) string {
	if len(s) == 0 {
		return s
	}
	for i := range r.rules {
		if ruleTriggeredString(&r.rules[i], s) {
			return string(r.MaskBytes([]byte(s)))
		}
	}
	return s
}

var (
	connectionSchemes = []string{"postgres://", "mysql://", "mongodb://", "redis://", "amqp://"}
	connRedacted      = []byte("[REDACTED_USER]:[REDACTED_SECRET]@")
	passwordKeys      = []string{"\"password\"", "\"secret\"", "\"token\"", "\"api_key\"", "\"access_key\""}
	passRedacted      = []byte("[REDACTED_SECRET]")
	bearerRedacted    = []byte("[REDACTED_BEARER_TOKEN]")
	apiKeyPrefixes    = []string{"sk_live_", "ghp_", "glpat-", "xoxb-"}
	apiKeyRedacted    = []byte("[REDACTED_API_KEY]")
	beginPem          = []byte("-----BEGIN ")
	endPem            = []byte("-----END ")
	keySuffix         = []byte("PRIVATE KEY-----")
	privKeyRedacted   = []byte("[REDACTED_PRIVATE_KEY]")
	triggerColons     = []byte("://")
	triggerBegin      = []byte("BEGIN")
)

func maskConnectionStringBytes(b []byte) ([]byte, bool) {
	if !bytes.Contains(b, triggerColons) {
		return b, false
	}
	modified := false
	out := b
	offset := 0
	for {
		foundScheme := ""
		schemePos := -1
		for _, s := range connectionSchemes {
			idx := indexFoldBytes(out[offset:], s)
			if idx != -1 && (schemePos == -1 || idx < schemePos) {
				schemePos = idx
				foundScheme = s
			}
		}
		if schemePos == -1 {
			break
		}
		credStart := offset + schemePos + len(foundScheme)
		atRel := -1
		for i := credStart; i < len(out); i++ {
			c := out[i]
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '/' || c == '"' { break }
			if c == '@' { atRel = i; break }
		}
		if atRel == -1 {
			offset = credStart
			continue
		}
		colonIdx := bytes.IndexByte(out[credStart:atRel], ':')
		if colonIdx <= 0 || colonIdx >= (atRel-credStart-1) {
			offset = atRel + 1
			continue
		}

		out = splice(out, credStart, atRel+1, connRedacted)
		modified = true
		offset = credStart + len(connRedacted)
	}
	return out, modified
}

func maskPasswordKeyValueBytes(b []byte) ([]byte, bool) {
	if bytes.IndexByte(b, '"') == -1 || bytes.IndexByte(b, ':') == -1 {
		return b, false
	}
	modified := false
	out := b
	offset := 0
	for {
		keyPos, keyLen := -1, 0
		for _, k := range passwordKeys {
			idx := indexFoldBytes(out[offset:], k)
			if idx != -1 && (keyPos == -1 || idx < keyPos) {
				keyPos, keyLen = idx, len(k)
			}
		}
		if keyPos == -1 {
			break
		}
		p := offset + keyPos + keyLen
		for p < len(out) && (out[p] == ' ' || out[p] == '\t') { p++ }
		if p >= len(out) || out[p] != ':' {
			offset = offset + keyPos + keyLen
			continue
		}
		p++
		for p < len(out) && (out[p] == ' ' || out[p] == '\t') { p++ }
		if p >= len(out) || out[p] != '"' {
			offset = p
			continue
		}
		valStart := p + 1
		q := bytes.IndexByte(out[valStart:], '"')
		if q <= 0 {
			offset = valStart
			continue
		}
		valEnd := valStart + q
		out = splice(out, valStart, valEnd, passRedacted)
		modified = true
		offset = valStart + len(passRedacted) + 1
	}
	return out, modified
}

func maskBearerTokenBytes(b []byte) ([]byte, bool) {
	modified := false
	out := b
	offset := 0
	for {
		idx := indexFoldBytes(out[offset:], "bearer")
		if idx == -1 {
			break
		}
		pos := offset + idx
		if pos > 0 && isWordChar(out[pos-1]) {
			offset = pos + 6
			continue
		}
		p := pos + 6
		spaceCount := 0
		for p < len(out) && (out[p] == ' ' || out[p] == '\t') { spaceCount++; p++ }
		if spaceCount == 0 || p >= len(out) {
			offset = p
			continue
		}
		tokStart := p
		for p < len(out) && isTokenChar(out[p]) { p++ }
		if (p - tokStart) < 20 {
			offset = p
			continue
		}
		out = splice(out, tokStart, p, bearerRedacted)
		modified = true
		offset = tokStart + len(bearerRedacted)
	}
	return out, modified
}

func maskAPIKeyBytes(b []byte) ([]byte, bool) {
	modified := false
	out := b
	offset := 0
	for {
		prefixPos, prefixLen := -1, 0
		for _, pfx := range apiKeyPrefixes {
			idx := indexFoldBytes(out[offset:], pfx)
			if idx != -1 && (prefixPos == -1 || idx < prefixPos) {
				prefixPos, prefixLen = idx, len(pfx)
			}
		}
		if prefixPos == -1 {
			break
		}
		tokStart := offset + prefixPos + prefixLen
		p := tokStart
		for p < len(out) && isKeyChar(out[p]) { p++ }
		if (p - tokStart) < 16 {
			offset = p
			continue
		}
		out = splice(out, tokStart, p, apiKeyRedacted)
		modified = true
		offset = tokStart + len(apiKeyRedacted)
	}
	return out, modified
}

func maskPrivateKeyBytes(b []byte) ([]byte, bool) {
	modified := false
	out := b
	offset := 0
	for {
		relBegin := bytes.Index(out[offset:], beginPem)
		if relBegin == -1 {
			break
		}
		beginIdx := offset + relBegin
		relEnd := bytes.Index(out[beginIdx:], endPem)
		if relEnd == -1 {
			break
		}
		endStart := beginIdx + relEnd + len(endPem)
		suffixIdx := bytes.Index(out[endStart:], keySuffix)
		if suffixIdx == -1 {
			offset = endStart
			continue
		}
		pemEnd := endStart + suffixIdx + len(keySuffix)
		out = splice(out, beginIdx, pemEnd, privKeyRedacted)
		modified = true
		offset = beginIdx + len(privKeyRedacted)
	}
	return out, modified
}

func splice(b []byte, start, end int, rep []byte) []byte {
	res := make([]byte, 0, len(b)-(end-start)+len(rep))
	res = append(res, b[:start]...)
	res = append(res, rep...)
	return append(res, b[end:]...)
}

func isWordChar(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' }
func isTokenChar(c byte) bool { return isWordChar(c) || c == '-' || c == '.' }
func isKeyChar(c byte) bool   { return isWordChar(c) || c == '-' }

func hasAPIKeyTrigger(b []byte) bool {
	offset := 0
	for {
		i1, i2 := bytes.IndexByte(b[offset:], '_'), bytes.IndexByte(b[offset:], '-')
		if i1 == -1 && i2 == -1 {
			return false
		}
		cand := i1
		if i1 == -1 || (i2 != -1 && i2 < i1) {
			cand = i2
		}
		pos := offset + cand
		if b[pos] == '_' {
			if (pos >= 7 && equalFoldBytes(b[pos-7:pos], "sk_live")) || (pos >= 3 && equalFoldBytes(b[pos-3:pos], "ghp")) {
				return true
			}
		} else {
			if (pos >= 5 && equalFoldBytes(b[pos-5:pos], "glpat")) || (pos >= 4 && equalFoldBytes(b[pos-4:pos], "xoxb")) {
				return true
			}
		}
		offset = pos + 1
	}
}

func equalFoldBytes(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range b {
		c := b[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != s[i] {
			return false
		}
	}
	return true
}

func hasAPIKeyTriggerString(s string) bool {
	offset := 0
	for {
		i1, i2 := strings.IndexByte(s[offset:], '_'), strings.IndexByte(s[offset:], '-')
		if i1 == -1 && i2 == -1 {
			return false
		}
		cand := i1
		if i1 == -1 || (i2 != -1 && i2 < i1) {
			cand = i2
		}
		pos := offset + cand
		if s[pos] == '_' {
			if (pos >= 7 && strings.EqualFold(s[pos-7:pos], "sk_live")) || (pos >= 3 && strings.EqualFold(s[pos-3:pos], "ghp")) {
				return true
			}
		} else {
			if (pos >= 5 && strings.EqualFold(s[pos-5:pos], "glpat")) || (pos >= 4 && strings.EqualFold(s[pos-4:pos], "xoxb")) {
				return true
			}
		}
		offset = pos + 1
	}
}

func indexFoldBytes(s []byte, sub string) int {
	n := len(sub)
	if n == 0 { return 0 }
	if len(s) < n { return -1 }
	f0L, f0U := sub[0], sub[0]
	if f0L >= 'a' && f0L <= 'z' { f0U -= 'a' - 'A' }
	if bytes.IndexByte(s, f0L) == -1 && (f0L == f0U || bytes.IndexByte(s, f0U) == -1) { return -1 }
	offset, limit := 0, len(s)-n
	for offset <= limit {
		idx1, idx2 := bytes.IndexByte(s[offset:], f0L), -1
		if f0L != f0U { idx2 = bytes.IndexByte(s[offset:], f0U) }
		cand := idx1
		if idx1 == -1 || (idx2 != -1 && idx2 < idx1) { cand = idx2 }
		if cand == -1 || offset+cand > limit { return -1 }
		pos, match := offset+cand, true
		for j := 1; j < n; j++ {
			sc := s[pos+j]
			if sc >= 'A' && sc <= 'Z' { sc += 'a' - 'A' }
			if sc != sub[j] { match = false; break }
		}
		if match { return pos }
		offset = pos + 1
	}
	return -1
}

func ruleTriggeredBytes(rule *RedactionRule, b []byte) bool {
	switch rule.Type {
	case PatternConnectionString:
		return bytes.Contains(b, triggerColons)
	case PatternPrivateKey:
		return bytes.Contains(b, triggerBegin)
	case PatternPasswordKeyValue:
		if bytes.IndexByte(b, '"') == -1 || bytes.IndexByte(b, ':') == -1 {
			return false
		}
	case PatternBearerToken:
		return containsFoldBytes(b, "bearer")
	case PatternAPIKey:
		return hasAPIKeyTrigger(b)
	}
	for _, tr := range rule.triggers {
		if containsFoldBytes(b, tr) {
			return true
		}
	}
	return false
}

func ruleTriggeredString(rule *RedactionRule, s string) bool {
	switch rule.Type {
	case PatternConnectionString:
		return strings.Contains(s, "://")
	case PatternPrivateKey:
		return strings.Contains(s, "BEGIN")
	case PatternPasswordKeyValue:
		if strings.IndexByte(s, '"') == -1 || strings.IndexByte(s, ':') == -1 {
			return false
		}
	case PatternBearerToken:
		return containsFoldString(s, "bearer")
	case PatternAPIKey:
		return hasAPIKeyTriggerString(s)
	}
	for _, tr := range rule.triggers {
		if containsFoldString(s, tr) {
			return true
		}
	}
	return false
}

func containsFoldBytes(s []byte, sub string) bool { return indexFoldBytes(s, sub) != -1 }

// containsFoldString checks if sub occurs within s (ASCII case-insensitive) without heap allocation.
func containsFoldString(s string, sub string) bool {
	n := len(sub)
	if n == 0 { return true }
	if len(s) < n { return false }
	f0L, f0U := sub[0], sub[0]
	if f0L >= 'a' && f0L <= 'z' { f0U -= 'a' - 'A' }
	if strings.IndexByte(s, f0L) == -1 && (f0L == f0U || strings.IndexByte(s, f0U) == -1) { return false }
	offset, limit := 0, len(s)-n
	for offset <= limit {
		idx1, idx2 := strings.IndexByte(s[offset:], f0L), -1
		if f0L != f0U { idx2 = strings.IndexByte(s[offset:], f0U) }
		cand := idx1
		if idx1 == -1 || (idx2 != -1 && idx2 < idx1) { cand = idx2 }
		if cand == -1 || offset+cand > limit { return false }
		pos, match := offset+cand, true
		for j := 1; j < n; j++ {
			sc := s[pos+j]
			if sc >= 'A' && sc <= 'Z' { sc += 'a' - 'A' }
			if sc != sub[j] { match = false; break }
		}
		if match { return true }
		offset = pos + 1
	}
	return false
}
