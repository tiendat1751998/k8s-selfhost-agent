package metrics

import (
	"context"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	domainLB "github.com/datdt/k8sselfhost/internal/domain/loadbalancer"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// TPSSnapshot holds aggregated throughput metrics from all 5 sources.
type TPSSnapshot struct {
	Timestamp time.Time    `json:"timestamp"`
	Network   NetworkTPS   `json:"network"`
	HTTP      HTTPTPS      `json:"http"`
	Database  DatabaseTPS  `json:"database"`
	Messaging MessagingTPS `json:"messaging"`
	PerNode   []NodeTPS    `json:"per_node"`
	Services  []ServiceTPS `json:"services"`
}

// ServiceTPS holds aggregated throughput and resource metrics for a Docker/Swarm service.
type ServiceTPS struct {
	ServiceName    string  `json:"service_name"`
	NodeID         string  `json:"node_id,omitempty"`
	NodeName       string  `json:"node_name,omitempty"`
	ContainerCount int     `json:"container_count"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryUsedMB   float64 `json:"memory_used_mb"`
	MemoryPercent  float64 `json:"memory_percent"`
	RxBytesPerSec  int64   `json:"rx_bytes_per_sec"`
	TxBytesPerSec  int64   `json:"tx_bytes_per_sec"`
	TotalRxBytes   int64   `json:"total_rx_bytes,omitempty"` // Lifetime cumulative Rx bytes
	TotalTxBytes   int64   `json:"total_tx_bytes,omitempty"` // Lifetime cumulative Tx bytes
	RequestsPerSec float64 `json:"requests_per_sec"`
	ErrorRate      float64 `json:"error_rate"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	MaxLatencyMs   float64 `json:"max_latency_ms,omitempty"`
	Status         string  `json:"status"` // healthy, degraded, down
}

// NetworkTPS holds network throughput metrics across nodes and containers.
type NetworkTPS struct {
	TotalRxBytesPerSec int64 `json:"total_rx_bytes_per_sec"`
	TotalTxBytesPerSec int64 `json:"total_tx_bytes_per_sec"`
	TotalPacketsPerSec int64 `json:"total_packets_per_sec,omitempty"`
}

// HTTPTPS holds HTTP throughput and health metrics from edge proxy/gateway.
type HTTPTPS struct {
	RequestsPerSec    float64 `json:"requests_per_sec"`
	ActiveConnections int     `json:"active_connections"`
	QueuedRequests    int     `json:"queued_requests"`
	TotalRequests     int64   `json:"total_requests"`
	ErrorRate         float64 `json:"error_rate"` // % of 4xx+5xx
	AvgLatencyMs      float64 `json:"avg_latency_ms,omitempty"`
	MaxLatencyMs      float64 `json:"max_latency_ms,omitempty"`
}

// DatabaseTPS holds database transactional throughput metrics from pg_stat_database.
type DatabaseTPS struct {
	TransactionsPerSec float64 `json:"transactions_per_sec"`
	ReadsPerSec        float64 `json:"reads_per_sec"`
	WritesPerSec       float64 `json:"writes_per_sec"`
	ActiveConnections  int     `json:"active_connections"`
	CacheHitRatio      float64 `json:"cache_hit_ratio"` // blks_hit / (blks_hit + blks_read)
}

// MessagingTPS holds messaging throughput and state metrics from NATS varz.
type MessagingTPS struct {
	InMsgsPerSec   float64 `json:"in_msgs_per_sec"`
	OutMsgsPerSec  float64 `json:"out_msgs_per_sec"`
	InBytesPerSec  int64   `json:"in_bytes_per_sec"`
	OutBytesPerSec int64   `json:"out_bytes_per_sec"`
	Connections    int     `json:"connections"`
	Streams        int     `json:"streams,omitempty"`
}

// NodeTPS holds throughput and resource rate metrics for a single node.
type NodeTPS struct {
	NodeName        string `json:"node_name"`
	NodeID          string `json:"node_id"`
	RxBytesPerSec   int64  `json:"rx_bytes_per_sec"`
	TxBytesPerSec   int64  `json:"tx_bytes_per_sec"`
	DiskReadPerSec  int64  `json:"disk_read_per_sec,omitempty"`
	DiskWritePerSec int64  `json:"disk_write_per_sec,omitempty"`
	Processes       int    `json:"processes"`
}

// MetricsSnapshotProvider defines the metric source interface for agent and container statistics.
type MetricsSnapshotProvider interface {
	GetLastSnapshot() *SystemOverview
	GetAgentMetrics() map[string]*AgentMetrics
}

// DBStatsQuerier defines the database query interface for pg_stat_database metrics.
type DBStatsQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type containerNetRaw struct {
	rxBytes int64
	txBytes int64
}

// safeDeltaInt64 protects against negative deltas when counters reset across restarts.
func safeDeltaInt64(curr, prev int64) int64 {
	if curr < prev {
		// Counter was reset after service restart / container recreation
		return curr
	}
	return curr - prev
}

// TPSCollector aggregates throughput metrics across network, HTTP, database, messaging, and containers.
type TPSCollector struct {
	metricsProvider MetricsSnapshotProvider
	dbPool          DBStatsQuerier
	httpClient      *http.Client
	natsMonitorURL  string
	traefikAPIURL   string
	lbProvider      domainLB.Provider
	logger          *zap.Logger
	interval        time.Duration
	requestCountFn  func() int64

	mu                 sync.RWMutex
	lastSnapshot       *TPSSnapshot
	prevDBStats        dbStatsRaw
	prevDBTime         time.Time
	prevNATSStats      natsStatsRaw
	prevNATSTime       time.Time
	prevHTTPStats      httpStatsRaw
	prevHTTPTime       time.Time
	prevContainerStats map[string]containerNetRaw
	prevContainerTime  time.Time

	stopCh chan struct{}
}

// TPSCollectorOption configures TPSCollector.
type TPSCollectorOption func(*TPSCollector)

// WithTPSInterval sets the polling interval.
func WithTPSInterval(d time.Duration) TPSCollectorOption {
	return func(c *TPSCollector) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithTPSHTTPClient sets a custom HTTP client for external API requests.
func WithTPSHTTPClient(client *http.Client) TPSCollectorOption {
	return func(c *TPSCollector) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithTraefikURL sets the Traefik API base URL.
func WithTraefikURL(url string) TPSCollectorOption {
	return func(c *TPSCollector) {
		if url != "" {
			c.traefikAPIURL = strings.TrimRight(url, "/")
		}
	}
}

// WithLoadBalancerProvider sets the load balancer metrics provider.
func WithLoadBalancerProvider(provider domainLB.Provider) TPSCollectorOption {
	return func(c *TPSCollector) {
		c.lbProvider = provider
	}
}

// WithNATSMonitorURL sets the NATS varz monitoring URL.
func WithNATSMonitorURL(url string) TPSCollectorOption {
	return func(c *TPSCollector) {
		if url != "" {
			c.natsMonitorURL = url
		}
	}
}

// WithTPSRequestCountFn sets the fallback request counter function.
func WithTPSRequestCountFn(fn func() int64) TPSCollectorOption {
	return func(c *TPSCollector) {
		c.requestCountFn = fn
	}
}

// NewTPSCollector creates a new TPSCollector instance.
func NewTPSCollector(metricsProvider MetricsSnapshotProvider, dbPool DBStatsQuerier, logger *zap.Logger, opts ...TPSCollectorOption) *TPSCollector {
	if logger == nil {
		logger = zap.NewNop()
	}

	c := &TPSCollector{
		metricsProvider:    metricsProvider,
		dbPool:             dbPool,
		httpClient:         &http.Client{Timeout: 4 * time.Second},
		traefikAPIURL:      "http://localhost:8080",
		natsMonitorURL:     "http://localhost:8222/varz",
		logger:             logger,
		interval:           5 * time.Second,
		prevContainerStats: make(map[string]containerNetRaw),
		stopCh:             make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start begins periodic background metric collection until context cancellation or Stop.
func (c *TPSCollector) Start(ctx context.Context) {
	c.logger.Info("Starting TPS collector service",
		zap.Duration("interval", c.interval),
		zap.String("traefik_url", c.traefikAPIURL),
		zap.String("nats_url", c.natsMonitorURL),
	)

	// Run initial collection
	if _, err := c.Collect(ctx); err != nil {
		c.logger.Warn("Initial TPS collection failed", zap.Error(err))
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping TPS collector (context cancelled)")
			return
		case <-c.stopCh:
			c.logger.Info("Stopping TPS collector (stop requested)")
			return
		case <-ticker.C:
			collectCtx, cancel := context.WithTimeout(ctx, c.interval-500*time.Millisecond)
			if _, err := c.Collect(collectCtx); err != nil {
				c.logger.Warn("Periodic TPS collection failed", zap.Error(err))
			}
			cancel()
		}
	}
}

// Stop terminates the background collection loop.
func (c *TPSCollector) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

// GetLastSnapshot returns the latest cached TPSSnapshot or performs a collection if nil.
func (c *TPSCollector) GetLastSnapshot() *TPSSnapshot {
	c.mu.RLock()
	snap := c.lastSnapshot
	c.mu.RUnlock()

	if snap != nil {
		return snap
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	fresh, _ := c.Collect(ctx)
	return fresh
}

// Collect gathers all 5 throughput sources and calculates TPS deltas.
func (c *TPSCollector) Collect(ctx context.Context) (*TPSSnapshot, error) {
	now := time.Now().UTC()

	// 1 & 5. Network I/O (Agents) & Container Network (Docker Stats) & Per-Service
	netTPS, perNode, services := c.collectNetworkAndContainerTPS(now)

	// 2. HTTP TPS (Traefik API)
	httpTPS := c.collectHTTPTPS(ctx, now)

	// 2b. Override with Load Balancer aggregate stats from Prometheus entrypoints when available
	lbReportedRPS := false
	if c.lbProvider != nil {
		aggStats, err := c.lbProvider.GetAggregateStats(ctx)
		if err == nil && aggStats != nil {
			httpTPS.RequestsPerSec = aggStats.TotalRequestsPerSec
			lbReportedRPS = true
			httpTPS.ActiveConnections = aggStats.ActiveConnections
			httpTPS.ErrorRate = aggStats.ErrorRate
			if aggStats.AvgLatencyMs > 0 {
				httpTPS.AvgLatencyMs = aggStats.AvgLatencyMs
			}
			if aggStats.MaxLatencyMs > 0 {
				httpTPS.MaxLatencyMs = aggStats.MaxLatencyMs
			}
			if aggStats.TotalRequests > 0 {
				httpTPS.TotalRequests = aggStats.TotalRequests
			}
		} else if err != nil {
			c.logger.Debug("Failed to get load balancer aggregate stats", zap.Error(err))
		}
	}

	// Fallback only if Load Balancer is NOT available or errored:
	if !lbReportedRPS {
		var traefikRx int64
		for _, s := range services {
			if isTraefikService(s.ServiceName) {
				traefikRx += s.RxBytesPerSec
			}
		}
		if traefikRx > 10000 {
			// Estimate real HTTP throughput: ~1.2 KB average request size
			estimatedRPS := math.Round((float64(traefikRx)/1200.0)*100) / 100
			httpTPS.RequestsPerSec = estimatedRPS
		}
	}

	// 3. Database TPS (pg_stat_database)
	dbTPS := c.collectDatabaseTPS(ctx, now)

	// 4. Messaging TPS (NATS /varz)
	msgTPS := c.collectMessagingTPS(ctx, now)

	// Enrich per-service TPS metrics with load balancer request stats and backend metrics
	services = c.enrichServicesWithLBStats(ctx, services, httpTPS, dbTPS, msgTPS)

	// If LB provider or gateway services report request activity, enrich aggregate HTTP TPS
	var sumLBRPS float64
	var sumLBErrors float64
	var countLBServices int
	for _, s := range services {
		if !isTraefikService(s.ServiceName) && s.RequestsPerSec > 0 {
			sumLBRPS += s.RequestsPerSec
			countLBServices++
			sumLBErrors += s.ErrorRate
		}
	}
	if sumLBRPS > httpTPS.RequestsPerSec {
		httpTPS.RequestsPerSec = math.Round(sumLBRPS*100) / 100
		if countLBServices > 0 && httpTPS.ErrorRate == 0 {
			httpTPS.ErrorRate = math.Round((sumLBErrors/float64(countLBServices))*100) / 100
		}
	}

	snapshot := &TPSSnapshot{
		Timestamp: now,
		Network:   netTPS,
		HTTP:      httpTPS,
		Database:  dbTPS,
		Messaging: msgTPS,
		PerNode:   perNode,
		Services:  services,
	}

	c.mu.Lock()
	c.lastSnapshot = snapshot
	c.mu.Unlock()

	return snapshot, nil
}
