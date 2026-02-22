package trustledger_test

import (
	"context"
	"testing"

	"github.com/nexus-protocol/nexus/internal/trustledger"
)

var ctx = context.Background()

func TestNew_genesisEntry(t *testing.T) {
	l := trustledger.New()

	n, err := l.Len(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 genesis entry, got %d", n)
	}

	entry, err := l.Get(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Action != "genesis" {
		t.Errorf("expected action 'genesis', got %q", entry.Action)
	}
	if entry.Hash != trustledger.GenesisHash {
		t.Errorf("genesis hash: got %q, want GenesisHash", entry.Hash)
	}
}

func TestAppend_chainsCorrectly(t *testing.T) {
	l := trustledger.New()

	e1, err := l.Append(ctx, "agent://nexusagentprotocol.com/a/agent_1", "register", "example.com", map[string]string{"key": "val"})
	if err != nil {
		t.Fatal(err)
	}

	e2, err := l.Append(ctx, "agent://nexusagentprotocol.com/a/agent_1", "activate", "nexus-system", nil)
	if err != nil {
		t.Fatal(err)
	}

	if e2.PrevHash != e1.Hash {
		t.Errorf("chain broken: e2.PrevHash=%q, want e1.Hash=%q", e2.PrevHash, e1.Hash)
	}

	n, err := l.Len(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 { // genesis + 2
		t.Errorf("expected 3 entries, got %d", n)
	}
}

func TestVerify_valid(t *testing.T) {
	l := trustledger.New()
	_, _ = l.Append(ctx, "agent://nexusagentprotocol.com/a/agent_1", "register", "example.com", nil)
	_, _ = l.Append(ctx, "agent://nexusagentprotocol.com/a/agent_1", "activate", "nexus-system", nil)

	if err := l.Verify(ctx); err != nil {
		t.Errorf("Verify() failed on valid chain: %v", err)
	}
}

func TestRoot_returnsLastHash(t *testing.T) {
	l := trustledger.New()
	e, _ := l.Append(ctx, "agent://nexusagentprotocol.com/a/agent_1", "register", "example.com", nil)

	root, err := l.Root(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if root != e.Hash {
		t.Errorf("Root(): got %q, want %q", root, e.Hash)
	}
}

func TestVerify_genesisOnlyChain(t *testing.T) {
	l := trustledger.New()
	if err := l.Verify(ctx); err != nil {
		t.Errorf("Verify() on genesis-only chain should pass: %v", err)
	}
}

func TestRoot_genesisOnly(t *testing.T) {
	l := trustledger.New()
	root, err := l.Root(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if root != trustledger.GenesisHash {
		t.Errorf("Root() on genesis-only: got %q, want GenesisHash", root)
	}
}
