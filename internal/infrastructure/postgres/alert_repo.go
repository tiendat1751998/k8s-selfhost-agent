package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
)

type AlertRepo struct {
	db DBTX
}

func NewAlertRepo(db DBTX) *AlertRepo {
	return &AlertRepo{db: db}
}

func (r *AlertRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *AlertRepo) CreateChannel(ctx context.Context, channel *alert.NotificationChannel) error {
	if channel.ID == "" {
		channel.ID = uuid.NewString()
	}
	now := time.Now()
	channel.CreatedAt = now
	channel.UpdatedAt = now

	configJSON, err := json.Marshal(channel.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO notification_channels (id, tenant_id, name, type, config, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		channel.ID,
		channel.TenantID,
		channel.Name,
		channel.Type,
		configJSON,
		channel.Enabled,
		channel.CreatedAt,
		channel.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert notification channel: %w", err)
	}
	return nil
}

func (r *AlertRepo) ListChannels(ctx context.Context, tenantID string) ([]*alert.NotificationChannel, error) {
	query := `
		SELECT id, tenant_id, name, type, config, enabled, created_at, updated_at
		FROM notification_channels
		WHERE tenant_id = $1
	`

	rows, err := r.getDB(ctx).Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query notification channels: %w", err)
	}
	defer rows.Close()

	var channels []*alert.NotificationChannel
	for rows.Next() {
		var c alert.NotificationChannel
		var configBytes []byte
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.Name, &c.Type, &configBytes, &c.Enabled, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan notification channel: %w", err)
		}
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &c.Config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal config: %w", err)
			}
		}
		channels = append(channels, &c)
	}
	return channels, nil
}

func (r *AlertRepo) CreateRule(ctx context.Context, rule *alert.AlertRule) error {
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	channelsJSON, err := json.Marshal(rule.ChannelIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal channel_ids: %w", err)
	}

	query := `
		INSERT INTO alert_rules (id, tenant_id, name, description, metric_name, condition, threshold, duration_seconds, severity, channel_ids, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		rule.ID,
		rule.TenantID,
		rule.Name,
		rule.Description,
		rule.MetricName,
		rule.Condition,
		rule.Threshold,
		rule.DurationSeconds,
		rule.Severity,
		channelsJSON,
		rule.Enabled,
		rule.CreatedAt,
		rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert alert rule: %w", err)
	}
	return nil
}

func (r *AlertRepo) ListRules(ctx context.Context, tenantID string) ([]*alert.AlertRule, error) {
	query := `
		SELECT id, tenant_id, name, description, metric_name, condition, threshold, duration_seconds, severity, channel_ids, enabled, created_at, updated_at
		FROM alert_rules
		WHERE tenant_id = $1
	`

	rows, err := r.getDB(ctx).Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert rules: %w", err)
	}
	defer rows.Close()

	var rules []*alert.AlertRule
	for rows.Next() {
		var rule alert.AlertRule
		var channelsBytes []byte
		if err := rows.Scan(
			&rule.ID, &rule.TenantID, &rule.Name, &rule.Description, &rule.MetricName, &rule.Condition, &rule.Threshold, &rule.DurationSeconds, &rule.Severity, &channelsBytes, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		if len(channelsBytes) > 0 {
			if err := json.Unmarshal(channelsBytes, &rule.ChannelIDs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal channel_ids: %w", err)
			}
		}
		rules = append(rules, &rule)
	}
	return rules, nil
}

func (r *AlertRepo) UpdateRule(ctx context.Context, rule *alert.AlertRule) error {
	rule.UpdatedAt = time.Now()
	channelsJSON, err := json.Marshal(rule.ChannelIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal channel_ids: %w", err)
	}
	query := `
		UPDATE alert_rules
		SET name = $1, description = $2, metric_name = $3, condition = $4, threshold = $5, duration_seconds = $6, severity = $7, channel_ids = $8, enabled = $9, updated_at = $10
		WHERE id = $11 AND tenant_id = $12
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		rule.Name, rule.Description, rule.MetricName, rule.Condition, rule.Threshold, rule.DurationSeconds, rule.Severity, channelsJSON, rule.Enabled, rule.UpdatedAt,
		rule.ID, rule.TenantID,
	)
	if err != nil {
		return fmt.Errorf("failed to update alert rule: %w", err)
	}
	return nil
}

func (r *AlertRepo) DeleteRule(ctx context.Context, id, tenantID string) error {
	query := `DELETE FROM alert_rules WHERE id = $1 AND tenant_id = $2`
	_, err := r.getDB(ctx).Exec(ctx, query, id, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete alert rule: %w", err)
	}
	return nil
}

