package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexus-protocol/nexus/internal/registry/model"
)

// ErrNotFound is returned when an agent is not found in the database.
var ErrNotFound = errors.New("agent not found")

// AgentRepository provides CRUD operations for agents against PostgreSQL.
type AgentRepository struct {
	db *pgxpool.Pool
}

// NewAgentRepository creates a new AgentRepository.
func NewAgentRepository(db *pgxpool.Pool) *AgentRepository {
	return &AgentRepository{db: db}
}

// Create inserts a new agent into the database.
func (r *AgentRepository) Create(ctx context.Context, agent *model.Agent) error {
	meta, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	agent.ID = uuid.New()
	now := time.Now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	if agent.RegistrationType == "" {
		agent.RegistrationType = model.RegistrationTypeDomain
	}
	agent.TrustTier = agent.ComputeTrustTier()

	query := `
		INSERT INTO agents (
			id, trust_root, capability_node, agent_id, display_name,
			description, endpoint, owner_domain, status, cert_serial,
			public_key_pem, metadata, created_at, updated_at, expires_at,
			owner_user_id, registration_type
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17
		)`

	_, err = r.db.Exec(ctx, query,
		agent.ID, agent.TrustRoot, agent.CapabilityNode, agent.AgentID,
		agent.DisplayName, agent.Description, agent.Endpoint, agent.OwnerDomain,
		agent.Status, agent.CertSerial, agent.PublicKeyPEM, meta,
		agent.CreatedAt, agent.UpdatedAt, agent.ExpiresAt,
		agent.OwnerUserID, agent.RegistrationType,
	)
	return err
}

// GetByID retrieves an agent by its internal UUID.
func (r *AgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Agent, error) {
	query := `SELECT * FROM agents WHERE id = $1`
	return r.scanOne(ctx, query, id)
}

// GetByAgentID retrieves an agent by trust_root + agent_id, with capability prefix matching.
// capNode is the top-level category from the URI (e.g. "finance").  Agents whose
// capability_node is that exact value OR starts with "{capNode}>" are matched, so
// "finance>accounting>reconciliation" is correctly found when querying with "finance".
func (r *AgentRepository) GetByAgentID(ctx context.Context, trustRoot, capNode, agentID string) (*model.Agent, error) {
	prefix := capNode + ">%"
	query := `
		SELECT * FROM agents
		WHERE trust_root = $1
		  AND (capability_node = $2 OR capability_node LIKE $3)
		  AND agent_id = $4`
	return r.scanOne(ctx, query, trustRoot, capNode, prefix, agentID)
}

