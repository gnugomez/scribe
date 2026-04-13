package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jordi-jordi/scribe/internal/hook"
	"github.com/jordi-jordi/scribe/internal/pool"
	"github.com/spf13/cobra"
)

func newHookCmd(p pool.Pool, poolPath string) *cobra.Command {
	var vendor, model, format string

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Receive an LLM tool hook event and add it to the pool",
		Long: `hook reads a JSON payload from stdin (sent by Claude Code, Copilot Chat,
or another supported tool) and adds an entry to the repo-local pool.

The model name is extracted from the payload when available, with --model
as a fallback. This command always exits 0 so it never blocks the calling tool.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if poolPath == "" {
				// Not in a git repo — silently exit so we don't disrupt the tool.
				return nil
			}

			parser, ok := hook.Get(format)
			if !ok {
				fmt.Fprintf(os.Stderr, "scribe hook: %v\n", hook.ErrUnknownFormat(format))
				return nil
			}

			entries, err := parser.Parse(cmd.InOrStdin(), vendor, model)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scribe hook: parse error: %v\n", err)
				return nil
			}
			if len(entries) == 0 {
				return nil
			}

			enrichEntriesWithSessionModel(p, entries)

			if err := p.Add(entries...); err != nil {
				fmt.Fprintf(os.Stderr, "scribe hook: pool error: %v\n", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&vendor, "vendor", "", "LLM vendor — used as fallback if not in payload (e.g. anthropic, github)")
	cmd.Flags().StringVar(&model, "model", "", "Model name — used as fallback if not in payload or env")
	cmd.Flags().StringVar(&format, "format", "claude", fmt.Sprintf("Hook payload format (available: %v)", hook.Names()))

	return cmd
}

func enrichEntriesWithSessionModel(p pool.Pool, entries []pool.Entry) {
	existing, err := p.Peek()
	if err != nil {
		return
	}

	modelBySession := make(map[string]string)
	for _, e := range existing {
		if e.SessionID == "" || isUnknownModel(e.Model) {
			continue
		}
		modelBySession[sessionKey(e.Vendor, e.SessionID)] = e.Model
	}

	for i := range entries {
		e := &entries[i]
		if isUnknownModel(e.Model) {
			if model, ok := resolveModelFromTranscript(e.Payload); ok {
				e.Model = model
			}
		}
		if e.SessionID != "" && isUnknownModel(e.Model) {
			if model, ok := modelBySession[sessionKey(e.Vendor, e.SessionID)]; ok {
				e.Model = model
			}
		}
		if e.SessionID != "" && !isUnknownModel(e.Model) {
			modelBySession[sessionKey(e.Vendor, e.SessionID)] = e.Model
		}
	}
}

type transcriptPayload struct {
	TranscriptPath string `json:"transcript_path"`
}

type transcriptLine struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
	} `json:"message"`
}

func resolveModelFromTranscript(rawPayload string) (string, bool) {
	var p transcriptPayload
	if err := json.Unmarshal([]byte(rawPayload), &p); err != nil || p.TranscriptPath == "" {
		return "", false
	}

	f, err := os.Open(p.TranscriptPath)
	if err != nil {
		return "", false
	}
	defer f.Close()

	var latest string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var l transcriptLine
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue
		}
		if l.Type == "assistant" && l.Message.Model != "" {
			latest = l.Message.Model
		}
	}
	if latest == "" {
		return "", false
	}
	return latest, true
}

func sessionKey(vendor, sessionID string) string {
	return vendor + "|" + sessionID
}

func isUnknownModel(model string) bool {
	return model == "" || model == "claude"
}
