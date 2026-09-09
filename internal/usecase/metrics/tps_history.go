package metrics

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

type dbStatsRaw struct {
	commit      int64
	rollback    int64
	tupReturned int64
	tupFetched  int64
	tupInserted int64
	tupUpdated  int64
	tupDeleted  int64
	blksRead    int64
	blksHit     int64
}

type natsStatsRaw struct {
	inMsgs   int64
	outMsgs  int64
	inBytes  int64
	outBytes int64
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
			commitDelta := safeDeltaInt64(xactCommit, c.prevDBStats.commit)
			rollbackDelta := safeDeltaInt64(xactRollback, c.prevDBStats.rollback)
			dbTPS.TransactionsPerSec = math.Round((float64(commitDelta+rollbackDelta)/elapsed)*100) / 100

			retDelta := safeDeltaInt64(tupReturned, c.prevDBStats.tupReturned)
			fetchDelta := safeDeltaInt64(tupFetched, c.prevDBStats.tupFetched)
			dbTPS.ReadsPerSec = math.Round((float64(retDelta+fetchDelta)/elapsed)*100) / 100

			insDelta := safeDeltaInt64(tupInserted, c.prevDBStats.tupInserted)
			updDelta := safeDeltaInt64(tupUpdated, c.prevDBStats.tupUpdated)
			delDelta := safeDeltaInt64(tupDeleted, c.prevDBStats.tupDeleted)
			dbTPS.WritesPerSec = math.Round((float64(insDelta+updDelta+delDelta)/elapsed)*100) / 100

			deltaBlksHit := safeDeltaInt64(blksHit, c.prevDBStats.blksHit)
			deltaBlksRead := safeDeltaInt64(blksRead, c.prevDBStats.blksRead)
			deltaTotalBlocks := deltaBlksHit + deltaBlksRead
			if deltaTotalBlocks > 0 {
				dbTPS.CacheHitRatio = math.Round((float64(deltaBlksHit)/float64(deltaTotalBlocks))*10000) / 10000
			}
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
		blksRead:    blksRead,
		blksHit:     blksHit,
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
			inBytesDelta := safeDeltaInt64(payload.InBytes, c.prevNATSStats.inBytes)
			outBytesDelta := safeDeltaInt64(payload.OutBytes, c.prevNATSStats.outBytes)
			msgTPS.InBytesPerSec = int64(float64(inBytesDelta) / elapsed)
			msgTPS.OutBytesPerSec = int64(float64(outBytesDelta) / elapsed)

			inMsgsDelta := safeDeltaInt64(payload.InMsgs, c.prevNATSStats.inMsgs)
			outMsgsDelta := safeDeltaInt64(payload.OutMsgs, c.prevNATSStats.outMsgs)

			if msgTPS.InMsgsPerSec == 0 && inMsgsDelta > 0 {
				msgTPS.InMsgsPerSec = math.Round((float64(inMsgsDelta)/elapsed)*100) / 100
			}
			if msgTPS.OutMsgsPerSec == 0 && outMsgsDelta > 0 {
				msgTPS.OutMsgsPerSec = math.Round((float64(outMsgsDelta)/elapsed)*100) / 100
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
