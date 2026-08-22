package metrics

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

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
	TotalRequests     int64   `json:"total_requests"`
	ErrorRate         float64 `json:"error_rate"` // % of 4xx+5xx
	AvgLatencyMs      float64 `json:"avg_latency_ms,omitempty"`
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

type dbStatsRaw struct {
	commit      int64
	rollback    int64
	tupReturned int64
	tupFetched  int64
	tupInserted int64
	tupUpdated  int64
	tupDeleted  int64
}

type natsStatsRaw struct {
	inMsgs   int64
	outMsgs  int64
	inBytes  int64
	outBytes int64
}

type httpStatsRaw struct {
	totalRequests int64
	errorRequests int64
}

type containerNetRaw struct {
	rxBytes int64
	txBytes int64
}

// TPSCollector aggregates throughput metrics across network, HTTP, database, messaging, and containers.
type TPSCollector struct {
	metricsProvider MetricsSnapshotProvider
	dbPool          DBStatsQuerier
	httpClient      *http.Client
	natsMonitorURL  string
	traefikAPIURL   string
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
	_, _ = c.Collect(ctx)

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
			_, _ = c.Collect(collectCtx)
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

	// 3. Database TPS (pg_stat_database)
	dbTPS := c.collectDatabaseTPS(ctx, now)

	// 4. Messaging TPS (NATS /varz)
	msgTPS := c.collectMessagingTPS(ctx, now)

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

// collectNetworkAndContainerTPS aggregates node and container network rates and per-service statistics.
func (c *TPSCollector) collectNetworkAndContainerTPS(now time.Time) (NetworkTPS, []NodeTPS, []ServiceTPS) {
	perNode := make([]NodeTPS, 0)
	services := make([]ServiceTPS, 0)
	var totalRx, totalTx int64

	if c.metricsProvider == nil {
		return NetworkTPS{}, perNode, services
	}

	nodeNameMap := make(map[string]string)
	nodeIDByName := make(map[string]string)
	agentMap := c.metricsProvider.GetAgentMetrics()
	for hostID, am := range agentMap {
		if am == nil {
			continue
		}
		if am.Hostname != "" {
			nodeNameMap[hostID] = am.Hostname
			nodeIDByName[strings.ToLower(strings.TrimSpace(am.Hostname))] = hostID
			nodeIDByName[normalizeNodeName(am.Hostname)] = hostID
		}
		var diskRead, diskWrite int64
		for _, p := range am.TopProcesses {
			diskRead += p.ReadBytesPerSec
			diskWrite += p.WriteBytesPerSec
		}

		node := NodeTPS{
			NodeName:        am.Hostname,
			NodeID:          hostID,
			RxBytesPerSec:   am.NetRxRate,
			TxBytesPerSec:   am.NetTxRate,
			DiskReadPerSec:  diskRead,
			DiskWritePerSec: diskWrite,
			Processes:       am.Processes,
		}
		perNode = append(perNode, node)
		totalRx += am.NetRxRate
		totalTx += am.NetTxRate
	}

	snapshot := c.metricsProvider.GetLastSnapshot()
	if snapshot != nil {
		for _, nm := range snapshot.Nodes {
			if nm.NodeID != "" && nm.NodeName != "" {
				nodeNameMap[nm.NodeID] = nm.NodeName
				nodeIDByName[strings.ToLower(strings.TrimSpace(nm.NodeName))] = nm.NodeID
				nodeIDByName[normalizeNodeName(nm.NodeName)] = nm.NodeID
			}
		}

		// If agentMap was empty, populate nodes from SystemOverview snapshot
		if len(agentMap) == 0 {
			for _, nm := range snapshot.Nodes {
				var diskRead, diskWrite int64
				for _, p := range nm.TopProcesses {
					diskRead += p.ReadBytesPerSec
					diskWrite += p.WriteBytesPerSec
				}
				node := NodeTPS{
					NodeName:        nm.NodeName,
					NodeID:          nm.NodeID,
					RxBytesPerSec:   nm.NetworkRxBytes,
					TxBytesPerSec:   nm.NetworkTxBytes,
					DiskReadPerSec:  diskRead,
					DiskWritePerSec: diskWrite,
					Processes:       nm.ContainerCount,
				}
				perNode = append(perNode, node)
				totalRx += nm.NetworkRxBytes
				totalTx += nm.NetworkTxBytes
			}
		}

		// Source 5: Aggregate Container Network delta stats & per-service rates
		c.mu.Lock()
		elapsed := 0.0
		if !c.prevContainerTime.IsZero() {
			elapsed = now.Sub(c.prevContainerTime).Seconds()
		}

		var containerRxDelta, containerTxDelta int64
		containerRxRates := make(map[string]int64, len(snapshot.Containers))
		containerTxRates := make(map[string]int64, len(snapshot.Containers))

		if elapsed > 0 {
			for _, cm := range snapshot.Containers {
				if prev, ok := c.prevContainerStats[cm.ContainerID]; ok {
					rxDelta := cm.NetworkRx - prev.rxBytes
					txDelta := cm.NetworkTx - prev.txBytes
					if rxDelta > 0 {
						containerRxDelta += rxDelta
						containerRxRates[cm.ContainerID] = int64(float64(rxDelta) / elapsed)
					}
					if txDelta > 0 {
						containerTxDelta += txDelta
						containerTxRates[cm.ContainerID] = int64(float64(txDelta) / elapsed)
					}
				}
			}
			// If no agent nodes reported, fallback to container network rates
			if len(agentMap) == 0 && len(snapshot.Nodes) == 0 {
				totalRx = int64(float64(containerRxDelta) / elapsed)
				totalTx = int64(float64(containerTxDelta) / elapsed)
			}
		}

		currMap := make(map[string]containerNetRaw, len(snapshot.Containers))
		for _, cm := range snapshot.Containers {
			currMap[cm.ContainerID] = containerNetRaw{
				rxBytes: cm.NetworkRx,
				txBytes: cm.NetworkTx,
			}
		}
		c.prevContainerStats = currMap
		c.prevContainerTime = now
		c.mu.Unlock()

		// Group containers by service and node
		if len(snapshot.Containers) > 0 {
			type serviceAcc struct {
				name          string
				nodeID        string
				nodeName      string
				totalCount    int
				runningCount  int
				totalCPU      float64
				totalMemBytes int64
				totalMemLimit int64
				rxRate        int64
				txRate        int64
			}

			serviceMap := make(map[string]*serviceAcc)
			serviceOrder := make([]string, 0)

			for _, cm := range snapshot.Containers {
				sName := extractServiceName(cm)
				if sName == "" {
					sName = "unknown"
				}

				nodeID := cm.NodeID
				nodeName := cm.NodeName

				if nodeID == "" && cm.Labels != nil {
					if nid, ok := cm.Labels["com.docker.swarm.node.id"]; ok {
						nodeID = nid
					}
				}

				if nodeName == "" && nodeID != "" {
					if name, ok := nodeNameMap[nodeID]; ok {
						nodeName = name
					}
				}

				if nodeName != "" {
					if id, ok := nodeIDByName[strings.ToLower(strings.TrimSpace(nodeName))]; ok {
						nodeID = id
					} else if id, ok := nodeIDByName[normalizeNodeName(nodeName)]; ok {
						nodeID = id
					}
				}

				if nodeName == "" && nodeID != "" {
					if id, ok := nodeIDByName[strings.ToLower(strings.TrimSpace(nodeID))]; ok {
						nodeName = nodeNameMap[id]
						nodeID = id
					} else if id, ok := nodeIDByName[normalizeNodeName(nodeID)]; ok {
						nodeName = nodeNameMap[id]
						nodeID = id
					}
				}

				if (nodeID == "" || nodeName == "") && len(snapshot.Nodes) == 1 {
					if nodeID == "" {
						nodeID = snapshot.Nodes[0].NodeID
					}
					if nodeName == "" {
						nodeName = snapshot.Nodes[0].NodeName
					}
				}

				if nodeName == "" && nodeID != "" {
					nodeName = nodeID
				}

				groupKey := sName + "|" + nodeID
				acc, ok := serviceMap[groupKey]
				if !ok {
					acc = &serviceAcc{
						name:     sName,
						nodeID:   nodeID,
						nodeName: nodeName,
					}
					serviceMap[groupKey] = acc
					serviceOrder = append(serviceOrder, groupKey)
				}

				acc.totalCount++
				if strings.EqualFold(cm.State, "running") {
					acc.runningCount++
				}
				acc.totalCPU += cm.CPUPercent
				acc.totalMemBytes += cm.MemoryUsed
				acc.totalMemLimit += cm.MemoryLimit
				acc.rxRate += containerRxRates[cm.ContainerID]
				acc.txRate += containerTxRates[cm.ContainerID]
			}

			for _, groupKey := range serviceOrder {
				acc := serviceMap[groupKey]
				var memPercent float64
				if acc.totalMemLimit > 0 {
					memPercent = math.Round((float64(acc.totalMemBytes)/float64(acc.totalMemLimit))*10000) / 100
				}

				status := "down"
				if acc.runningCount == acc.totalCount && acc.totalCount > 0 {
					status = "healthy"
				} else if acc.runningCount > 0 {
					status = "degraded"
				}

				services = append(services, ServiceTPS{
					ServiceName:    acc.name,
					NodeID:         acc.nodeID,
					NodeName:       acc.nodeName,
					ContainerCount: acc.totalCount,
					CPUPercent:     math.Round(acc.totalCPU*100) / 100,
					MemoryUsedMB:   math.Round((float64(acc.totalMemBytes)/(1024*1024))*100) / 100,
					MemoryPercent:  memPercent,
					RxBytesPerSec:  acc.rxRate,
					TxBytesPerSec:  acc.txRate,
					Status:         status,
				})
			}

			// Sort services by CPU percent descending, then MemoryUsedMB descending
			sort.Slice(services, func(i, j int) bool {
				if services[i].CPUPercent != services[j].CPUPercent {
					return services[i].CPUPercent > services[j].CPUPercent
				}
				return services[i].MemoryUsedMB > services[j].MemoryUsedMB
			})
		}
	}

	return NetworkTPS{
		TotalRxBytesPerSec: totalRx,
		TotalTxBytesPerSec: totalTx,
	}, perNode, services
}

// collectHTTPTPS gathers HTTP throughput and service stats from Traefik API.
func (c *TPSCollector) collectHTTPTPS(ctx context.Context, now time.Time) HTTPTPS {
	var httpTPS HTTPTPS

	type traefikOverview struct {
		HTTP struct {
			Routers struct {
				Total    int `json:"total"`
				Warnings int `json:"warnings"`
				Errors   int `json:"errors"`
			} `json:"routers"`
			Services struct {
				Total    int `json:"total"`
				Warnings int `json:"warnings"`
				Errors   int `json:"errors"`
			} `json:"services"`
			Middlewares struct {
				Total int `json:"total"`
			} `json:"middlewares"`
		} `json:"http"`
	}

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	var totalReqs int64
	var errorCount int64
	var activeConns int

	if c.traefikAPIURL != "" {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.traefikAPIURL+"/api/overview", nil)
		if err == nil {
			resp, doErr := client.Do(req)
			if doErr == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var ov traefikOverview
					if decodeErr := json.NewDecoder(resp.Body).Decode(&ov); decodeErr == nil {
						activeConns = ov.HTTP.Services.Total + ov.HTTP.Routers.Total
						errorCount = int64(ov.HTTP.Routers.Errors)
					}
				}
			} else {
				c.logger.Debug("Traefik overview API request failed", zap.Error(doErr))
			}
		}

		// Also check services for active upstream servers
		srvReqCtx, srvCancel := context.WithTimeout(ctx, 2*time.Second)
		defer srvCancel()
		srvReq, srvErr := http.NewRequestWithContext(srvReqCtx, http.MethodGet, c.traefikAPIURL+"/api/http/services", nil)
		if srvErr == nil {
			srvResp, srvDoErr := client.Do(srvReq)
			if srvDoErr == nil {
				defer srvResp.Body.Close()
				if srvResp.StatusCode == http.StatusOK {
					type traefikServiceItem struct {
						Status       string            `json:"status"`
						ServerStatus map[string]string `json:"serverStatus"`
					}
					var services []traefikServiceItem
					if err := json.NewDecoder(srvResp.Body).Decode(&services); err == nil {
						serverCount := 0
						for _, s := range services {
							for _, state := range s.ServerStatus {
								if strings.EqualFold(state, "UP") {
									serverCount++
								}
							}
						}
						if serverCount > 0 {
							activeConns = serverCount
						}
					}
				}
			}
		}
	}

	if c.requestCountFn != nil {
		totalReqs = c.requestCountFn()
	}

	c.mu.Lock()
	if !c.prevHTTPTime.IsZero() {
		elapsed := now.Sub(c.prevHTTPTime).Seconds()
		if elapsed > 0 {
			if totalReqs > 0 {
				reqDelta := totalReqs - c.prevHTTPStats.totalRequests
				if reqDelta < 0 {
					reqDelta = 0
				}
				httpTPS.RequestsPerSec = math.Round((float64(reqDelta)/elapsed)*100) / 100
			}

			if totalReqs > 0 && errorCount > 0 {
				httpTPS.ErrorRate = math.Round((float64(errorCount)/float64(totalReqs))*10000) / 100
			}
		}
	}
	c.prevHTTPStats = httpStatsRaw{
		totalRequests: totalReqs,
		errorRequests: errorCount,
	}
	c.prevHTTPTime = now
	c.mu.Unlock()

	httpTPS.ActiveConnections = activeConns
	httpTPS.TotalRequests = totalReqs
	return httpTPS
}