// List returns all agents, with optional filtering by trust_root and capability_node.
func (r *AgentRepository) List(ctx context.Context, trustRoot, capNode string, limit, offset int) ([]*model.Agent, error) {
	if limit <= 0 {
		limit = 50
	}
	// Capability filter uses prefix matching: "finance" matches "finance>accounting>reconciliation".
	capPrefix := capNode + ">%"
	query := `
		SELECT * FROM agents
		WHERE ($1 = '' OR trust_root = $1)
		  AND ($2 = '' OR capability_node = $2 OR capability_node LIKE $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(ctx, query, trustRoot, capNode, capPrefix, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*model.Agent
	for rows.Next() {
		a, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// ListByOwnerDomain returns active agents for a given owner domain.
func (r *AgentRepository) ListByOwnerDomain(ctx context.Context, ownerDomain string, limit, offset int) ([]*model.Agent, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT * FROM agents
		WHERE owner_domain = $1
		  AND status = 'active'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, ownerDomain, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*model.Agent
	for rows.Next() {
		a, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// SearchByOrg returns all active agents registered under the given org namespace.
// In the new URI model, trust_root stores the org name (e.g. "acme"), so this
// is the canonical lookup for "what agents does the org acme have?"
func (r *AgentRepository) SearchByOrg(ctx context.Context, orgName string, limit, offset int) ([]*model.Agent, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT * FROM agents
		WHERE trust_root = $1
		  AND status = 'active'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, orgName, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*model.Agent
	for rows.Next() {
		a, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// SearchByCapability returns active agents whose capability_node exactly matches
// or is a hierarchical child of the given capability prefix (using ">" as separator).
// e.g. "finance" also matches "finance>accounting", "finance>accounting>reconciliation".
// Results are ordered by effective trust tier (trusted → verified → basic → unverified)
// then newest first. An optional orgName restricts results to a single trust root.
func (r *AgentRepository) SearchByCapability(ctx context.Context, capability, orgName string, limit, offset int) ([]*model.Agent, error) {
	if limit <= 0 {
		limit = 50
	}
	prefix := capability + ">%"
	query := `
		SELECT * FROM agents
		WHERE (capability_node = $1 OR capability_node LIKE $2)
		  AND status = 'active'
		  AND ($3 = '' OR trust_root = $3)
		ORDER BY
		  CASE
		    WHEN cert_serial != '' AND registration_type = 'domain' THEN 1
		    WHEN cert_serial != ''                                   THEN 2
		    WHEN registration_type = 'domain'                        THEN 3
		    ELSE 4
		  END,
		  created_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(ctx, query, capability, prefix, orgName, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*model.Agent
	for rows.Next() {
		a, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// ListByOwnerUserID returns all agents owned by a specific user, newest first.
func (r *AgentRepository) ListByOwnerUserID(ctx context.Context, ownerUserID uuid.UUID, limit, offset int) ([]*model.Agent, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT * FROM agents
		WHERE owner_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, ownerUserID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*model.Agent
	for rows.Next() {
		a, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// CountByOwner returns the number of non-revoked agents owned by a user.
func (r *AgentRepository) CountByOwner(ctx context.Context, ownerUserID uuid.UUID) (int, error) {
	var count int
	q := `SELECT COUNT(*) FROM agents WHERE owner_user_id = $1 AND status != 'revoked'`
	if err := r.db.QueryRow(ctx, q, ownerUserID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count by owner: %w", err)
	}
	return count, nil
}

// Update modifies an existing agent record.
func (r *AgentRepository) Update(ctx context.Context, agent *model.Agent) error {
	meta, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	agent.UpdatedAt = time.Now().UTC()
	query := `
		UPDATE agents SET
			display_name  = $2,
			description   = $3,
			endpoint      = $4,
			public_key_pem = $5,
			metadata      = $6,
			updated_at    = $7,
			version       = $8,
			tags          = $9,
			support_url   = $10,
			pricing_info  = $11
		WHERE id = $1`

	tags := agent.Tags
	if tags == nil {
		tags = []string{}
	}
	tag, err := r.db.Exec(ctx, query,
		agent.ID, agent.DisplayName, agent.Description,
		agent.Endpoint, agent.PublicKeyPEM, meta, agent.UpdatedAt,
		agent.Version, tags, agent.SupportURL, agent.PricingInfo,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus changes the status of an agent.
func (r *AgentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.AgentStatus) error {
	query := `UPDATE agents SET status = $2, updated_at = $3 WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id, status, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ActivateWithCert atomically sets status=active and stores the issued certificate
// serial and PEM on the agent record.
func (r *AgentRepository) ActivateWithCert(ctx context.Context, id uuid.UUID, certSerial, certPEM string) error {
	query := `
		UPDATE agents SET
			status         = 'active',
			cert_serial    = $2,
			public_key_pem = $3,
			updated_at     = $4
		WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id, certSerial, certPEM, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete permanently removes an agent record.
func (r *AgentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agents WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scanOne executes a query returning a single agent row.
func (r *AgentRepository) scanOne(ctx context.Context, query string, args ...any) (*model.Agent, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return r.scan(rows)
}

// scan reads a single agent from a pgx.Rows cursor.
// Column order matches the agents table definition including the two ALTER TABLE columns
// (owner_user_id, registration_type) appended at the end.
func (r *AgentRepository) scan(rows pgx.Rows) (*model.Agent, error) {
	var a model.Agent
	var metaRaw []byte

	err := rows.Scan(
		&a.ID, &a.TrustRoot, &a.CapabilityNode, &a.AgentID,
		&a.DisplayName, &a.Description, &a.Endpoint, &a.OwnerDomain,
		&a.Status, &a.CertSerial, &a.PublicKeyPEM, &metaRaw,
		&a.CreatedAt, &a.UpdatedAt, &a.ExpiresAt,
		&a.OwnerUserID, &a.RegistrationType,
		&a.Version, &a.Tags, &a.SupportURL, &a.PricingInfo,
		&a.LastSeenAt, &a.HealthStatus,
	)
	if err != nil {
		return nil, err
	}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &a.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	a.TrustTier = a.ComputeTrustTier()
	return &a, nil
}
