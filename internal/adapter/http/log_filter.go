package http

import (
	"bufio"
	"strings"
)

const (
	baseLogBufferSize = 64 * 1024  // 64KB base buffer
	maxLogBufferSize  = 256 * 1024 // 256KB max line buffer
)

// bytesContainsFold performs an ASCII case-insensitive search for substrLower inside s.
// substrLower MUST be lowercase ASCII.
func bytesContainsFold(s []byte, substrLower string) bool {
	if len(substrLower) == 0 {
		return true
	}
	if len(s) < len(substrLower) {
		return false
	}
	subLen := len(substrLower)
	maxI := len(s) - subLen
	firstLower := substrLower[0]
	firstUpper := firstLower
	if firstLower >= 'a' && firstLower <= 'z' {
		firstUpper = firstLower - ('a' - 'A')
	}

	for i := 0; i <= maxI; i++ {
		c := s[i]
		if c != firstLower && c != firstUpper {
			continue
		}
		match := true
		for j := 1; j < subLen; j++ {
			sc := s[i+j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 'a' - 'A'
			}
			if sc != substrLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// classifyLogLevel classifies a log line into one of: "error", "warn", "debug", "info".
// - error: matches err, fatal, panic, exception, fail, [err]
// - warn: matches warn, warning, timeout
// - debug: matches debug, trace
// - info: all other nominal lines
func classifyLogLevel(line []byte) string {
	// Error patterns: err, fatal, panic, exception, fail, [err]
	if bytesContainsFold(line, "fatal") ||
		bytesContainsFold(line, "panic") ||
		bytesContainsFold(line, "exception") ||
		bytesContainsFold(line, "fail") ||
		bytesContainsFold(line, "err") {
		return "error"
	}

	// Warn patterns: warn, warning, timeout
	if bytesContainsFold(line, "warn") ||
		bytesContainsFold(line, "timeout") {
		return "warn"
	}

	// Debug patterns: debug, trace
	if bytesContainsFold(line, "debug") ||
		bytesContainsFold(line, "trace") {
		return "debug"
	}

	// Nominal info
	return "info"
}

// FilterLogStream performs fast server-side log filtering with bounded buffers and minimal allocations.
// - Uses bufio.Scanner with a 64KB base and 256KB max line buffer.
// - Performs case-insensitive search matching for searchQuery (if provided).
// - Performs fast log level classification (error, warn, debug, info).
// - If levelFilter is specified and != "all", only matching lines are retained.
// - If limit > 0, caps output to at most limit lines.
// - Joins matching lines with newline delimiter.
func FilterLogStream(rawLogs string, searchQuery string, levelFilter string, limit int) string {
	if rawLogs == "" {
		return ""
	}

	searchLower := strings.ToLower(strings.TrimSpace(searchQuery))
	levelLower := strings.ToLower(strings.TrimSpace(levelFilter))
	switch levelLower {
	case "warning":
		levelLower = "warn"
	case "trace":
		levelLower = "debug"
	case "errors":
		levelLower = "error"
	}

	scanner := bufio.NewScanner(strings.NewReader(rawLogs))
	buf := make([]byte, baseLogBufferSize)
	scanner.Buffer(buf, maxLogBufferSize)

	var b strings.Builder
	if len(rawLogs) < maxLogBufferSize {
		b.Grow(len(rawLogs))
	} else {
		b.Grow(maxLogBufferSize)
	}

	count := 0
	for scanner.Scan() {
		line := scanner.Bytes()

		// 1. Case-insensitive search query filter
		if searchLower != "" && !bytesContainsFold(line, searchLower) {
			continue
		}

		// 2. Log level filter
		if levelLower != "" && levelLower != "all" {
			detectedLevel := classifyLogLevel(line)
			if detectedLevel != levelLower {
				continue
			}
		}

		// 3. Append line
		if count > 0 {
			b.WriteByte('\n')
		}
		b.Write(line)
		count++

		// 4. Limit truncation
		if limit > 0 && count >= limit {
			break
		}
	}

	return b.String()
}
