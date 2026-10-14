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

var (
	triggerColons = []byte("://")
	triggerBegin  = []byte("BEGIN")
)

// RedactionRule holds a pre-compiled regex and its replacement for stream masking.
type RedactionRule struct {
	Type           SecretPatternType
	Regex          *regexp.Regexp
	Replacement    []byte
	replacementStr string
	triggers       []string
	triggerBytes   [][]byte
}

// InStreamRedactor performs in-stream detection and masking of sensitive credentials.
type InStreamRedactor struct {
	rules      []RedactionRule
	triggerMap map[byte][]int
}

// NewInStreamRedactor constructs an InStreamRedactor with pre-compiled automata rules.
func NewInStreamRedactor() *InStreamRedactor {
	r := &InStreamRedactor{
		rules:      make([]RedactionRule, 0, 8),
		triggerMap: make(map[byte][]int),
	}

	defaultRules := []RedactionRule{
		{
			Type:        PatternConnectionString,
			Regex:       regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis|amqp):\/\/[^:\s]+:[^@\s]+@`),
			Replacement: []byte(`$1://[REDACTED_USER]:[REDACTED_SECRET]@`),
			triggers:    []string{"://"},
		},
		{
			Type:        PatternBearerToken,
			Regex:       regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9_\-\.]{20,}`),
			Replacement: []byte(`$1[REDACTED_BEARER_TOKEN]`),
			triggers:    []string{"bearer"},
		},
		{
			Type:        PatternAPIKey,
			Regex:       regexp.MustCompile(`(?i)(sk_live_|ghp_|glpat-|xoxb-)[a-zA-Z0-9_\-]{16,}`),
			Replacement: []byte(`$1[REDACTED_API_KEY]`),
			triggers:    []string{"sk_live_", "ghp_", "glpat-", "xoxb-"},
		},
		{
			Type:        PatternPasswordKeyValue,
			Regex:       regexp.MustCompile(`(?i)("(?:password|secret|token|api_key|access_key)"\s*:\s*")[^"]+(")`),
			Replacement: []byte(`$1[REDACTED_SECRET]$2`),
			triggers:    []string{"password", "secret", "token", "api_key", "access_key"},
		},
		{
			Type:        PatternPrivateKey,
			Regex:       regexp.MustCompile(`(?s)-----BEGIN [A-Z ]+PRIVATE KEY-----.*?-----END [A-Z ]+PRIVATE KEY-----`),
			Replacement: []byte(`[REDACTED_PRIVATE_KEY]`),
			triggers:    []string{"BEGIN"},
		},
	}

	for _, rule := range defaultRules {
		r.AddRule(rule)
	}

	return r
}

// AddRule adds a new custom redaction rule to the InStreamRedactor with safe case indexing.
func (r *InStreamRedactor) AddRule(rule RedactionRule) {
	if rule.replacementStr == "" {
		rule.replacementStr = string(rule.Replacement)
	}
	if len(rule.triggerBytes) == 0 && len(rule.triggers) > 0 {
		rule.triggerBytes = make([][]byte, len(rule.triggers))
		for j, tr := range rule.triggers {
			rule.triggerBytes[j] = []byte(tr)
		}
	}
	ruleIdx := len(r.rules)
	r.rules = append(r.rules, rule)

	if r.triggerMap == nil {
		r.triggerMap = make(map[byte][]int)
	}
	for _, tr := range rule.triggers {
		if len(tr) == 0 {
			continue
		}
		r.indexTriggerByte(tr[0], ruleIdx)
	}
}

// indexTriggerByte safely maps initial byte variations into triggerMap without arithmetic underflow.
func (r *InStreamRedactor) indexTriggerByte(b byte, ruleIdx int) {
	if r.triggerMap == nil {
		r.triggerMap = make(map[byte][]int)
	}
	if b >= 'a' && b <= 'z' {
		r.triggerMap[b] = append(r.triggerMap[b], ruleIdx)
		upper := b - ('a' - 'A')
		r.triggerMap[upper] = append(r.triggerMap[upper], ruleIdx)
	} else if b >= 'A' && b <= 'Z' {
		r.triggerMap[b] = append(r.triggerMap[b], ruleIdx)
		lower := b + ('a' - 'A')
		r.triggerMap[lower] = append(r.triggerMap[lower], ruleIdx)
	} else {
		// Digits ('0'..'9'), symbols, and non-ASCII bytes (b >= 128) are indexed safely without underflow
		r.triggerMap[b] = append(r.triggerMap[b], ruleIdx)
	}
}

