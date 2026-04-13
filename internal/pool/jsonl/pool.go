package jsonl

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/jordi-jordi/scribe/internal/pool"
)

// Pool is a JSONL-backed pool stored at <repo>/.git/scribe/pool.jsonl.
// Each line is a JSON-encoded pool.Entry. Drain reads and then truncates
// the file. The path is injected so the pool stays decoupled from git.
type Pool struct {
	path string
}

// NewPool returns a Pool that writes to path.
// Call with the value returned by git.PoolPath().
func NewPool(path string) *Pool {
	return &Pool{path: path}
}

// Add appends entries to the pool file.
func (p *Pool) Add(entries ...pool.Entry) error {
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

// Peek returns the current pool contents without modifying the file.
func (p *Pool) Peek() ([]pool.Entry, error) {
	return p.readAll()
}

// Drain returns all entries and clears the pool file.
func (p *Pool) Drain() ([]pool.Entry, error) {
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

// Clear truncates the pool file.
func (p *Pool) Clear() error {
	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		return nil
	}
	return os.Truncate(p.path, 0)
}

func (p *Pool) readAll() ([]pool.Entry, error) {
	f, err := os.Open(p.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []pool.Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e pool.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}
