package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	domainconsensus "github.com/datdt/k8sselfhost/internal/domain/consensus"
)

type consensusLeaseRepo struct {
	db DBTX
}

// NewConsensusLeaseRepo constructs a PostgreSQL-backed LeaseStore repository.
func NewConsensusLeaseRepo(db DBTX) domainconsensus.LeaseStore {
	return &consensusLeaseRepo{db: db}
}

func (r *consensusLeaseRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

const upsertConsensusLeaseSQL = `
INSERT INTO consensus_leases (cluster_id, holder_id, fencing_token, acquired_at, expires_at)
VALUES ($1, $2, 1, NOW(), $3)
ON CONFLICT (cluster_id) DO UPDATE
SET holder_id = EXCLUDED.holder_id,
    fencing_token = consensus_leases.fencing_token + 1,
    acquired_at = NOW(),
    expires_at = EXCLUDED.expires_at
WHERE consensus_leases.expires_at <= NOW() OR consensus_leases.holder_id = EXCLUDED.holder_id
RETURNING holder_id, fencing_token, acquired_at, expires_at;
`

// AcquireOrRenewLease atomically acquires or renews a distributed cluster fencing lease.
func (r *consensusLeaseRepo) AcquireOrRenewLease(
	ctx context.Context,
	clusterID, holderID string,
	ttl time.Duration,
) (*domainconsensus.FencingLease, error) {
	if clusterID == "" || holderID == "" {
		return nil, fmt.Errorf("%w: clusterID and holderID cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: ttl must be positive", domainconsensus.ErrInvalidConsensusInput)
	}

	expiresAt := time.Now().UTC().Add(ttl)
	var holder string
	var token int64
	var acquiredAt, scannedExpiresAt time.Time

	row := r.getDB(ctx).QueryRow(ctx, upsertConsensusLeaseSQL, clusterID, holderID, expiresAt)
	err := row.Scan(&holder, &token, &acquiredAt, &scannedExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainconsensus.ErrLeaseConflict
		}
		return nil, fmt.Errorf("acquiring or renewing consensus lease for cluster %s: %w", clusterID, err)
	}

	return domainconsensus.NewFencingLease(clusterID, holder, token, acquiredAt, scannedExpiresAt)
}

const getActivePrimarySQL = `
SELECT holder_id, fencing_token
FROM consensus_leases
WHERE cluster_id = $1;
`

// GetActivePrimary returns the current lease holder and fencing token for a cluster.
func (r *consensusLeaseRepo) GetActivePrimary(ctx context.Context, clusterID string) (string, int64, error) {
	if clusterID == "" {
		return "", 0, fmt.Errorf("%w: clusterID cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}

	var holder string
	var token int64
	err := r.getDB(ctx).QueryRow(ctx, getActivePrimarySQL, clusterID).Scan(&holder, &token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("getting active primary for cluster %s: %w", clusterID, err)
	}

	return holder, token, nil
}

const validateFencingTokenSQL = `
SELECT fencing_token, expires_at
FROM consensus_leases
WHERE cluster_id = $1;
`

// ValidateToken validates that a fencing token matches the active lease and has not expired.
func (r *consensusLeaseRepo) ValidateToken(ctx context.Context, clusterID string, token int64) (bool, error) {
	if clusterID == "" {
		return false, fmt.Errorf("%w: clusterID cannot be empty", domainconsensus.ErrInvalidConsensusInput)
	}
	if token <= 0 {
		return false, fmt.Errorf("%w: token must be greater than zero", domainconsensus.ErrInvalidConsensusInput)
	}

	var currentToken int64
	var expiresAt time.Time
	err := r.getDB(ctx).QueryRow(ctx, validateFencingTokenSQL, clusterID).Scan(&currentToken, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, domainconsensus.ErrFencingTokenStale
		}
		return false, fmt.Errorf("validating fencing token for cluster %s: %w", clusterID, err)
	}

	if token != currentToken {
		return false, domainconsensus.ErrFencingTokenStale
	}
	if time.Now().UTC().After(expiresAt) {
		return false, fmt.Errorf("%w: lock lease has expired", domainconsensus.ErrFencingTokenStale)
	}

	return true, nil
}