// collectDatabaseTPS queries pg_stat_database for transactional TPS and cache hit metrics.
func (c *TPSCollector) collectDatabaseTPS(ctx context.Context, now time.Time) DatabaseTPS {
	var dbTPS DatabaseTPS

	if c.dbPool == nil {
		return dbTPS
	}

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	const query = `
		SELECT 
			xact_commit, 
			xact_rollback, 
			tup_returned, 
			tup_fetched, 
			tup_inserted, 
			tup_updated, 
			tup_deleted, 
			COALESCE(numbackends, 0), 
			blks_read, 
			blks_hit 
		FROM pg_stat_database 
		WHERE datname = current_database()
	`

	var (
		xactCommit   int64
		xactRollback int64
		tupReturned  int64
		tupFetched   int64
		tupInserted  int64
		tupUpdated   int64
		tupDeleted   int64
		numBackends  int64
		blksRead     int64
		blksHit      int64
	)

	err := c.dbPool.QueryRow(queryCtx, query).Scan(
		&xactCommit, &xactRollback, &tupReturned, &tupFetched,
		&tupInserted, &tupUpdated, &tupDeleted, &numBackends,
		&blksRead, &blksHit,
	)
	if err != nil {
		c.logger.Debug("Failed to query pg_stat_database for database TPS", zap.Error(err))
		return dbTPS
	}

	var cacheHitRatio float64
	totalBlocks := blksHit + blksRead
	if totalBlocks > 0 {
		cacheHitRatio = math.Round((float64(blksHit)/float64(totalBlocks))*10000) / 10000
	}

	dbTPS.ActiveConnections = int(numBackends)
	dbTPS.CacheHitRatio = cacheHitRatio

	c.mu.Lock()
	if !c.prevDBTime.IsZero() {
		elapsed := now.Sub(c.prevDBTime).Seconds()
		if elapsed > 0 {
			commitDelta := xactCommit - c.prevDBStats.commit
			rollbackDelta := xactRollback - c.prevDBStats.rollback
			if commitDelta < 0 {
				commitDelta = 0
			}
			if rollbackDelta < 0 {
				rollbackDelta = 0
			}
			dbTPS.TransactionsPerSec = math.Round((float64(commitDelta+rollbackDelta)/elapsed)*100) / 100

			retDelta := tupReturned - c.prevDBStats.tupReturned
			fetchDelta := tupFetched - c.prevDBStats.tupFetched
			if retDelta < 0 {
				retDelta = 0
			}
			if fetchDelta < 0 {
				fetchDelta = 0
			}
			dbTPS.ReadsPerSec = math.Round((float64(retDelta+fetchDelta)/elapsed)*100) / 100

			insDelta := tupInserted - c.prevDBStats.tupInserted
			updDelta := tupUpdated - c.prevDBStats.tupUpdated
			delDelta := tupDeleted - c.prevDBStats.tupDeleted
			if insDelta < 0 {
				insDelta = 0
			}
			if updDelta < 0 {
				updDelta = 0
			}
			if delDelta < 0 {
				delDelta = 0
			}
			dbTPS.WritesPerSec = math.Round((float64(insDelta+updDelta+delDelta)/elapsed)*100) / 100
		}
	}
	c.prevDBStats = dbStatsRaw{
		commit:      xactCommit,
		rollback:    xactRollback,
		tupReturned: tupReturned,
		tupFetched:  tupFetched,
		tupInserted: tupInserted,
		tupUpdated:  tupUpdated,
		tupDeleted:  tupDeleted,
	}
	c.prevDBTime = now
	c.mu.Unlock()

	return dbTPS
}

