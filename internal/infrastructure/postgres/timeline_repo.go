package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/timeline"
)

type timelineRepo struct {
	db DBTX
}

// NewTimelineRepo creates a new Postgres-backed Timeline repository.
func NewTimelineRepo(db DBTX) timeline.Repository {
	return &timelineRepo{db: db}
}

func (r *timelineRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *timelineRepo) List(ctx context.Context, eventType *timeline.EventType, since time.Time, limit, offset int) ([]timeline.Event, int, error) {
	var query string
	var countQuery string
	var args []interface{}
	var countArgs []interface{}
	
	if eventType != nil {
		query = `
			SELECT id, type, title, detail, namespace, cluster, metadata, created_at 
			FROM timeline_events 
			WHERE created_at >= $1 AND type = $2
			ORDER BY created_at DESC 
			LIMIT $3 OFFSET $4
		`
		countQuery = `SELECT COUNT(*) FROM timeline_events WHERE created_at >= $1 AND type = $2`
		args = []interface{}{since, string(*eventType), limit, offset}
		countArgs = []interface{}{since, string(*eventType)}
	} else {
		query = `
			SELECT id, type, title, detail, namespace, cluster, metadata, created_at 
			FROM timeline_events 
			WHERE created_at >= $1 
			ORDER BY created_at DESC 
			LIMIT $2 OFFSET $3
		`
		countQuery = `SELECT COUNT(*) FROM timeline_events WHERE created_at >= $1`
		args = []interface{}{since, limit, offset}
		countArgs = []interface{}{since}
	}

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting timeline events: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying timeline events: %w", err)
	}
	defer rows.Close()

	var events []timeline.Event
	for rows.Next() {
		var e timeline.Event
		var metaBytes []byte
		var ns, cl *string

		if err := rows.Scan(
			&e.ID, &e.Type, &e.Title, &e.Detail, &ns, &cl, &metaBytes, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning timeline event: %w", err)
		}
		
		if ns != nil {
			e.Namespace = *ns
		}
		if cl != nil {
			e.Cluster = *cl
		}
		
		if len(metaBytes) > 0 {
			if err := json.Unmarshal(metaBytes, &e.Metadata); err != nil {
				return nil, 0, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		} else {
			e.Metadata = make(map[string]string)
		}

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating timeline events: %w", err)
	}

	return events, total, nil
}

func (r *timelineRepo) GetByID(ctx context.Context, id string) (*timeline.Event, error) {
	query := `
		SELECT id, type, title, detail, namespace, cluster, metadata, created_at 
		FROM timeline_events 
		WHERE id = $1
	`
	var e timeline.Event
	var metaBytes []byte
	var ns, cl *string

	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&e.ID, &e.Type, &e.Title, &e.Detail, &ns, &cl, &metaBytes, &e.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("timeline event not found")
		}
		return nil, fmt.Errorf("querying timeline event: %w", err)
	}

	if ns != nil {
		e.Namespace = *ns
	}
	if cl != nil {
		e.Cluster = *cl
	}

	if len(metaBytes) > 0 {
		if err := json.Unmarshal(metaBytes, &e.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	} else {
		e.Metadata = make(map[string]string)
	}

	return &e, nil
}

func (r *timelineRepo) Create(ctx context.Context, e *timeline.Event) error {
	query := `
		INSERT INTO timeline_events (type, title, detail, namespace, cluster, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	
	metaBytes, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	var ns, cl *string
	if e.Namespace != "" {
		ns = &e.Namespace
	}
	if e.Cluster != "" {
		cl = &e.Cluster
	}

	err = r.getDB(ctx).QueryRow(ctx, query,
		string(e.Type), e.Title, e.Detail, ns, cl, metaBytes, e.CreatedAt,
	).Scan(&e.ID)
	
	if err != nil {
		return fmt.Errorf("inserting timeline event: %w", err)
	}
	
	return nil
}
