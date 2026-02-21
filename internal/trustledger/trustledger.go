// Package trustledger implements a Merkle-chain audit log for agent lifecycle events.
//
// The chain begins with a well-known genesis entry whose Hash equals GenesisHash
// (64 hex zeros). Every subsequent entry records the SHA-256 of its predecessor,
// making any tampering detectable via Verify.
//
// Two implementations of the Ledger interface are provided:
//   - MemoryLedger: in-process, for testing and development.
//   - PostgresLedger: durable, for production use.
package trustledger
