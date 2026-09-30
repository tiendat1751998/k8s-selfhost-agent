package http

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
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

func getContainerAliases(t string) []string {
	if s := strings.ToLower(strings.TrimSpace(t)); s == "postgres_db" || s == "postgres" {
		return []string{"postgres_db", "postgres"}
	}
	if t != "" {
		return []string{t}
	}
	return nil
}

func resolveTenant(ctx context.Context, r *http.Request) string {
	if tenantID := tenancy.TenantIDFromContext(ctx); strings.TrimSpace(tenantID) != "" {
		return strings.TrimSpace(tenantID)
	}
	if r != nil {
		if hTenant := strings.TrimSpace(r.Header.Get("X-Tenant-ID")); hTenant != "" {
			return hTenant
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func parseLogLevelParam(q url.Values, key string) (logging.LogLevel, error) {
	s := firstNonEmpty(q.Get(key), q.Get("level"))
	if s != "" {
		return logging.ParseLogLevel(s)
	}
	return "", nil
}

func parseTimeRangeParams(q url.Values) (time.Time, time.Time, error) {
	var startTime, endTime time.Time
	if s := q.Get("start_time"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid start_time format (RFC3339 required)")
		}
		startTime = t
	}
	if s := q.Get("end_time"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid end_time format (RFC3339 required)")
		}
		endTime = t
	}
	return startTime, endTime, nil
}

// queryAliases queries multiple container aliases and merges/deduplicates the results.
func queryAliases(ctx context.Context, svc LoggingService, filter logging.LogFilter, aliases []string) *logging.LogSearchResult {
	var allEntries []logging.LogEntry
	seen := make(map[string]bool)
	for _, alias := range aliases {
		af := filter
		af.ContainerName, af.ServiceName = alias, alias
		subRes, subErr := svc.QueryLogs(ctx, af)
		if subErr != nil || subRes == nil { continue }
		for _, e := range subRes.Entries {
			if strings.TrimSpace(e.Message) == "-- No entries --" { continue }
			key := fmt.Sprintf("%d|%s|%s", e.Timestamp.UnixNano(), e.ContainerName, e.Message)
			if !seen[key] {
				seen[key] = true
				allEntries = append(allEntries, e)
			}
		}
	}
	sort.SliceStable(allEntries, func(i, j int) bool { return allEntries[i].Timestamp.After(allEntries[j].Timestamp) })
	totalCount := int64(len(allEntries))
	offset, limit := filter.Offset, filter.Limit
	if limit <= 0 { limit = 500 }
	paged := []logging.LogEntry{}
	if offset < len(allEntries) {
		end := offset + limit
		if end > len(allEntries) { end = len(allEntries) }
		paged = allEntries[offset:end]
	}
	sort.SliceStable(paged, func(i, j int) bool { return paged[i].Timestamp.Before(paged[j].Timestamp) })
	return &logging.LogSearchResult{
		Entries: paged, TotalCount: totalCount, HasMore: offset+len(paged) < len(allEntries),
	}
}

// tailAliases tails logs across multiple container aliases into a single merged channel.
func tailAliases(ctx context.Context, svc LoggingService, filter logging.LogFilter, aliases []string) <-chan logging.LogEntry {
	mergedCh := make(chan logging.LogEntry, 100)
	var wg sync.WaitGroup
	for _, alias := range aliases {
		af := filter
		af.ContainerName, af.ServiceName = alias, alias
		ach, err := svc.TailLogs(ctx, af)
		if err != nil { continue }
		wg.Add(1)
		go func(c <-chan logging.LogEntry) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done(): return
				case e, ok := <-c:
					if !ok { return }
					select {
					case <-ctx.Done(): return
					case mergedCh <- e:
					}
				}
			}
		}(ach)
	}
	go func() { wg.Wait(); close(mergedCh) }()
	return mergedCh
}