// GetTriggerRules returns the rule indices matching an initial byte, if any.
func (r *InStreamRedactor) GetTriggerRules(b byte) []int {
	if r.triggerMap == nil {
		return nil
	}
	return r.triggerMap[b]
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
			if matchCount < len(matchedIndices) {
				matchedIndices[matchCount] = i
				matchCount++
			}
		}
	}

	if matchCount == 0 {
		return line
	}

	out := line
	for k := 0; k < matchCount; k++ {
		rule := &r.rules[matchedIndices[k]]
		if !rule.Regex.Match(out) {
			continue
		}
		out = rule.Regex.ReplaceAll(out, rule.Replacement)
	}
	return out
}

// MaskString replaces sensitive credentials in string s with redacted placeholders.
// If no triggers are present (fast path), the original string is returned with zero allocations.
func (r *InStreamRedactor) MaskString(s string) string {
	if len(s) == 0 {
		return s
	}

	var matchedIndices [8]int
	matchCount := 0

	for i := range r.rules {
		if ruleTriggeredString(&r.rules[i], s) {
			if matchCount < len(matchedIndices) {
				matchedIndices[matchCount] = i
				matchCount++
			}
		}
	}

	if matchCount == 0 {
		return s
	}

	out := s
	for k := 0; k < matchCount; k++ {
		rule := &r.rules[matchedIndices[k]]
		if !rule.Regex.MatchString(out) {
			continue
		}
		out = rule.Regex.ReplaceAllString(out, rule.replacementStr)
	}
	return out
}

func ruleTriggeredBytes(rule *RedactionRule, b []byte) bool {
	if len(rule.triggers) == 0 {
		return true
	}
	if rule.Type == PatternConnectionString {
		return bytes.Contains(b, triggerColons)
	}
	if rule.Type == PatternPrivateKey {
		return bytes.Contains(b, triggerBegin)
	}

	for _, tr := range rule.triggers {
		if containsFoldBytes(b, tr) {
			return true
		}
	}
	return false
}

func ruleTriggeredString(rule *RedactionRule, s string) bool {
	if len(rule.triggers) == 0 {
		return true
	}
	if rule.Type == PatternConnectionString {
		return strings.Contains(s, "://")
	}
	if rule.Type == PatternPrivateKey {
		return strings.Contains(s, "BEGIN")
	}

	for _, tr := range rule.triggers {
		if containsFoldString(s, tr) {
			return true
		}
	}
	return false
}

// containsFoldBytes checks if sub occurs within s (ASCII case-insensitive) without heap allocation.
// Handles initial byte and subsequent characters safely for uppercase, lowercase, symbols, and non-ASCII bytes.
func containsFoldBytes(s []byte, sub string) bool {
	n := len(sub)
	if n == 0 {
		return true
	}
	if len(s) < n {
		return false
	}

	b0 := sub[0]
	var fLower, fUpper byte
	if b0 >= 'a' && b0 <= 'z' {
		fLower = b0
		fUpper = b0 - ('a' - 'A')
	} else if b0 >= 'A' && b0 <= 'Z' {
		fLower = b0 + ('a' - 'A')
		fUpper = b0
	} else {
		fLower = b0
		fUpper = b0
	}

	max := len(s) - n
	for i := 0; i <= max; i++ {
		c := s[i]
		if c == fLower || c == fUpper {
			match := true
			for j := 1; j < n; j++ {
				sc := s[i+j]
				if sc >= 'A' && sc <= 'Z' {
					sc += 'a' - 'A'
				}
				tc := sub[j]
				if tc >= 'A' && tc <= 'Z' {
					tc += 'a' - 'A'
				}
				if sc != tc {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// containsFoldString checks if sub occurs within s (ASCII case-insensitive) without heap allocation.
// Handles initial byte and subsequent characters safely for uppercase, lowercase, symbols, and non-ASCII bytes.
func containsFoldString(s string, sub string) bool {
	n := len(sub)
	if n == 0 {
		return true
	}
	if len(s) < n {
		return false
	}

	b0 := sub[0]
	var fLower, fUpper byte
	if b0 >= 'a' && b0 <= 'z' {
		fLower = b0
		fUpper = b0 - ('a' - 'A')
	} else if b0 >= 'A' && b0 <= 'Z' {
		fLower = b0 + ('a' - 'A')
		fUpper = b0
	} else {
		fLower = b0
		fUpper = b0
	}

	max := len(s) - n
	for i := 0; i <= max; i++ {
		c := s[i]
		if c == fLower || c == fUpper {
			match := true
			for j := 1; j < n; j++ {
				sc := s[i+j]
				if sc >= 'A' && sc <= 'Z' {
					sc += 'a' - 'A'
				}
				tc := sub[j]
				if tc >= 'A' && tc <= 'Z' {
					tc += 'a' - 'A'
				}
				if sc != tc {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

