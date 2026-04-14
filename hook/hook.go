package hook

import (
	"fmt"
	"io"
	"sort"

	"github.com/gnugomez/scribe/store"
)

// Parser parses a hook payload from a specific LLM tool and returns store
// entries to record. Implementations self-register via init().
type Parser interface {
	// Name returns the unique format identifier (e.g. "claude", "copilot").
	Name() string

	// Parse reads the hook payload from r and returns entries to record.
	// fallbackVendor and fallbackModel are used when the payload does not
	// contain that information.
	Parse(r io.Reader, fallbackVendor, fallbackModel string) ([]store.Entry, error)
}

var registry = map[string]Parser{}

// Register adds p to the global parser registry. Called from init().
func Register(p Parser) { registry[p.Name()] = p }

// Get returns the parser for the given format name, or false if unknown.
func Get(name string) (Parser, bool) {
	p, ok := registry[name]
	return p, ok
}

// Names returns registered format names in sorted order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ErrUnknownFormat returns an error for an unrecognized format name.
func ErrUnknownFormat(name string) error {
	return fmt.Errorf("unknown hook format %q (available: %v)", name, Names())
}
