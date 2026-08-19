package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/cloud"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type cloudAccountRepo struct {
	pool DBTX
}

// NewCloudAccountRepo creates a new PostgreSQL-backed Cloud Account repository.
func NewCloudAccountRepo(pool DBTX) cloud.AccountRepository {
	return &cloudAccountRepo{pool: pool}
}

func (r *cloudAccountRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new CloudAccount with credentials encrypted at rest.
func (r *cloudAccountRepo) Create(ctx context.Context, account *cloud.CloudAccount) error {
	if account == nil {
		return fmt.Errorf("cloud account cannot be nil")
	}

	if account.ID == "" {
		account.ID = uuid.NewString()
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole != "platform_admin" && tenantID != "" {
		account.TenantID = tenantID
	}
	if account.TenantID == "" {
		account.TenantID = "default-tenant"
	}

	if account.Status == "" {
		account.Status = string(cloud.AccountStatusActive)
	}

	now := time.Now().UTC()
	if account.CreatedAt.IsZero() {
		account.CreatedAt = now
	}
	account.UpdatedAt = now

	encCreds, err := crypto.Encrypt(account.EncryptedCreds)
	if err != nil {
		return fmt.Errorf("encrypting cloud account credentials: %w", err)
	}

	query := `
		INSERT INTO cloud_accounts (id, name, provider, encrypted_creds, region, status, tenant_id, last_sync_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		account.ID,
		account.Name,
		string(account.Provider),
		encCreds,
		account.Region,
		account.Status,
		account.TenantID,
		account.LastSyncAt,
		account.CreatedAt,
		account.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating cloud account: %w", err)
	}

	return nil
}

// GetByID retrieves a CloudAccount by ID and decrypts stored credentials.
func (r *cloudAccountRepo) GetByID(ctx context.Context, id string) (*cloud.CloudAccount, error) {
	query := `
		SELECT id, name, provider, encrypted_creds, region, status, tenant_id, last_sync_at, created_at, updated_at
		FROM cloud_accounts
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)

	var acc cloud.CloudAccount
	var providerStr string
	var encCreds string

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&acc.ID,
		&acc.Name,
		&providerStr,
		&encCreds,
		&acc.Region,
		&acc.Status,
		&acc.TenantID,
		&acc.LastSyncAt,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting cloud account by id: %w", err)
	}

	acc.Provider = cloud.ProviderType(providerStr)

	if encCreds != "" {
		decrypted, err := crypto.Decrypt(encCreds)
		if err != nil {
			return nil, fmt.Errorf("decrypting cloud account credentials: %w", err)
		}
		acc.EncryptedCreds = decrypted
	}

	return &acc, nil
}

// List returns cloud accounts matching tenantID without decrypting credentials (creds empty).
func (r *cloudAccountRepo) List(ctx context.Context, tenantID string) ([]cloud.CloudAccount, error) {
	var query string
	var args []any

	if tenantID != "" {
		query = `
			SELECT id, name, provider, region, status, tenant_id, last_sync_at, created_at, updated_at
			FROM cloud_accounts
			WHERE tenant_id = $1
			ORDER BY created_at DESC
		`
		args = []any{tenantID}
	} else {
		query = `
			SELECT id, name, provider, region, status, tenant_id, last_sync_at, created_at, updated_at
			FROM cloud_accounts
			ORDER BY created_at DESC
		`
	}

	query, args = BuildTenantQuery(ctx, query, args...)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing cloud accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]cloud.CloudAccount, 0)
	for rows.Next() {
		var acc cloud.CloudAccount
		var providerStr string
		if err := rows.Scan(
			&acc.ID,
			&acc.Name,
			&providerStr,
			&acc.Region,
			&acc.Status,
			&acc.TenantID,
			&acc.LastSyncAt,
			&acc.CreatedAt,
			&acc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning cloud account: %w", err)
		}
		acc.Provider = cloud.ProviderType(providerStr)
		acc.EncryptedCreds = "" // Omit credentials in list responses for security
		accounts = append(accounts, acc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating cloud accounts: %w", err)
	}

	return accounts, nil
}

// Update updates an existing CloudAccount. If EncryptedCreds is provided, it is re-encrypted.
func (r *cloudAccountRepo) Update(ctx context.Context, account *cloud.CloudAccount) error {
	if account == nil {
		return fmt.Errorf("cloud account cannot be nil")
	}

	account.UpdatedAt = time.Now().UTC()

	var encCreds string
	var err error
	if account.EncryptedCreds != "" {
		encCreds, err = crypto.Encrypt(account.EncryptedCreds)
		if err != nil {
			return fmt.Errorf("encrypting cloud account credentials: %w", err)
		}
	}

	query := `
		UPDATE cloud_accounts
		SET name = $1,
		    provider = $2,
		    encrypted_creds = CASE WHEN $3 = '' THEN encrypted_creds ELSE $3 END,
		    region = $4,
		    status = $5,
		    last_sync_at = $6,
		    updated_at = $7
		WHERE id = $8
	`
	query, args := BuildTenantQuery(ctx, query,
		account.Name,
		string(account.Provider),
		encCreds,
		account.Region,
		account.Status,
		account.LastSyncAt,
		account.UpdatedAt,
		account.ID,
	)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating cloud account: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("cloud account not found: %s", account.ID)
	}

	return nil
}

// Delete removes a CloudAccount by ID.
func (r *cloudAccountRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM cloud_accounts WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting cloud account: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("cloud account not found: %s", id)
	}

	return nil
}
