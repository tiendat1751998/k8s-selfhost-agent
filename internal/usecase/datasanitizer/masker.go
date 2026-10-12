package datasanitizer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/domain/datasanitizer"
)

var defaultStagingBcryptHash string

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("StagingDefaultPass123!"), bcrypt.DefaultCost)
	if err == nil {
		defaultStagingBcryptHash = string(h)
	} else {
		defaultStagingBcryptHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	}
}

// SQLMasker handles parsing and transforming SQL dump streams.
type SQLMasker struct {
	policy datasanitizer.SanitizationPolicy
}

// NewSQLMasker creates a new SQLMasker with the given policy.
func NewSQLMasker(policy datasanitizer.SanitizationPolicy) *SQLMasker {
	return &SQLMasker{policy: policy}
}

// Policy returns the active sanitization policy.
func (m *SQLMasker) Policy() datasanitizer.SanitizationPolicy {
	return m.policy
}

// MaskValue masks an individual raw string value according to rule specification.
func (m *SQLMasker) MaskValue(rule datasanitizer.MaskingColumnRule, rawVal string) string {
	hint := strings.ToLower(rule.FormatHint)
	col := strings.ToLower(rule.ColumnName)

	if hint == "email" || (hint == "" && strings.Contains(col, "email")) {
		return m.maskEmail(rawVal, rule.Salt)
	}

	if hint == "phone" || (hint == "" && (strings.Contains(col, "phone") || strings.Contains(col, "mobile") || strings.Contains(col, "tel"))) {
		return m.maskPhone(rawVal, rule.Salt)
	}

	if hint == "password" || strings.Contains(col, "password") || strings.Contains(col, "passwd") {
		return defaultStagingBcryptHash
	}

	if hint == "api_key" || hint == "token" || strings.Contains(col, "api_key") || strings.Contains(col, "token") {
		return "sk_test_staging_placeholder"
	}

	if hint == "credit_card" || strings.Contains(col, "card") || strings.Contains(col, "cc_number") {
		return "4111-XXXX-XXXX-1111"
	}

	if hint == "cccd" || strings.Contains(col, "cccd") || strings.Contains(col, "national_id") {
		return "0010XXXXXXXX"
	}

	switch rule.RuleType {
	case datasanitizer.RuleTypeSaltedHash:
		h := sha256.Sum256([]byte(rawVal + rule.Salt))
		return hex.EncodeToString(h[:16])
	case datasanitizer.RuleTypeFixedPlaceholder:
		if rule.DefaultValue != "" {
			return rule.DefaultValue
		}
		return "staging_placeholder"
	case datasanitizer.RuleTypeRedact:
		if rule.DefaultValue != "" {
			return rule.DefaultValue
		}
		return "[REDACTED]"
	default:
		return "staging_masked"
	}
}

func (m *SQLMasker) maskEmail(rawVal, salt string) string {
	h := sha256.Sum256([]byte(rawVal + salt))
	hexStr := hex.EncodeToString(h[:])
	return hexStr[:10] + "@staging.internal"
}

func (m *SQLMasker) maskPhone(rawVal, salt string) string {
	h := sha256.Sum256([]byte(rawVal + salt))
	num := binary.BigEndian.Uint64(h[:8]) % 100000000
	return fmt.Sprintf("09%08d", num)
}

// TransformStream streams SQL statements from reader to writer, transforming sensitive columns.
func (m *SQLMasker) TransformStream(r io.Reader, w io.Writer) (int64, error) {
	return m.TransformStreamWithContext(context.Background(), r, w)
}

