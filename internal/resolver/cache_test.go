package resolver

import (
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := newResolverCache(time.Minute)

	c.set("key1", "https://agent.example.com", "active", "abc123")

	entry, ok := c.get("key1")
	if !ok {
		t.Fatal("expected cache hit for key1")
	}
	if entry.endpoint != "https://agent.example.com" {
		t.Errorf("endpoint: got %q, want %q", entry.endpoint, "https://agent.example.com")
	}
	if entry.status != "active" {
		t.Errorf("status: got %q, want %q", entry.status, "active")
	}
	if entry.certSerial != "abc123" {
		t.Errorf("certSerial: got %q, want %q", entry.certSerial, "abc123")
	}
}

func TestCache_Miss(t *testing.T) {
	c := newResolverCache(time.Minute)
	_, ok := c.get("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := newResolverCache(10 * time.Millisecond)
	c.set("key", "endpoint", "active", "")

	// Immediate hit
	if _, ok := c.get("key"); !ok {
		t.Fatal("expected cache hit before expiry")
	}

	time.Sleep(20 * time.Millisecond)

	// After TTL, should be a miss
	if _, ok := c.get("key"); ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestCache_Invalidate(t *testing.T) {
	c := newResolverCache(time.Minute)
	c.set("key", "endpoint", "active", "")
	c.invalidate("key")

	if _, ok := c.get("key"); ok {
		t.Error("expected cache miss after invalidation")
	}
}

func TestCache_Evict(t *testing.T) {
	c := newResolverCache(10 * time.Millisecond)

	c.set("k1", "e1", "active", "")
	c.set("k2", "e2", "active", "")
	c.set("k3", "e3", "active", "")

	if c.len() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.len())
	}

	time.Sleep(20 * time.Millisecond)

	n := c.evict()
	if n != 3 {
		t.Errorf("evict() removed %d entries, want 3", n)
	}
	if c.len() != 0 {
		t.Errorf("cache has %d entries after eviction, want 0", c.len())
	}
}

func TestCache_Len(t *testing.T) {
	c := newResolverCache(time.Minute)
	if c.len() != 0 {
		t.Errorf("new cache len: got %d, want 0", c.len())
	}
	c.set("k1", "e", "s", "")
	c.set("k2", "e", "s", "")
	if c.len() != 2 {
		t.Errorf("after 2 sets, len: got %d, want 2", c.len())
	}
}
