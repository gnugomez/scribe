package jsonl_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jordi-jordi/scribe/internal/pool"
	"github.com/jordi-jordi/scribe/internal/pool/jsonl"
)

func newTestPool(t *testing.T) *jsonl.Pool {
	t.Helper()
	return jsonl.NewPool(filepath.Join(t.TempDir(), "pool.jsonl"))
}

var (
	entryA = pool.Entry{Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Vendor: "anthropic", Model: "claude-sonnet-4-6"}
	entryB = pool.Entry{Timestamp: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC), Vendor: "github", Model: "gpt-4o"}
)

func TestPool_EmptyOnNonExistentFile(t *testing.T) {
	p := newTestPool(t)

	entries, err := p.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty pool, got %d entries", len(entries))
	}
}

func TestPool_AddAndPeek(t *testing.T) {
	p := newTestPool(t)

	if err := p.Add(entryA, entryB); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entries, err := p.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Vendor != "anthropic" || entries[0].Model != "claude-sonnet-4-6" {
		t.Errorf("unexpected entry[0]: %+v", entries[0])
	}
	if entries[1].Vendor != "github" || entries[1].Model != "gpt-4o" {
		t.Errorf("unexpected entry[1]: %+v", entries[1])
	}
}

func TestPool_PeekDoesNotClear(t *testing.T) {
	p := newTestPool(t)
	_ = p.Add(entryA)

	_, _ = p.Peek()
	entries, err := p.Peek()
	if err != nil {
		t.Fatalf("second Peek: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("pool should still have 1 entry after Peek, got %d", len(entries))
	}
}

func TestPool_Drain(t *testing.T) {
	p := newTestPool(t)
	_ = p.Add(entryA, entryB)

	entries, err := p.Drain()
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 drained entries, got %d", len(entries))
	}

	// Pool must be empty after drain.
	remaining, _ := p.Peek()
	if len(remaining) != 0 {
		t.Fatalf("pool should be empty after Drain, got %d entries", len(remaining))
	}
}

func TestPool_DrainOnEmptyPool(t *testing.T) {
	p := newTestPool(t)

	entries, err := p.Drain()
	if err != nil {
		t.Fatalf("Drain on empty pool: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestPool_Clear(t *testing.T) {
	p := newTestPool(t)
	_ = p.Add(entryA)

	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	entries, _ := p.Peek()
	if len(entries) != 0 {
		t.Fatalf("pool should be empty after Clear, got %d entries", len(entries))
	}
}

func TestPool_ClearOnNonExistentFile(t *testing.T) {
	p := newTestPool(t)
	// Clear on a pool file that was never created should be a no-op.
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear on non-existent file: %v", err)
	}
}

func TestPool_AddCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "pool.jsonl")
	p := jsonl.NewPool(path)

	if err := p.Add(entryA); err != nil {
		t.Fatalf("Add with missing parent dirs: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pool file not created: %v", err)
	}
}

func TestPool_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pool.jsonl")

	// Write a mix of valid and malformed JSON lines.
	content := `not-json
{"timestamp":"2026-01-01T00:00:00Z","vendor":"anthropic","model":"claude-sonnet-4-6"}
{broken
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := jsonl.NewPool(path)
	entries, err := p.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
}
