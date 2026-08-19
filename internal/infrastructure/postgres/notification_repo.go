package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/notification"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type notificationRepo struct {
	db DBTX
}

// NewNotificationRepo creates a new Postgres-backed Notification repository.
func NewNotificationRepo(db DBTX) notification.Repository {
	return &notificationRepo{db: db}
}

func (r *notificationRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *notificationRepo) ListChannels(ctx context.Context) ([]notification.Channel, error) {
	query := `
		SELECT id, type, name, webhook_url, config, enabled, created_at, updated_at 
		FROM notification_channels 
		ORDER BY name ASC
	`
	query, args := BuildTenantQuery(ctx, query)
	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying notification channels: %w", err)
	}
	defer rows.Close()

	var channels []notification.Channel
	for rows.Next() {
		var ch notification.Channel
		var configBytes []byte
		var webhookURL *string

		if err := rows.Scan(
			&ch.ID, &ch.Type, &ch.Name, &webhookURL, &configBytes, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning notification channel: %w", err)
		}
		
		if webhookURL != nil {
			ch.WebhookURL = *webhookURL
		}
		
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &ch.Config); err != nil {
				return nil, fmt.Errorf("unmarshaling config: %w", err)
			}
		} else {
			ch.Config = make(map[string]string)
		}

		channels = append(channels, ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating notification channels: %w", err)
	}

	return channels, nil
}

func (r *notificationRepo) CreateChannel(ctx context.Context, ch *notification.Channel) error {
	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = "default"
	}

	query := `
		INSERT INTO notification_channels (type, name, webhook_url, config, enabled, tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	
	configBytes, err := json.Marshal(ch.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	
	var webhookURL *string
	if ch.WebhookURL != "" {
		webhookURL = &ch.WebhookURL
	}

	err = r.getDB(ctx).QueryRow(ctx, query,
		string(ch.Type), ch.Name, webhookURL, configBytes, ch.Enabled, tenantID, ch.CreatedAt, ch.UpdatedAt,
	).Scan(&ch.ID)
	
	if err != nil {
		return fmt.Errorf("inserting notification channel: %w", err)
	}
	
	return nil
}

func (r *notificationRepo) UpdateChannel(ctx context.Context, ch *notification.Channel) error {
	ch.UpdatedAt = time.Now()
	query := `
		UPDATE notification_channels 
		SET type = $1, name = $2, webhook_url = $3, config = $4, enabled = $5, updated_at = $6
		WHERE id = $7
	`
	
	configBytes, err := json.Marshal(ch.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	
	var webhookURL *string
	if ch.WebhookURL != "" {
		webhookURL = &ch.WebhookURL
	}

	query, args := BuildTenantQuery(ctx, query,
		string(ch.Type), ch.Name, webhookURL, configBytes, ch.Enabled, ch.UpdatedAt, ch.ID,
	)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating notification channel: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("notification channel not found")
	}
	return nil
}

func (r *notificationRepo) DeleteChannel(ctx context.Context, id string) error {
	query := `DELETE FROM notification_channels WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)
	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting notification channel: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("notification channel not found")
	}
	return nil
}

func (r *notificationRepo) ListNotifications(ctx context.Context, limit, offset int) ([]notification.Notification, int, error) {
	// List unread notifications
	query := `
		SELECT id, channel_id, title, severity, message, status, error_detail, is_read, created_at 
		FROM notifications 
		WHERE is_read = false
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2
	`
	countQuery := `SELECT COUNT(*) FROM notifications WHERE is_read = false`

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting notifications: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying notifications: %w", err)
	}
	defer rows.Close()

	return r.scanNotifications(rows), total, nil
}

func (r *notificationRepo) ListHistory(ctx context.Context, limit, offset int) ([]notification.Notification, int, error) {
	// List all notifications
	query := `
		SELECT id, channel_id, title, severity, message, status, error_detail, is_read, created_at 
		FROM notifications 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2
	`
	countQuery := `SELECT COUNT(*) FROM notifications`

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting notifications history: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying notifications history: %w", err)
	}
	defer rows.Close()

	return r.scanNotifications(rows), total, nil
}

func (r *notificationRepo) scanNotifications(rows pgx.Rows) []notification.Notification {
	var notifs []notification.Notification
	for rows.Next() {
		var n notification.Notification
		var channelID *string
		var errDetail *string

		if err := rows.Scan(
			&n.ID, &channelID, &n.Title, &n.Severity, &n.Message, 
			&n.Status, &errDetail, &n.Read, &n.CreatedAt,
		); err != nil {
			continue // skip error rows for now
		}
		
		if channelID != nil {
			n.ChannelID = *channelID
		}
		if errDetail != nil {
			n.Error = *errDetail
		}

		notifs = append(notifs, n)
	}
	return notifs
}

func (r *notificationRepo) MarkRead(ctx context.Context, id string) error {
	query := `UPDATE notifications SET is_read = true WHERE id = $1`
	cmd, err := r.getDB(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("marking notification read: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *notificationRepo) MarkAllRead(ctx context.Context) error {
	query := `UPDATE notifications SET is_read = true WHERE is_read = false`
	_, err := r.getDB(ctx).Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("marking all notifications read: %w", err)
	}
	return nil
}

func (r *notificationRepo) CreateNotification(ctx context.Context, n *notification.Notification) error {
	query := `
		INSERT INTO notifications (channel_id, title, severity, message, status, error_detail, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	
	var channelID *string
	if n.ChannelID != "" {
		channelID = &n.ChannelID
	}
	var errDetail *string
	if n.Error != "" {
		errDetail = &n.Error
	}

	err := r.getDB(ctx).QueryRow(ctx, query,
		channelID, n.Title, string(n.Severity), n.Message, string(n.Status), errDetail, n.Read, n.CreatedAt,
	).Scan(&n.ID)
	
	if err != nil {
		return fmt.Errorf("inserting notification: %w", err)
	}
	
	return nil
}
