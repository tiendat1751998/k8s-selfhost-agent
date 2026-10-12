package database

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/database"
)

// RedisClient defines the subset of redis commands required for SRE defragmentation operations.
type RedisClient interface {
	Info(ctx context.Context, sections ...string) *redis.StringCmd
	ConfigGet(ctx context.Context, parameter string) *redis.MapStringStringCmd
	ConfigSet(ctx context.Context, parameter, value string) *redis.StatusCmd
}

// ClusterRedisResolver resolves a RedisClient for a specific cluster.
type ClusterRedisResolver func(ctx context.Context, clusterID string) (RedisClient, error)

// RedisSREManager handles Redis memory fragmentation analysis and active defrag control.
type RedisSREManager struct {
	defaultClient RedisClient
	resolver      ClusterRedisResolver
	logger        *zap.Logger
}

// NewRedisSREManager creates a new RedisSREManager instance.
func NewRedisSREManager(defaultClient RedisClient, resolver ClusterRedisResolver, logger *zap.Logger) *RedisSREManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RedisSREManager{
		defaultClient: defaultClient,
		resolver:      resolver,
		logger:        logger,
	}
}

func (m *RedisSREManager) getClient(ctx context.Context, clusterID string) (RedisClient, error) {
	if m.resolver != nil {
		cli, err := m.resolver(ctx, clusterID)
		if err == nil && cli != nil {
			return cli, nil
		}
	}
	if m.defaultClient != nil {
		return m.defaultClient, nil
	}
	return nil, fmt.Errorf("no Redis connection available for cluster %q", clusterID)
}

// GetRedisFragmentation queries INFO memory and CONFIG GET activedefrag to analyze fragmentation.
func (m *RedisSREManager) GetRedisFragmentation(ctx context.Context, clusterID string) (*database.RedisFragmentationStatus, error) {
	rdb, err := m.getClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving redis client: %w", err)
	}

	infoStr, err := rdb.Info(ctx, "memory").Result()
	if err != nil {
		return nil, fmt.Errorf("querying redis INFO memory: %w", err)
	}

	usedMemory, rssMemory, infoRatio, defragRunning, err := parseRedisMemoryInfo(infoStr)
	if err != nil {
		return nil, fmt.Errorf("parsing redis memory info: %w", err)
	}

	var activeDefragEnabled bool
	cfgMap, err := rdb.ConfigGet(ctx, "activedefrag").Result()
	if err == nil && cfgMap != nil {
		if val, ok := cfgMap["activedefrag"]; ok {
			activeDefragEnabled = strings.EqualFold(val, "yes")
		}
	}

	status, err := database.NewRedisFragmentationStatus(usedMemory, rssMemory, activeDefragEnabled, defragRunning)
	if err != nil {
		return nil, fmt.Errorf("building redis fragmentation status: %w", err)
	}

	// If Redis provided a specific ratio in INFO, prefer it if valid and non-zero
	if infoRatio > 0 {
		status.FragmentationRatio = infoRatio
	}

	return status, nil
}

// ToggleRedisActiveDefrag enables or disables active defragmentation and tunes parameters.
func (m *RedisSREManager) ToggleRedisActiveDefrag(ctx context.Context, clusterID string, enable bool) error {
	rdb, err := m.getClient(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("resolving redis client: %w", err)
	}

	if enable {
		if err := rdb.ConfigSet(ctx, "activedefrag", "yes").Err(); err != nil {
			return fmt.Errorf("enabling activedefrag: %w", err)
		}
		// Tune active defrag parameters per spec
		if err := rdb.ConfigSet(ctx, "active-defrag-ignore-bytes", "100mb").Err(); err != nil {
			m.logger.Warn("failed to set active-defrag-ignore-bytes", zap.String("cluster", clusterID), zap.Error(err))
		}
		if err := rdb.ConfigSet(ctx, "active-defrag-threshold-lower", "10").Err(); err != nil {
			m.logger.Warn("failed to set active-defrag-threshold-lower", zap.String("cluster", clusterID), zap.Error(err))
		}

		m.logger.Info("auto-enabled Redis active defragmentation", zap.String("cluster", clusterID))
	} else {
		if err := rdb.ConfigSet(ctx, "activedefrag", "no").Err(); err != nil {
			return fmt.Errorf("disabling activedefrag: %w", err)
		}

		m.logger.Info("auto-disabled Redis active defragmentation", zap.String("cluster", clusterID))
	}

	return nil
}

// parseRedisMemoryInfo extracts memory and defrag metrics from INFO memory raw output.
func parseRedisMemoryInfo(infoStr string) (int64, int64, float64, bool, error) {
	var (
		usedMemory    int64
		rssMemory     int64
		ratio         float64
		defragRunning bool
	)

	scanner := bufio.NewScanner(strings.NewReader(infoStr))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "used_memory":
			if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
				usedMemory = parsed
			}
		case "used_memory_rss":
			if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
				rssMemory = parsed
			}
		case "mem_fragmentation_ratio":
			if parsed, err := strconv.ParseFloat(val, 64); err == nil {
				ratio = parsed
			}
		case "active_defrag_running":
			defragRunning = val == "1"
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, 0, false, fmt.Errorf("scanning info memory: %w", err)
	}

	return usedMemory, rssMemory, ratio, defragRunning, nil
}

// CompositeDatabaseSREManager satisfies database.DatabaseSREManager by uniting PostgreSQL and Redis SRE operations.
type CompositeDatabaseSREManager struct {
	postgres *PostgresSREManager
	redis    *RedisSREManager
}

// NewCompositeDatabaseSREManager constructs a unified DatabaseSREManager.
func NewCompositeDatabaseSREManager(pg *PostgresSREManager, rdb *RedisSREManager) *CompositeDatabaseSREManager {
	return &CompositeDatabaseSREManager{
		postgres: pg,
		redis:    rdb,
	}
}

// Compile-time interface assertion
var _ database.DatabaseSREManager = (*CompositeDatabaseSREManager)(nil)

func (c *CompositeDatabaseSREManager) InspectIdleTransactions(ctx context.Context, clusterID, dbName string) ([]database.IdleTransactionRecord, error) {
	if c.postgres == nil {
		return nil, errors.New("postgres SRE manager is unconfigured")
	}
	return c.postgres.InspectIdleTransactions(ctx, clusterID, dbName)
}

func (c *CompositeDatabaseSREManager) TerminateIdleTransaction(ctx context.Context, clusterID, dbName string, pid int) error {
	if c.postgres == nil {
		return errors.New("postgres SRE manager is unconfigured")
	}
	return c.postgres.TerminateIdleTransaction(ctx, clusterID, dbName, pid)
}

func (c *CompositeDatabaseSREManager) AnalyzeTableBloat(ctx context.Context, clusterID, dbName string) ([]database.TableBloatReport, error) {
	if c.postgres == nil {
		return nil, errors.New("postgres SRE manager is unconfigured")
	}
	return c.postgres.AnalyzeTableBloat(ctx, clusterID, dbName)
}

func (c *CompositeDatabaseSREManager) GetRedisFragmentation(ctx context.Context, clusterID string) (*database.RedisFragmentationStatus, error) {
	if c.redis == nil {
		return nil, errors.New("redis SRE manager is unconfigured")
	}
	return c.redis.GetRedisFragmentation(ctx, clusterID)
}

func (c *CompositeDatabaseSREManager) ToggleRedisActiveDefrag(ctx context.Context, clusterID string, enable bool) error {
	if c.redis == nil {
		return errors.New("redis SRE manager is unconfigured")
	}
	return c.redis.ToggleRedisActiveDefrag(ctx, clusterID, enable)
}