func (r *AlertRepo) CreateHistory(ctx context.Context, history *alert.AlertHistory) error {
	if history.ID == "" {
		history.ID = uuid.NewString()
	}
	now := time.Now()
	history.CreatedAt = now
	history.UpdatedAt = now
	query := `
		INSERT INTO alert_history (id, tenant_id, rule_id, status, value, message, acknowledged_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.getDB(ctx).Exec(ctx, query,
		history.ID, history.TenantID, history.RuleID, history.Status, history.Value, history.Message, history.AcknowledgedBy, history.CreatedAt, history.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert alert history: %w", err)
	}
	return nil
}

func (r *AlertRepo) ListHistory(ctx context.Context, tenantID string) ([]*alert.AlertHistory, error) {
	query := `
		SELECT id, tenant_id, rule_id, status, value, message, acknowledged_by, created_at, updated_at
		FROM alert_history
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT 500
	`

	rows, err := r.getDB(ctx).Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert history: %w", err)
	}
	defer rows.Close()

	var histories []*alert.AlertHistory
	for rows.Next() {
		var h alert.AlertHistory
		var ackBy *string
		if err := rows.Scan(
			&h.ID, &h.TenantID, &h.RuleID, &h.Status, &h.Value, &h.Message, &ackBy, &h.CreatedAt, &h.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan alert history: %w", err)
		}
		if ackBy != nil {
			h.AcknowledgedBy = *ackBy
		}
		histories = append(histories, &h)
	}
	return histories, nil
}

func (r *AlertRepo) AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error {
	query := `
		UPDATE alert_history
		SET status = 'acknowledged', acknowledged_by = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`

	_, err := r.getDB(ctx).Exec(ctx, query, userID, time.Now(), id, tenantID)
	if err != nil {
		return fmt.Errorf("failed to acknowledge alert: %w", err)
	}
	return nil
}

func (r *AlertRepo) ResolveAlert(ctx context.Context, id, tenantID, userID string) error {
	now := time.Now().UTC()
	query := `
		UPDATE alert_history
		SET status = 'resolved', acknowledged_by = $1, resolved_at = $2, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`
	_, err := r.getDB(ctx).Exec(ctx, query, userID, now, id, tenantID)
	if err != nil {
		fallbackQuery := `
			UPDATE alert_history
			SET status = 'resolved', acknowledged_by = $1, updated_at = $2
			WHERE id = $3 AND tenant_id = $4
		`
		if _, fbErr := r.getDB(ctx).Exec(ctx, fallbackQuery, userID, now, id, tenantID); fbErr != nil {
			return fmt.Errorf("failed to resolve alert: %w", fbErr)
		}
	}
	return nil
}

func (r *AlertRepo) CreateSilence(ctx context.Context, s *alert.Silence) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.StartsAt.IsZero() {
		s.StartsAt = time.Now().UTC()
	}
	if s.EndsAt.IsZero() {
		dur := s.DurationMinutes
		if dur <= 0 {
			dur = 60
		}
		s.EndsAt = s.StartsAt.Add(time.Duration(dur) * time.Minute)
	}
	s.CreatedAt = time.Now().UTC()

	query := `
		INSERT INTO alert_silences (id, tenant_id, alert_id, comment, created_by, duration_minutes, starts_at, ends_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			comment = EXCLUDED.comment,
			duration_minutes = EXCLUDED.duration_minutes,
			starts_at = EXCLUDED.starts_at,
			ends_at = EXCLUDED.ends_at
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		s.ID, s.TenantID, s.AlertID, s.Comment, s.CreatedBy, s.DurationMinutes, s.StartsAt, s.EndsAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert silence: %w", err)
	}
	return nil
}

func (r *AlertRepo) ListSilences(ctx context.Context, tenantID string) ([]*alert.Silence, error) {
	query := `
		SELECT id, tenant_id, alert_id, comment, created_by, duration_minutes, starts_at, ends_at, created_at
		FROM alert_silences
		WHERE tenant_id = $1 AND ends_at > NOW()
		ORDER BY created_at DESC
	`
	rows, err := r.getDB(ctx).Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list silences: %w", err)
	}
	defer rows.Close()

	var silences []*alert.Silence
	for rows.Next() {
		var s alert.Silence
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.AlertID, &s.Comment, &s.CreatedBy, &s.DurationMinutes, &s.StartsAt, &s.EndsAt, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan silence: %w", err)
		}
		silences = append(silences, &s)
	}
	return silences, nil
}

func (r *AlertRepo) IsAlertSilenced(ctx context.Context, tenantID, alertID string) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM alert_silences
		WHERE tenant_id = $1 AND alert_id = $2 AND starts_at <= NOW() AND ends_at > NOW()
	`
	var count int
	if err := r.getDB(ctx).QueryRow(ctx, query, tenantID, alertID).Scan(&count); err != nil {
		return false, fmt.Errorf("checking alert silence: %w", err)
	}
	return count > 0, nil
}