// TransformStreamWithContext streams SQL statements with context cancellation checks.
func (m *SQLMasker) TransformStreamWithContext(ctx context.Context, r io.Reader, w io.Writer) (int64, error) {
	reader := bufio.NewReaderSize(r, 64*1024)
	writer := bufio.NewWriterSize(w, 64*1024)
	defer writer.Flush()

	var totalRowsMasked int64

	for {
		if err := ctx.Err(); err != nil {
			return totalRowsMasked, err
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			maskedLine, rows := m.transformLine(line)
			totalRowsMasked += rows
			if _, writeErr := writer.WriteString(maskedLine); writeErr != nil {
				return totalRowsMasked, writeErr
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return totalRowsMasked, err
		}
	}

	return totalRowsMasked, nil
}

func (m *SQLMasker) transformLine(line string) (string, int64) {
	trimmed := strings.TrimSpace(line)
	upper := strings.ToUpper(trimmed)

	if strings.HasPrefix(upper, "INSERT INTO ") {
		return m.transformInsert(line)
	}

	if strings.HasPrefix(upper, "UPDATE ") {
		return m.transformUpdate(line)
	}

	return line, 0
}

func (m *SQLMasker) transformInsert(line string) (string, int64) {
	upper := strings.ToUpper(line)
	valuesIdx := -1
	for _, cand := range []string{" VALUES", " VALUES(", "\tVALUES", "\nVALUES"} {
		if idx := strings.Index(upper, cand); idx != -1 {
			if valuesIdx == -1 || idx < valuesIdx {
				valuesIdx = idx
			}
		}
	}
	if valuesIdx == -1 {
		return line, 0
	}

	preValues := line[:valuesIdx]
	postValues := line[valuesIdx:]

	firstParen := strings.Index(preValues, "(")
	lastParen := strings.LastIndex(preValues, ")")
	if firstParen == -1 || lastParen == -1 || firstParen >= lastParen {
		return line, 0
	}

	intoKeyword := "INTO "
	intoIdx := strings.Index(strings.ToUpper(preValues), intoKeyword)
	if intoIdx == -1 {
		return line, 0
	}

	tablePart := strings.TrimSpace(preValues[intoIdx+len(intoKeyword) : firstParen])
	tableName := cleanIdentifier(tablePart)

	colsStr := preValues[firstParen+1 : lastParen]
	cols := splitIdentifiers(colsStr)

	// Check which column indices need masking
	ruleMap := make(map[int]datasanitizer.MaskingColumnRule)
	for i, col := range cols {
		if rule, found := m.policy.FindRule(tableName, col); found {
			ruleMap[i] = *rule
		}
	}

	if len(ruleMap) == 0 {
		return line, 0
	}

	// Parse and transform tuples in postValues (e.g. " VALUES (1, '...'), (2, '...')")
	newPostValues, count := m.transformValuesClause(postValues, ruleMap)
	return preValues + newPostValues, count
}

func (m *SQLMasker) transformValuesClause(clause string, ruleMap map[int]datasanitizer.MaskingColumnRule) (string, int64) {
	var sb strings.Builder
	var rowsMasked int64

	inQuotes := false
	quoteChar := byte(0)
	inTuple := false
	var tupleBuf strings.Builder

	for i := 0; i < len(clause); i++ {
		ch := clause[i]

		if inQuotes {
			tupleBuf.WriteByte(ch)
			if ch == quoteChar {
				// Check for escaped quote ''
				if i+1 < len(clause) && clause[i+1] == quoteChar {
					tupleBuf.WriteByte(clause[i+1])
					i++
				} else {
					inQuotes = false
				}
			}
			continue
		}

		if ch == '\'' || ch == '"' {
			inQuotes = true
			quoteChar = ch
			if inTuple {
				tupleBuf.WriteByte(ch)
			} else {
				sb.WriteByte(ch)
			}
			continue
		}

		if !inTuple && ch == '(' {
			inTuple = true
			tupleBuf.Reset()
			tupleBuf.WriteByte(ch)
			continue
		}

		if inTuple {
			tupleBuf.WriteByte(ch)
			if ch == ')' {
				inTuple = false
				transformedTuple, masked := m.transformTuple(tupleBuf.String(), ruleMap)
				sb.WriteString(transformedTuple)
				if masked {
					rowsMasked++
				}
			}
			continue
		}

		sb.WriteByte(ch)
	}

	if inTuple {
		sb.WriteString(tupleBuf.String())
	}

	return sb.String(), rowsMasked
}

func (m *SQLMasker) transformTuple(tupleStr string, ruleMap map[int]datasanitizer.MaskingColumnRule) (string, bool) {
	trimmed := strings.TrimSpace(tupleStr)
	if !strings.HasPrefix(trimmed, "(") || !strings.HasSuffix(trimmed, ")") {
		return tupleStr, false
	}

	inner := trimmed[1 : len(trimmed)-1]
	elems := splitSQLExpressions(inner)
	maskedAny := false

	for idx, rule := range ruleMap {
		if idx < len(elems) {
			rawVal := cleanLiteral(elems[idx])
			masked := m.MaskValue(rule, rawVal)
			elems[idx] = "'" + strings.ReplaceAll(masked, "'", "''") + "'"
			maskedAny = true
		}
	}

	return "(" + strings.Join(elems, ", ") + ")", maskedAny
}

func (m *SQLMasker) transformUpdate(line string) (string, int64) {
	trimmed := strings.TrimSpace(line)
	upper := strings.ToUpper(trimmed)

	setIdx := strings.Index(upper, " SET ")
	if setIdx == -1 {
		return line, 0
	}

	tablePart := strings.TrimSpace(trimmed[len("UPDATE "):setIdx])
	tableName := cleanIdentifier(tablePart)

	afterSet := trimmed[setIdx+len(" SET "):]
	whereIdx := strings.Index(strings.ToUpper(afterSet), " WHERE ")
	semiIdx := strings.Index(afterSet, ";")

	endIdx := len(afterSet)
	if whereIdx != -1 {
		endIdx = whereIdx
	} else if semiIdx != -1 {
		endIdx = semiIdx
	}

	assignmentsPart := afterSet[:endIdx]
	remainder := afterSet[endIdx:]

	assignments := splitSQLExpressions(assignmentsPart)
	maskedAny := false

	for i, assign := range assignments {
		eqIdx := strings.Index(assign, "=")
		if eqIdx == -1 {
			continue
		}
		colName := cleanIdentifier(strings.TrimSpace(assign[:eqIdx]))
		valExpr := strings.TrimSpace(assign[eqIdx+1:])

		if rule, found := m.policy.FindRule(tableName, colName); found {
			rawVal := cleanLiteral(valExpr)
			masked := m.MaskValue(*rule, rawVal)
			assignments[i] = assign[:eqIdx] + "= '" + strings.ReplaceAll(masked, "'", "''") + "'"
			maskedAny = true
		}
	}

	if !maskedAny {
		return line, 0
	}

	newLine := trimmed[:setIdx+len(" SET ")] + strings.Join(assignments, ", ") + remainder
	if strings.HasSuffix(line, "\n") {
		newLine += "\n"
	}
	return newLine, 1
}

func splitIdentifiers(s string) []string {
	parts := strings.Split(s, ",")
	res := make([]string, len(parts))
	for i, p := range parts {
		res[i] = cleanIdentifier(p)
	}
	return res
}

func cleanIdentifier(s string) string {
	s = strings.TrimSpace(s)
	// Handle schema prefix: public."users" -> users
	if dotIdx := strings.LastIndex(s, "."); dotIdx != -1 {
		s = s[dotIdx+1:]
	}
	return strings.Trim(s, "\"`' ")
}

func cleanLiteral(s string) string {
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
		if len(s) >= 2 {
			inner := s[1 : len(s)-1]
			return strings.ReplaceAll(inner, "''", "'")
		}
	}
	return s
}

func splitSQLExpressions(s string) []string {
	var items []string
	var cur strings.Builder
	inQuotes := false
	quoteChar := byte(0)
	parenDepth := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inQuotes {
			cur.WriteByte(ch)
			if ch == quoteChar {
				if i+1 < len(s) && s[i+1] == quoteChar {
					cur.WriteByte(s[i+1])
					i++
				} else {
					inQuotes = false
				}
			}
			continue
		}

		if ch == '\'' || ch == '"' {
			inQuotes = true
			quoteChar = ch
			cur.WriteByte(ch)
			continue
		}

		if ch == '(' {
			parenDepth++
			cur.WriteByte(ch)
			continue
		}
		if ch == ')' {
			parenDepth--
			cur.WriteByte(ch)
			continue
		}

		if ch == ',' && parenDepth == 0 {
			items = append(items, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}

		cur.WriteByte(ch)
	}

	if cur.Len() > 0 || len(items) > 0 {
		items = append(items, strings.TrimSpace(cur.String()))
	}

	return items
}
