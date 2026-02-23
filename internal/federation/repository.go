package federation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ErrNotFound is returned when a registry lookup finds no matching row.
var ErrNotFound = errors.New("registry not found")

// federationRepo is the repository interface consumed by FederationService.
// Defined here to keep the service testable without a real DB.
type federationRepo interface {
	Create(ctx context.Context, r *RegisteredRegistry) error
	GetByTrustRoot(ctx context.Context, trustRoot string) (*RegisteredRegistry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RegisteredRegistry, error)
	List(ctx context.Context, status RegistryStatus, limit, offset int) ([]*RegisteredRegistry, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status RegistryStatus) error
	SetIntermediateCA(ctx context.Context, id uuid.UUID, certPEM string) error
}

// FederationRepository is the Postgres-backed implementation of federationRepo.
type FederationRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewFederationRepository creates a new FederationRepository.
func NewFederationRepository(pool *pgxpool.Pool, logger *zap.Logger) *FederationRepository {
	return &FederationRepository{pool: pool, logger: logger}
}

// Create inserts a new registered_registries row.
func (r *FederationRepository) Create(ctx context.Context, reg *RegisteredRegistry) error {
	const q = `
		INSERT INTO registered_registries (trust_root, endpoint_url, intermediate_ca, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, registered_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		reg.TrustRoot,
		reg.EndpointURL,
		reg.IntermediateCA,
		string(reg.Status),
	)
	return row.Scan(&reg.ID, &reg.RegisteredAt, &reg.UpdatedAt)
}

// GetByTrustRoot fetches a registry by its trust_root value.
func (r *FederationRepository) GetByTrustRoot(ctx context.Context, trustRoot string) (*RegisteredRegistry, error) {
	const q = `
		SELECT id, trust_root, endpoint_url, intermediate_ca, status, registered_at, updated_at
		FROM registered_registries
		WHERE trust_root = $1`

	return r.scan(r.pool.QueryRow(ctx, q, trustRoot))
}

// GetByID fetches a registry by its primary key UUID.
func (r *FederationRepository) GetByID(ctx context.Context, id uuid.UUID) (*RegisteredRegistry, error) {
	const q = `
		SELECT id, trust_root, endpoint_url, intermediate_ca, status, registered_at, updated_at
		FROM registered_registries
		WHERE id = $1`

	return r.scan(r.pool.QueryRow(ctx, q, id))
}

// List returns registered registries filtered by status with pagination.
// An empty status string returns all records.
func (r *FederationRepository) List(ctx context.Context, status RegistryStatus, limit, offset int) ([]*RegisteredRegistry, error) {
	if limit <= 0 {
		limit = 50
	}

	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		const q = `
			SELECT id, trust_root, endpoint_url, intermediate_ca, status, registered_at, updated_at
			FROM registered_registries
			ORDER BY registered_at DESC
			LIMIT $1 OFFSET $2`
		rows, err = r.pool.Query(ctx, q, limit, offset)
	} else {
		const q = `
			SELECT id, trust_root, endpoint_url, intermediate_ca, status, registered_at, updated_at
			FROM registered_registries
			WHERE status = $1
			ORDER BY registered_at DESC
			LIMIT $2 OFFSET $3`
		rows, err = r.pool.Query(ctx, q, string(status), limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list registries: %w", err)
	}
	defer rows.Close()

	var result []*RegisteredRegistry
	for rows.Next() {
		reg, scanErr := r.scanRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, reg)
	}
	return result, rows.Err()
}

// UpdateStatus changes the status of a registry entry.
func (r *FederationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status RegistryStatus) error {
	const q = `
		UPDATE registered_registries
		SET status = $1, updated_at = now()
		WHERE id = $2`

	tag, err := r.pool.Exec(ctx, q, string(status), id)
	if err != nil {
		return fmt.Errorf("update registry status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetIntermediateCA stores the PEM-encoded intermediate CA cert for a registry.
func (r *FederationRepository) SetIntermediateCA(ctx context.Context, id uuid.UUID, certPEM string) error {
	const q = `
		UPDATE registered_registries
		SET intermediate_ca = $1, updated_at = now()
		WHERE id = $2`

	tag, err := r.pool.Exec(ctx, q, certPEM, id)
	if err != nil {
		return fmt.Errorf("set intermediate CA: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scan reads a single row from a pgx.Row (QueryRow result).
func (r *FederationRepository) scan(row pgx.Row) (*RegisteredRegistry, error) {
	reg := &RegisteredRegistry{}
	var status string
	err := row.Scan(
		&reg.ID,
		&reg.TrustRoot,
		&reg.EndpointURL,
		&reg.IntermediateCA,
		&status,
		&reg.RegisteredAt,
		&reg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan registry: %w", err)
	}
	reg.Status = RegistryStatus(status)
	return reg, nil
}

// scanRow reads a single row from a pgx.Rows (Query result).
func (r *FederationRepository) scanRow(rows pgx.Rows) (*RegisteredRegistry, error) {
	reg := &RegisteredRegistry{}
	var status string
	err := rows.Scan(
		&reg.ID,
		&reg.TrustRoot,
		&reg.EndpointURL,
		&reg.IntermediateCA,
		&status,
		&reg.RegisteredAt,
		&reg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan registry row: %w", err)
	}
	reg.Status = RegistryStatus(status)
	return reg, nil
}
