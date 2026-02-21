package trustledger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// GenesisHash is the canonical well-known hash of the genesis entry.
// It serves as the trust anchor of the chain; all subsequent entry hashes
// chain from this constant rather than from a computed value.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Entry is a single audit record in the trust ledger.
type Entry struct {
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
	AgentURI  string    `json:"agent_uri"`
	Action    string    `json:"action"`    // register, activate, revoke, update, genesis
	Actor     string    `json:"actor"`     // owner domain or "nexus-system"
	DataHash  string    `json:"data_hash"` // SHA-256 of the associated payload
	PrevHash  string    `json:"prev_hash"`
	Hash      string    `json:"hash"`
}

// hashEntry computes a deterministic SHA-256 hash over an entry's fields.
// This function must never be called on the genesis entry (index 0).
func hashEntry(e *Entry) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s|%s|%s|%s|%s",
		e.Index, e.Timestamp.Format(time.RFC3339Nano),
		e.AgentURI, e.Action, e.Actor, e.DataHash, e.PrevHash,
	)
	return hex.EncodeToString(h.Sum(nil))
}

// sha256Sum returns the hex-encoded SHA-256 digest of data.
func sha256Sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
