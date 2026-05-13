package jsonl

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gnugomez/scribe/store"
)

// Pool is a JSONL-backed pool. Each line is a JSON-encoded store.Entry.
// It satisfies both store.EditPool and store.SessionPool.
type Pool struct {
	path string
}

// NewPool returns a Pool that writes to path.
func NewPool(path string) *Pool {
	return &Pool{path: path}
}

func (p *Pool) Add(entries ...store.Entry) error {
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

func (p *Pool) Peek() ([]store.Entry, error) {
	return p.readAll()
}

func (p *Pool) Drain() ([]store.Entry, error) {
	entries, err := p.readAll()
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		if err := p.Clear(); err != nil {
			return entries, err
		}
	}
	return entries, nil
}

func (p *Pool) DrainMatching(pairs map[string]struct{}) ([]store.Entry, error) {
	entries, err := p.readAll()
	if err != nil {
		return nil, err
	}
	var matched, remaining []store.Entry
	for _, e := range entries {
		key := e.Vendor + ":" + e.Model
		if _, ok := pairs[key]; ok {
			matched = append(matched, e)
		} else {
			remaining = append(remaining, e)
		}
	}
	// Rewrite the pool with only unmatched entries.
	if err := p.Clear(); err != nil {
		return matched, err
	}
	if len(remaining) > 0 {
		if err := p.Add(remaining...); err != nil {
			return matched, err
		}
	}
	return matched, nil
}

func (p *Pool) Clear() error {
	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		return nil
	}
	return os.Truncate(p.path, 0)
}

func (p *Pool) readAll() ([]store.Entry, error) {
	f, err := os.Open(p.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []store.Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e store.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}
