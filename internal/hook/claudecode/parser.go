// Package claudecode implements a hook.Parser for Claude Code's PostToolUse
// hook payload. It self-registers via init().
//
// Model resolution order:
//  1. "model" field in the hook JSON payload (if Claude Code sends it)
//  2. CLAUDE_MODEL environment variable
//  3. fallbackModel argument (from --model CLI flag)
package claudecode

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/jordi-jordi/scribe/internal/hook"
	"github.com/jordi-jordi/scribe/internal/pool"
)

const vendor = "anthropic"

func init() { hook.Register(&Parser{}) }

// Parser handles Claude Code PostToolUse JSON payloads.
// Claude Code sends one JSON object per hook invocation via stdin:
//
//	{"tool_name":"Write","tool_input":{...},"tool_response":{...},"model":"claude-sonnet-4-6"}
//
// The "model" field may or may not be present depending on the Claude Code
// version; the parser falls back to the CLAUDE_MODEL env var and then to the
// --model flag value.
type Parser struct{}

func (p *Parser) Name() string { return "claude-code" }

type payload struct {
	Model string `json:"model"`
}

func (p *Parser) Parse(r io.Reader, _, fallbackModel string) ([]pool.Entry, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var pl payload
		_ = json.Unmarshal([]byte(line), &pl) // model field is optional

		model := resolveModel(pl.Model, fallbackModel)
		return []pool.Entry{{
			Timestamp: time.Now().UTC(),
			Vendor:    vendor,
			Model:     model,
		}}, scanner.Err()
	}
	return nil, scanner.Err()
}

// resolveModel picks the best available model name in priority order.
func resolveModel(fromPayload, fallback string) string {
	if fromPayload != "" {
		return fromPayload
	}
	if env := os.Getenv("CLAUDE_MODEL"); env != "" {
		return env
	}
	return fallback
}