// collectMessagingTPS calls NATS /varz to retrieve message rates and active connections.
func (c *TPSCollector) collectMessagingTPS(ctx context.Context, now time.Time) MessagingTPS {
	var msgTPS MessagingTPS

	if c.natsMonitorURL == "" {
		return msgTPS
	}

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.natsMonitorURL, nil)
	if err != nil {
		c.logger.Debug("Invalid NATS monitor URL", zap.Error(err))
		return msgTPS
	}

	resp, doErr := client.Do(req)
	if doErr != nil {
		c.logger.Debug("NATS monitoring endpoint unreachable", zap.String("url", c.natsMonitorURL), zap.Error(doErr))
		return msgTPS
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Debug("NATS monitoring returned non-200 status", zap.Int("status", resp.StatusCode))
		return msgTPS
	}

	type natsVarzPayload struct {
		InMsgs      int64   `json:"in_msgs"`
		OutMsgs     int64   `json:"out_msgs"`
		InBytes     int64   `json:"in_bytes"`
		OutBytes    int64   `json:"out_bytes"`
		InRate      float64 `json:"in_rate"`
		OutRate     float64 `json:"out_rate"`
		Connections int     `json:"connections"`
		JetStream   *struct {
			Stats struct {
				Streams int `json:"streams"`
			} `json:"stats"`
		} `json:"jetstream,omitempty"`
	}

	var payload natsVarzPayload
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		c.logger.Debug("Failed to decode NATS varz response", zap.Error(decodeErr))
		return msgTPS
	}

	msgTPS.Connections = payload.Connections
	msgTPS.InMsgsPerSec = math.Round(payload.InRate*100) / 100
	msgTPS.OutMsgsPerSec = math.Round(payload.OutRate*100) / 100
	if payload.JetStream != nil {
		msgTPS.Streams = payload.JetStream.Stats.Streams
	}

	c.mu.Lock()
	if !c.prevNATSTime.IsZero() {
		elapsed := now.Sub(c.prevNATSTime).Seconds()
		if elapsed > 0 {
			inBytesDelta := payload.InBytes - c.prevNATSStats.inBytes
			outBytesDelta := payload.OutBytes - c.prevNATSStats.outBytes
			if inBytesDelta < 0 {
				inBytesDelta = 0
			}
			if outBytesDelta < 0 {
				outBytesDelta = 0
			}
			msgTPS.InBytesPerSec = int64(float64(inBytesDelta) / elapsed)
			msgTPS.OutBytesPerSec = int64(float64(outBytesDelta) / elapsed)

			if msgTPS.InMsgsPerSec == 0 && payload.InMsgs > c.prevNATSStats.inMsgs {
				msgTPS.InMsgsPerSec = math.Round((float64(payload.InMsgs-c.prevNATSStats.inMsgs)/elapsed)*100) / 100
			}
			if msgTPS.OutMsgsPerSec == 0 && payload.OutMsgs > c.prevNATSStats.outMsgs {
				msgTPS.OutMsgsPerSec = math.Round((float64(payload.OutMsgs-c.prevNATSStats.outMsgs)/elapsed)*100) / 100
			}
		}
	}
	c.prevNATSStats = natsStatsRaw{
		inMsgs:   payload.InMsgs,
		outMsgs:  payload.OutMsgs,
		inBytes:  payload.InBytes,
		outBytes: payload.OutBytes,
	}
	c.prevNATSTime = now
	c.mu.Unlock()

	return msgTPS
}

