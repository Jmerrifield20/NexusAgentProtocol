package trustledger

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MemoryLedger is an in-memory, thread-safe Ledger implementation.
// It is primarily useful for testing and for single-process deployments
// that do not require durable persistence across restarts.
type MemoryLedger struct {
	mu      sync.RWMutex
	entries []*Entry
}

// New creates a MemoryLedger initialised with the canonical genesis entry.
// The genesis entry is at index 0 and its hash is GenesisHash.
func New() *MemoryLedger {
	l := &MemoryLedger{}
	genesis := &Entry{
		Index:     0,
		Timestamp: time.Now().UTC(),
		Action:    "genesis",
		Actor:     "nexus-system",
		DataHash:  GenesisHash,
		PrevHash:  GenesisHash,
		Hash:      GenesisHash, // genesis hash is the well-known constant, not computed
	}
	l.entries = append(l.entries, genesis)
	return l
}

// Append implements Ledger.
func (l *MemoryLedger) Append(_ context.Context, agentURI, action, actor string, payload any) (*Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	dataHash := sha256Sum(payloadJSON)
	prev := l.entries[len(l.entries)-1]

	entry := &Entry{
		Index:     len(l.entries),
		Timestamp: time.Now().UTC(),
		AgentURI:  agentURI,
		Action:    action,
		Actor:     actor,
		DataHash:  dataHash,
		PrevHash:  prev.Hash,
	}
	entry.Hash = hashEntry(entry)
	l.entries = append(l.entries, entry)
	return entry, nil
}

// Get implements Ledger.
func (l *MemoryLedger) Get(_ context.Context, index int) (*Entry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index < 0 || index >= len(l.entries) {
		return nil, fmt.Errorf("index %d out of range", index)
	}
	return l.entries[index], nil
}

// Len implements Ledger.
func (l *MemoryLedger) Len(_ context.Context) (int, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries), nil
}

// Verify implements Ledger. It walks the chain and checks that all hashes
// are consistent. The genesis entry (index 0) is validated against GenesisHash.
func (l *MemoryLedger) Verify(_ context.Context) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for i, curr := range l.entries {
		if i == 0 {
			// Genesis: must equal the well-known constant.
			if curr.Hash != GenesisHash {
				return fmt.Errorf("genesis entry has wrong hash: got %q", curr.Hash)
			}
			continue
		}

		prev := l.entries[i-1]
		if curr.PrevHash != prev.Hash {
			return fmt.Errorf("hash chain broken at index %d", curr.Index)
		}
		if curr.Hash != hashEntry(curr) {
			return fmt.Errorf("entry %d has invalid hash", curr.Index)
		}
	}
	return nil
}

// Root implements Ledger.
func (l *MemoryLedger) Root(_ context.Context) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) == 0 {
		return "", nil
	}
	return l.entries[len(l.entries)-1].Hash, nil
}
