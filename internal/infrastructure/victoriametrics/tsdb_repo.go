// Package victoriametrics provides a high-throughput time-series database adapter
// for VictoriaMetrics with Prometheus line protocol ingestion and PromQL range querying.
package victoriametrics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrTenantRequired is returned when a write or query operation lacks a tenant ID.
	ErrTenantRequired = errors.New("tenant_id is required for tenant isolation")
	// ErrInvalidStatus is returned when VictoriaMetrics responds with non-success status.
	ErrInvalidStatus = errors.New("victoriametrics returned unexpected status code")
)

// MetricSample represents a single point-in-time telemetry sample for a cluster node.
type MetricSample struct {
	NodeID      string  `json:"node_id"`
	ClusterID   string  `json:"cluster_id"`
	TenantID    string  `json:"tenant_id"`
	TimestampMs int64   `json:"timestamp_ms"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float64 `json:"mem_percent"`
	DiskPercent float64 `json:"disk_percent"`
	NetRxBytes  int64   `json:"net_rx_bytes"`
	NetTxBytes  int64   `json:"net_tx_bytes"`
}

// MetricPoint holds a single timestamped float measurement.
type MetricPoint struct {
	TimestampMs int64   `json:"timestamp_ms"`
	Value       float64 `json:"value"`
}

// TimeSeries represents a series of measurements with associated metric labels.
type TimeSeries struct {
	Metric map[string]string `json:"metric"`
	Points []MetricPoint     `json:"points"`
}

// TSDBConfig configures connection parameters for VictoriaMetrics.
type TSDBConfig struct {
	BaseURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// TSDBRepository implements high-throughput batch writes and queries against VictoriaMetrics.
type TSDBRepository struct {
	baseURL string
	client  *http.Client
	bufPool sync.Pool
}

// Internal JSON response types matching Prometheus/VictoriaMetrics HTTP API
type vmQueryResponse struct {
	Status string `json:"status"`
	Data   vmData `json:"data"`
}

type vmData struct {
	ResultType string     `json:"resultType"`
	Result     []vmResult `json:"result"`
}

type vmResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

// NewTSDBRepository constructs a TSDBRepository with connection pooling and buffer reuse.
func NewTSDBRepository(cfg TSDBConfig) *TSDBRepository {
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 50,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	return &TSDBRepository{
		baseURL: baseURL,
		client:  client,
		bufPool: sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, 0, 16384))
			},
		},
	}
}

// WriteBatch serializes MetricSample slices into Prometheus line protocol using pooled buffers
// and sends them via HTTP POST to VictoriaMetrics /api/v1/import/prometheus.
func (r *TSDBRepository) WriteBatch(ctx context.Context, samples []MetricSample) error {
	if len(samples) == 0 {
		return nil
	}

	// Validate tenant isolation on all samples before network dispatch
	for i := range samples {
		if samples[i].TenantID == "" {
			return fmt.Errorf("sample on node %s: %w", samples[i].NodeID, ErrTenantRequired)
		}
	}

	buf := r.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		// Cap buffer capacity to prevent runaway memory if a giant batch was processed
		if buf.Cap() <= 1048576 {
			r.bufPool.Put(buf)
		}
	}()

	var scratch [32]byte
	for i := range samples {
		s := &samples[i]
		writePrometheusFloatLine(buf, &scratch, "node_cpu_percent", s.TenantID, s.ClusterID, s.NodeID, s.CPUPercent, s.TimestampMs)
		writePrometheusFloatLine(buf, &scratch, "node_mem_percent", s.TenantID, s.ClusterID, s.NodeID, s.MemPercent, s.TimestampMs)
		writePrometheusFloatLine(buf, &scratch, "node_disk_percent", s.TenantID, s.ClusterID, s.NodeID, s.DiskPercent, s.TimestampMs)
		writePrometheusIntLine(buf, &scratch, "node_net_rx_bytes", s.TenantID, s.ClusterID, s.NodeID, s.NetRxBytes, s.TimestampMs)
		writePrometheusIntLine(buf, &scratch, "node_net_tx_bytes", s.TenantID, s.ClusterID, s.NodeID, s.NetTxBytes, s.TimestampMs)
	}

	reqURL := r.baseURL + "/api/v1/import/prometheus"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting metrics batch to %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%w: status=%d response=%s", ErrInvalidStatus, resp.StatusCode, string(respBody))
	}

	return nil
}

// writePrometheusFloatLine formats a float metric line directly into the buffer.
func writePrometheusFloatLine(buf *bytes.Buffer, scratch *[32]byte, metric, tenantID, clusterID, nodeID string, val float64, tsMs int64) {
	buf.WriteString(metric)
	buf.WriteString(`{tenant_id="`)
	buf.WriteString(tenantID)
	buf.WriteString(`",cluster_id="`)
	buf.WriteString(clusterID)
	buf.WriteString(`",node_id="`)
	buf.WriteString(nodeID)
	buf.WriteString(`"} `)
	buf.Write(strconv.AppendFloat(scratch[:0], val, 'f', 2, 64))
	buf.WriteByte(' ')
	buf.Write(strconv.AppendInt(scratch[:0], tsMs, 10))
	buf.WriteByte('\n')
}

// writePrometheusIntLine formats an integer metric line directly into the buffer.
func writePrometheusIntLine(buf *bytes.Buffer, scratch *[32]byte, metric, tenantID, clusterID, nodeID string, val int64, tsMs int64) {
	buf.WriteString(metric)
	buf.WriteString(`{tenant_id="`)
	buf.WriteString(tenantID)
	buf.WriteString(`",cluster_id="`)
	buf.WriteString(clusterID)
	buf.WriteString(`",node_id="`)
	buf.WriteString(nodeID)
	buf.WriteString(`"} `)
	buf.Write(strconv.AppendInt(scratch[:0], val, 10))
	buf.WriteByte(' ')
	buf.Write(strconv.AppendInt(scratch[:0], tsMs, 10))
	buf.WriteByte('\n')
}

// QueryRange executes a PromQL range query against /api/v1/query_range, enforcing tenant isolation.
func (r *TSDBRepository) QueryRange(ctx context.Context, tenantID, clusterID, nodeID string, startMs, endMs int64) ([]TimeSeries, error) {
	if tenantID == "" {
		return nil, ErrTenantRequired
	}

	// Build label selector enforcing tenant_id isolation
	var selector strings.Builder
	selector.WriteString(`{tenant_id="`)
	selector.WriteString(tenantID)
	selector.WriteByte('"')

	if clusterID != "" {
		selector.WriteString(`,cluster_id="`)
		selector.WriteString(clusterID)
		selector.WriteByte('"')
	}
	if nodeID != "" {
		selector.WriteString(`,node_id="`)
		selector.WriteString(nodeID)
		selector.WriteByte('"')
	}
	selector.WriteByte('}')

	step := calculateStep(startMs, endMs)

	params := url.Values{}
	params.Set("query", selector.String())
	params.Set("start", strconv.FormatFloat(float64(startMs)/1000.0, 'f', 3, 64))
	params.Set("end", strconv.FormatFloat(float64(endMs)/1000.0, 'f', 3, 64))
	params.Set("step", step)

	reqURL := r.baseURL + "/api/v1/query_range?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating query request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying victoriametrics at %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%w: status=%d response=%s", ErrInvalidStatus, resp.StatusCode, string(body))
	}

	var res vmQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decoding query response: %w", err)
	}

	out := make([]TimeSeries, len(res.Data.Result))
	for i, rItem := range res.Data.Result {
		pts := make([]MetricPoint, 0, len(rItem.Values))
		for _, rawPoint := range rItem.Values {
			if len(rawPoint) < 2 {
				continue
			}
			secFloat, ok1 := rawPoint[0].(float64)
			valStr, ok2 := rawPoint[1].(string)
			if !ok1 || !ok2 {
				continue
			}
			valFloat, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}
			pts = append(pts, MetricPoint{
				TimestampMs: int64(secFloat * 1000),
				Value:       valFloat,
			})
		}
		out[i] = TimeSeries{
			Metric: rItem.Metric,
			Points: pts,
		}
	}

	return out, nil
}

// calculateStep determines an appropriate PromQL query step duration based on time span.
func calculateStep(startMs, endMs int64) string {
	diffSec := (endMs - startMs) / 1000
	if diffSec <= 60 {
		return "1s"
	}
	if diffSec <= 600 {
		return "5s"
	}
	if diffSec <= 3600 {
		return "15s"
	}
	if diffSec <= 86400 {
		return "1m"
	}
	return "5m"
}
