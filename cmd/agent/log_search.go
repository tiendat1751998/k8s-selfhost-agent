package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// QueryLogs queries logs across sources matching the given filters.
func (s *LogServer) QueryLogs(ctx context.Context, app string, tail int, sinceStr, untilStr, query, level string) ([]LogEntry, error) {
	s.mu.RLock()
	sources := s.sources
	s.mu.RUnlock()

	sinceTime, err := parseTimeParam(sinceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid since parameter: %w", err)
	}

	untilTime, err := parseTimeParam(untilStr)
	if err != nil {
		return nil, fmt.Errorf("invalid until parameter: %w", err)
	}

	var allEntries []LogEntry
	for _, src := range sources {
		entries, err := src.GetLogs(ctx, app, tail, sinceTime, untilTime, query, level)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	// Sort chronologically
	sort.SliceStable(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Tail filtering if specified and positive
	if tail > 0 && len(allEntries) > tail {
		allEntries = allEntries[len(allEntries)-tail:]
	}

	return allEntries, nil
}

// SearchLogs executes search query across logs.
func (s *LogServer) SearchLogs(ctx context.Context, req LogSearchRequest) ([]LogEntry, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	// Query with all lines and then limit
	entries, err := s.QueryLogs(ctx, req.App, 0, req.Since, req.Until, req.Query, req.Level)
	if err != nil {
		return nil, err
	}

	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return entries, nil
}

// formatLogEntry formats a LogEntry into a line string.
func formatLogEntry(e LogEntry) string {
	if e.Raw != "" {
		return e.Raw
	}
	ts := e.Timestamp.UTC().Format(time.RFC3339Nano)
	if e.Service != "" {
		if e.Level != "" {
			return fmt.Sprintf("%s [%s] [%s] %s", ts, e.Service, strings.ToUpper(e.Level), e.Message)
		}
		return fmt.Sprintf("%s [%s] %s", ts, e.Service, e.Message)
	}
	if e.Level != "" {
		return fmt.Sprintf("%s [%s] %s", ts, strings.ToUpper(e.Level), e.Message)
	}
	return fmt.Sprintf("%s %s", ts, e.Message)
}

// -----------------------------------------------------------------------------
// Helper parsing functions
// -----------------------------------------------------------------------------

// parseLogLine parses timestamp, level, message from raw log line.
func parseLogLine(rawLine string, defaultService string) LogEntry {
	rawLine = strings.TrimRight(rawLine, "\r\n")
	entry := LogEntry{
		Service: defaultService,
		Raw:     rawLine,
	}
	if strings.TrimSpace(rawLine) == "" {
		entry.Timestamp = time.Now().UTC()
		entry.Level = "info"
		return entry
	}

	// Try extracting timestamp from beginning of line (e.g. Docker format: "2026-08-24T14:30:00.123456789Z message")
	fields := strings.SplitN(rawLine, " ", 2)
	parsedTime := false
	if len(fields) >= 1 {
		tsCandidate := strings.Trim(fields[0], "[]")
		if t, err := time.Parse(time.RFC3339Nano, tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		} else if t, err := time.Parse(time.RFC3339, tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		} else if t, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		} else if t, err := time.Parse("2006-01-02 15:04:05", tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		}
	}

	if !parsedTime {
		entry.Timestamp = time.Now().UTC()
		entry.Message = rawLine
	}

	entry.Level = detectLogLevel(rawLine)
	return entry
}

// detectLogLevel classifies log level from content.
func detectLogLevel(line string) string {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") || strings.Contains(lower, "err") ||
		strings.Contains(lower, "fatal") || strings.Contains(lower, "crit") ||
		strings.Contains(lower, "panic") || strings.Contains(lower, "level=error") ||
		strings.Contains(lower, `"level":"error"`) {
		return "error"
	}
	if strings.Contains(lower, "warn") || strings.Contains(lower, "warning") ||
		strings.Contains(lower, "level=warn") || strings.Contains(lower, `"level":"warn"`) {
		return "warn"
	}
	if strings.Contains(lower, "debug") || strings.Contains(lower, "trace") ||
		strings.Contains(lower, "level=debug") || strings.Contains(lower, `"level":"debug"`) {
		return "debug"
	}
	return "info"
}

// matchLevel checks if detected level satisfies the requested filter level.
func matchLevel(detectedLevel, filterLevel string) bool {
	filterLevel = strings.TrimSpace(strings.ToLower(filterLevel))
	if filterLevel == "" {
		return true
	}
	detectedLevel = strings.TrimSpace(strings.ToLower(detectedLevel))

	switch filterLevel {
	case "error", "err", "fatal", "critical":
		return detectedLevel == "error" || detectedLevel == "fatal" || detectedLevel == "critical"
	case "warn", "warning":
		return detectedLevel == "error" || detectedLevel == "fatal" || detectedLevel == "critical" || detectedLevel == "warn"
	case "info":
		return detectedLevel == "error" || detectedLevel == "warn" || detectedLevel == "info"
	case "debug", "trace":
		return true
	default:
		return detectedLevel == filterLevel || strings.Contains(detectedLevel, filterLevel)
	}
}

// matchTime checks if timestamp is within [since, until] window.
func matchTime(ts time.Time, sinceTime, untilTime *time.Time) bool {
	if sinceTime != nil && ts.Before(*sinceTime) {
		return false
	}
	if untilTime != nil && ts.After(*untilTime) {
		return false
	}
	return true
}

// matchQuery checks if query substring is in message or raw line.
func matchQuery(raw, msg, query string) bool {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(raw), query) || strings.Contains(strings.ToLower(msg), query)
}

// parseTimeParam parses ISO timestamp or relative duration (e.g. "15m", "1h", "24h").
func parseTimeParam(val string) (*time.Time, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil, nil
	}

	// Try relative duration
	if d, err := parseRelativeDuration(val); err == nil {
		t := time.Now().UTC().Add(-d)
		return &t, nil
	}

	// Try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try "2006-01-02T15:04:05"
	if t, err := time.Parse("2006-01-02T15:04:05", val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try "2006-01-02 15:04:05"
	if t, err := time.Parse("2006-01-02 15:04:05", val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try unix timestamp
	if sec, err := strconv.ParseInt(val, 10, 64); err == nil {
		t := time.Unix(sec, 0).UTC()
		return &t, nil
	}

	return nil, fmt.Errorf("unrecognized time format: %q", val)
}

// parseRelativeDuration parses durations including day ('d') and week ('w') suffixes.
func parseRelativeDuration(val string) (time.Duration, error) {
	val = strings.TrimSpace(val)
	if strings.HasSuffix(val, "d") {
		numStr := strings.TrimSuffix(val, "d")
		days, err := strconv.Atoi(numStr)
		if err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	if strings.HasSuffix(val, "w") {
		numStr := strings.TrimSuffix(val, "w")
		weeks, err := strconv.Atoi(numStr)
		if err == nil {
			return time.Duration(weeks) * 7 * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(val)
}

