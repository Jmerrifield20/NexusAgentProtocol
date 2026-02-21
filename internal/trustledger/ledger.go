package trustledger

import "context"

// Ledger is the interface for the append-only Merkle-chain audit log.
// Both MemoryLedger and PostgresLedger implement this interface.
type Ledger interface {
	// Append adds a new entry chained to the previous one.
	// payload is JSON-marshalled and its SHA-256 is stored as DataHash.
	Append(ctx context.Context, agentURI, action, actor string, payload any) (*Entry, error)

	// Get returns the entry at the given zero-based index.
	Get(ctx context.Context, index int) (*Entry, error)

	// Len returns the total number of entries (including the genesis entry).
	Len(ctx context.Context) (int, error)

	// Verify walks the entire chain and checks hash consistency.
	// Returns nil if the chain is intact.
	Verify(ctx context.Context) error

	// Root returns the hash of the most recent entry (the chain tip).
	Root(ctx context.Context) (string, error)
}
