package http

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

type errorSummarizer interface {
	SummarizeErrors(ctx context.Context, filter domainLogging.LogFilter) ([]agent.ErrorRadarGroup, error)
}

// HandleErrorsRadar handles GET /api/v1/logs/errors for blind error discovery.
func (h *LogHandler) HandleErrorsRadar(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()

	node := firstNonEmpty(q.Get("node"), q.Get("node_name"), q.Get("host"))
	service := firstNonEmpty(q.Get("service"), q.Get("app"), q.Get("container_name"), q.Get("container"))
	level := firstNonEmpty(q.Get("level"), q.Get("log_level"))
	limit := 50
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 { limit = l }
	}

	startTime, endTime, _ := parseTimeRangeParams(q)
	if startTime.IsZero() {
		if s := firstNonEmpty(q.Get("since")); s != "" {
			if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
				startTime = t
			} else if t, err := time.Parse(time.RFC3339, s); err == nil {
				startTime = t
			}
		}
	}
	if endTime.IsZero() {
		if u := firstNonEmpty(q.Get("until")); u != "" {
			if t, err := time.Parse(time.RFC3339Nano, u); err == nil {
				endTime = t
			} else if t, err := time.Parse(time.RFC3339, u); err == nil {
				endTime = t
			}
		}
	}

	filter := domainLogging.LogFilter{
		ContainerName: service,
		ServiceName:   service,
		StartTime:     startTime,
		EndTime:       endTime,
		Limit:         limit,
	}
	if level != "" {
		lvl, _ := domainLogging.ParseLogLevel(level)
		filter.LogLevel = lvl
	}
	if node != "" {
		filter.Attributes = map[string]string{"node": node, "node_name": node}
	}

	var groups []agent.ErrorRadarGroup
	var err error

	// 1. Try statusProvider or service direct SummarizeErrors
	if es, ok := h.statusProvider.(errorSummarizer); ok {
		groups, err = es.SummarizeErrors(r.Context(), filter)
	} else if es, ok := h.service.(errorSummarizer); ok {
		groups, err = es.SummarizeErrors(r.Context(), filter)
	} else if repoHolder, ok := h.service.(interface{ Repository() any }); ok {
		if es, ok := repoHolder.Repository().(errorSummarizer); ok {
			groups, err = es.SummarizeErrors(r.Context(), filter)
		}
	}

	// 2. Fallback: query logs via logging service and aggregate in-memory
	if groups == nil && err == nil && h.service != nil {
		fallbackFilter := filter
		if fallbackFilter.LogLevel == "" { fallbackFilter.LogLevel = domainLogging.LogLevelError }
		fallbackFilter.Limit = 1000
		res, qErr := h.service.QueryLogs(r.Context(), fallbackFilter)
		if qErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": qErr.Error()})
			return
		}
		if res != nil {
			groupMap := make(map[string]*agent.ErrorRadarGroup)
			for _, e := range res.Entries {
				if logengine.IsNoiseService(e.ContainerName) {
					continue
				}
				nodeName := ""
				if e.Attributes != nil {
					nodeName = e.Attributes["node_name"]
					if nodeName == "" { nodeName = e.Attributes["node_id"] }
				}
				fp := logengine.FingerprintError([]byte(e.Message))
				key := fmt.Sprintf("%s|%s|%s|%s", nodeName, e.ContainerName, string(e.LogLevel), fp)
				if eg, exists := groupMap[key]; exists {
					eg.Count++
					if e.Timestamp.Before(eg.FirstSeen) { eg.FirstSeen = e.Timestamp }
					if e.Timestamp.After(eg.LastSeen) { eg.LastSeen = e.Timestamp }
				} else {
					groupMap[key] = &agent.ErrorRadarGroup{
						Node: nodeName, Service: e.ContainerName, Level: string(e.LogLevel),
						Fingerprint: fp, SampleMessage: e.Message, Count: 1,
						FirstSeen: e.Timestamp, LastSeen: e.Timestamp,
					}
				}
			}
			for _, g := range groupMap { groups = append(groups, *g) }
			sort.Slice(groups, func(i, j int) bool {
				if groups[i].Count != groups[j].Count { return groups[i].Count > groups[j].Count }
				return groups[i].LastSeen.After(groups[j].LastSeen)
			})
			if limit > 0 && len(groups) > limit { groups = groups[:limit] }
		}
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if groups == nil { groups = []agent.ErrorRadarGroup{} }

	totalOccurrences := 0
	for _, g := range groups { totalOccurrences += g.Count }

	writeJSON(w, http.StatusOK, map[string]any{
		"groups":            groups,
		"total_groups":     len(groups),
		"total_occurrences": totalOccurrences,
		"took_ms":          float64(time.Since(start).Microseconds()) / 1000.0,
	})
}
