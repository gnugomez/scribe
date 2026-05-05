package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

// poolIsStale reports whether the pool was last written against a different
// HEAD commit than the current one. Returns false on any error so we never
// clear pools spuriously when git is unavailable.
func poolIsStale(headPath string, hashFn func() (string, error)) bool {
	current, err := hashFn()
	if err != nil {
		return false
	}
	stored, err := os.ReadFile(headPath)
	if os.IsNotExist(err) {
		return false // no sentinel means pool was just created, not stale
	}
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(stored)) != current
}

// writePoolHead records the current HEAD hash in the sentinel file next to
// the pool. Errors are silently ignored — a missing sentinel is safe.
func writePoolHead(headPath string, hashFn func() (string, error)) {
	hash, err := hashFn()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(headPath), 0o755)
	_ = os.WriteFile(headPath, []byte(hash), 0o644)
}

// removePoolHead deletes the sentinel file. Called when the pool is drained
// or cleared so subsequent stale checks start fresh.
func removePoolHead(headPath string) {
	_ = os.Remove(headPath)
}
