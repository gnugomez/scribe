package pool

import "time"

// Entry is a single AI tool usage event added to the pool.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Vendor    string    `json:"vendor"` // e.g. "anthropic", "github"
	Model     string    `json:"model"`  // e.g. "claude-sonnet-4-6"
}

// Pool accumulates AI tool usage events and can be drained when amending a
// commit. Implementations must be safe for concurrent use.
type Pool interface {
	// Add appends entries to the pool.
	Add(entries ...Entry) error

	// Peek returns the current pool contents without modifying them.
	Peek() ([]Entry, error)

	// Drain returns all entries and clears the pool in one operation.
	Drain() ([]Entry, error)

	// Clear empties the pool without returning its contents.
	Clear() error
}
