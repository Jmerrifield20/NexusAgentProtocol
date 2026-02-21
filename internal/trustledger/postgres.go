package trustledger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// advisoryLockKey is a stable PostgreSQL advisory lock key used to serialise
// concurrent Append calls. The value is arbitrary but must be consistent
// across all registry instances.
const advisoryLockKey = int64(1_159_876_543)

// PostgresLedger persists the Merkle-chain audit log to a PostgreSQL database.
// It implements the Ledger interface.
type PostgresLedger struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPostgresLedger creates a PostgresLedger backed by the given connection pool.
func NewPostgresLedger(pool *pgxpool.Pool, logger *zap.Logger) *PostgresLedger {
	return &PostgresLedger{pool: pool, logger: logger}
}

// Append implements Ledger.
// It acquires a PostgreSQL advisory lock, reads the chain tail, computes the
// new entry hash, and inserts it — all within a single serialisable transaction.
func (l *PostgresLedger) Append(ctx context.Context, agentURI, action, actor string, payload any) (*Entry, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	dataHash := sha256Sum(payloadJSON)

	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Serialise concurrent appends with a transaction-scoped advisory lock.
	// The lock is automatically released when the transaction commits or rolls back.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", advisoryLockKey); err != nil {
		return nil, fmt.Errorf("acquire advisory lock: %w", err)
	}

	// Read the current tail of the chain.
	var prevIdx int
	var prevHash string
	if err := tx.QueryRow(ctx,
		"SELECT idx, hash FROM trust_ledger ORDER BY idx DESC LIMIT 1",
	).Scan(&prevIdx, &prevHash); err != nil {
		return nil, fmt.Errorf("read ledger tail: %w", err)
	}

	now := time.Now().UTC()
	entry := &Entry{
		Index:     prevIdx + 1,
		Timestamp: now,
		AgentURI:  agentURI,
		Action:    action,
		Actor:     actor,
		DataHash:  dataHash,
		PrevHash:  prevHash,
	}
	entry.Hash = hashEntry(entry)

	if _, err := tx.Exec(ctx,
		`INSERT INTO trust_ledger (idx, timestamp, agent_uri, action, actor, data_hash, prev_hash, hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.Index, entry.Timestamp, entry.AgentURI,
		entry.Action, entry.Actor, entry.DataHash,
		entry.PrevHash, entry.Hash,
	); err != nil {
		return nil, fmt.Errorf("insert ledger entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ledger tx: %w", err)
	}

	l.logger.Debug("ledger entry appended",
		zap.Int("idx", entry.Index),
		zap.String("action", entry.Action),
		zap.String("agent_uri", entry.AgentURI),
	)
	return entry, nil
}

// Get implements Ledger.
func (l *PostgresLedger) Get(ctx context.Context, index int) (*Entry, error) {
	entry := &Entry{}
	if err := l.pool.QueryRow(ctx,
		`SELECT idx, timestamp, agent_uri, action, actor, data_hash, prev_hash, hash
		 FROM trust_ledger WHERE idx = $1`, index,
	).Scan(
		&entry.Index, &entry.Timestamp, &entry.AgentURI,
		&entry.Action, &entry.Actor, &entry.DataHash,
		&entry.PrevHash, &entry.Hash,
	); err != nil {
		return nil, fmt.Errorf("get ledger entry %d: %w", index, err)
	}
	return entry, nil
}

// Len implements Ledger.
func (l *PostgresLedger) Len(ctx context.Context) (int, error) {
	var n int
	if err := l.pool.QueryRow(ctx, "SELECT COUNT(*) FROM trust_ledger").Scan(&n); err != nil {
		return 0, fmt.Errorf("count ledger entries: %w", err)
	}
	return n, nil
}

// Verify implements Ledger. It streams all rows ordered by idx and validates
// the hash chain. O(n) in ledger length; may be slow for very large ledgers.
func (l *PostgresLedger) Verify(ctx context.Context) error {
	rows, err := l.pool.Query(ctx,
		`SELECT idx, timestamp, agent_uri, action, actor, data_hash, prev_hash, hash
		 FROM trust_ledger ORDER BY idx ASC`,
	)
	if err != nil {
		return fmt.Errorf("query ledger: %w", err)
	}
	defer rows.Close()

	var prev *Entry
	for rows.Next() {
		curr := &Entry{}
		if err := rows.Scan(
			&curr.Index, &curr.Timestamp, &curr.AgentURI,
			&curr.Action, &curr.Actor, &curr.DataHash,
			&curr.PrevHash, &curr.Hash,
		); err != nil {
			return fmt.Errorf("scan ledger row: %w", err)
		}

		if prev == nil {
			// Validate genesis: hash must be the well-known constant.
			if curr.Hash != GenesisHash {
				return fmt.Errorf("genesis entry has wrong hash: got %q", curr.Hash)
			}
			prev = curr
			continue
		}

		if curr.PrevHash != prev.Hash {
			return fmt.Errorf("hash chain broken at index %d", curr.Index)
		}
		if curr.Hash != hashEntry(curr) {
			return fmt.Errorf("entry %d has invalid hash", curr.Index)
		}
		prev = curr
	}
	return rows.Err()
}

// Root implements Ledger.
func (l *PostgresLedger) Root(ctx context.Context) (string, error) {
	var hash string
	if err := l.pool.QueryRow(ctx,
		"SELECT hash FROM trust_ledger ORDER BY idx DESC LIMIT 1",
	).Scan(&hash); err != nil {
		return "", fmt.Errorf("get ledger root: %w", err)
	}
	return hash, nil
}
