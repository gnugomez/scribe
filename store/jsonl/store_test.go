package jsonl_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gnugomez/scribe/store"
	"github.com/gnugomez/scribe/store/jsonl"
)

const (
	poolFilename  = "pool.jsonl"
	peekErrFormat = "Peek: %v"
)

func newTestPool(t *testing.T) *jsonl.Pool {
	t.Helper()
	return jsonl.NewPool(filepath.Join(t.TempDir(), poolFilename))
}

var (
	entryA = store.Entry{Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Vendor: "anthropic", Model: "claude-sonnet-4-6"}
	entryB = store.Entry{Timestamp: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC), Vendor: "github", Model: "gpt-4o"}
)

func TestStoreEmptyOnNonExistentFile(t *testing.T) {
	s := newTestPool(t)
	entries, err := s.Peek()
	if err != nil {
		t.Fatalf(peekErrFormat, err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty store, got %d entries", len(entries))
	}
}

func TestStoreAddAndPeek(t *testing.T) {
	s := newTestPool(t)
	if err := s.Add(entryA, entryB); err != nil {
		t.Fatalf("Add: %v", err)
	}
	entries, err := s.Peek()
	if err != nil {
		t.Fatalf(peekErrFormat, err)
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

func TestStorePeekDoesNotClear(t *testing.T) {
	s := newTestPool(t)
	_ = s.Add(entryA)
	_, _ = s.Peek()
	entries, err := s.Peek()
	if err != nil {
		t.Fatalf("second Peek: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("store should still have 1 entry after Peek, got %d", len(entries))
	}
}

func TestStoreDrain(t *testing.T) {
	s := newTestPool(t)
	_ = s.Add(entryA, entryB)
	entries, err := s.Drain()
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 drained entries, got %d", len(entries))
	}
	remaining, _ := s.Peek()
	if len(remaining) != 0 {
		t.Fatalf("store should be empty after Drain, got %d entries", len(remaining))
	}
}

func TestStoreDrainOnEmptyStore(t *testing.T) {
	s := newTestPool(t)
	entries, err := s.Drain()
	if err != nil {
		t.Fatalf("Drain on empty store: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestStoreClear(t *testing.T) {
	s := newTestPool(t)
	_ = s.Add(entryA)
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	entries, _ := s.Peek()
	if len(entries) != 0 {
		t.Fatalf("store should be empty after Clear, got %d entries", len(entries))
	}
}

func TestStoreClearOnNonExistentFile(t *testing.T) {
	s := newTestPool(t)
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear on non-existent file: %v", err)
	}
}

func TestStoreAddCreatesDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deep", poolFilename)
	s := jsonl.NewPool(path)
	if err := s.Add(entryA); err != nil {
		t.Fatalf("Add with missing parent dirs: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("store file not created: %v", err)
	}
}

func TestStoreSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, poolFilename)
	content := `not-json
{"timestamp":"2026-01-01T00:00:00Z","vendor":"anthropic","model":"claude-sonnet-4-6"}
{broken
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s := jsonl.NewPool(path)
	entries, err := s.Peek()
	if err != nil {
		t.Fatalf(peekErrFormat, err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
}
