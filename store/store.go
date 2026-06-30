// Copyright (c) 2026 Eclipse Foundation AISBL
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package store

import "time"

// Entry is a single LLM tool usage event.
type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	Vendor      string    `json:"vendor"`                 // e.g. "anthropic", "github"
	Model       string    `json:"model"`                  // e.g. "claude-sonnet-4-6"
	ModelSource string    `json:"model_source,omitempty"` // payload|session|flag|default
	SessionID   string    `json:"session_id,omitempty"`
	EventName   string    `json:"event_name,omitempty"` // e.g. "SessionStart", "PostToolUse"
	Payload     string    `json:"payload,omitempty"`    // raw payload for debugging
}

// EditPool holds tool-use events that feed commit attribution.
// It is drained (cleared) on every 'scribe amend' or 'scribe clear'.
type EditPool interface {
	Add(entries ...Entry) error
	Peek() ([]Entry, error)
	Drain() ([]Entry, error)
	// DrainMatching removes entries whose Vendor:Model pair is in the given
	// set and returns them. Unmatched entries remain in the pool.
	DrainMatching(pairs map[string]struct{}) ([]Entry, error)
	Clear() error
}

// SessionPool holds session-start events so the model is always available
// for session-based fallback even after the edit pool has been drained.
// It is never cleared by normal scribe operations.
type SessionPool interface {
	Add(entries ...Entry) error
	Peek() ([]Entry, error)
}
