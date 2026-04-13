// Package copilot implements a hook.Parser for GitHub Copilot Chat's VS Code
// Agent Hook payload. It self-registers via init().
//
// Model resolution order:
//  1. "model" field in the hook JSON payload
//  2. COPILOT_MODEL or GITHUB_COPILOT_MODEL environment variable
//  3. fallbackModel argument (from --model CLI flag)
package copilot

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/jordi-jordi/scribe/internal/hook"
	"github.com/jordi-jordi/scribe/internal/pool"
)

const vendor = "github"

func init() { hook.Register(&Parser{}) }

// Parser handles GitHub Copilot Chat VS Code Agent Hook payloads.
// The VS Code Agent Hooks API (preview) sends tool-use events via stdin.
// Expected shape:
//
//	{"model":"gpt-4o","tool":"...","input":{...}}
//
// Refer to https://code.visualstudio.com/docs/copilot/customization/hooks
// for the authoritative payload schema as the API is in preview.
type Parser struct{}

func (p *Parser) Name() string { return "copilot" }

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
			Payload:   line,
		}}, scanner.Err()
	}
	return nil, scanner.Err()
}

// resolveModel picks the best available model name in priority order.
func resolveModel(fromPayload, fallback string) string {
	if fromPayload != "" {
		return fromPayload
	}
	for _, env := range []string{"COPILOT_MODEL", "GITHUB_COPILOT_MODEL"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	if fallback != "" {
		return fallback
	}
	return "copilot" // VS Code Copilot Chat does not expose the model in its hook payload
}