// DeriveTraefikURL parses the Docker host address or returns a default HTTP URL for Traefik API.
func DeriveTraefikURL(dockerHost string) string {
	dockerHost = strings.TrimSpace(dockerHost)
	if dockerHost == "" {
		return "http://localhost:8080"
	}
	if strings.HasPrefix(dockerHost, "http://") || strings.HasPrefix(dockerHost, "https://") {
		return dockerHost
	}
	if strings.HasPrefix(dockerHost, "tcp://") {
		trimmed := strings.TrimPrefix(dockerHost, "tcp://")
		host := trimmed
		if idx := strings.Index(trimmed, ":"); idx != -1 {
			host = trimmed[:idx]
		}
		if host == "" || host == "0.0.0.0" {
			host = "localhost"
		}
		return "http://" + host + ":8080"
	}
	return "http://localhost:8080"
}

// DeriveNATSMonitorURL parses the NATS connection URL and produces the HTTP varz monitoring endpoint.
func DeriveNATSMonitorURL(natsURL string) string {
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		return "http://localhost:8222/varz"
	}
	if strings.HasPrefix(natsURL, "http://") || strings.HasPrefix(natsURL, "https://") {
		if strings.HasSuffix(natsURL, "/varz") {
			return natsURL
		}
		return strings.TrimRight(natsURL, "/") + "/varz"
	}

	var host string
	if u, err := url.Parse(natsURL); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	} else {
		cleaned := strings.TrimPrefix(natsURL, "nats://")
		if idx := strings.Index(cleaned, ":"); idx != -1 {
			host = cleaned[:idx]
		} else {
			host = cleaned
		}
	}

	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	return "http://" + host + ":8222/varz"
}

// extractServiceName retrieves or derives the service name for a container.
func extractServiceName(cm ContainerMetrics) string {
	if cm.ServiceName != "" {
		return cm.ServiceName
	}
	if cm.Labels != nil {
		if sn, ok := cm.Labels["com.docker.swarm.service.name"]; ok && sn != "" {
			return sn
		}
		if sn, ok := cm.Labels["com.docker.compose.service"]; ok && sn != "" {
			return sn
		}
	}
	name := strings.TrimPrefix(cm.ContainerName, "/")
	if name == "" {
		name = cm.ContainerID
		if len(name) > 12 {
			name = name[:12]
		}
		return name
	}
	if parts := strings.Split(name, "."); len(parts) >= 3 {
		return parts[0]
	}
	return name
}

