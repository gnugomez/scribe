package cmd

import (
	"fmt"
	"os"

	"github.com/gnugomez/scribe/hook"
	"github.com/gnugomez/scribe/store"
	"github.com/spf13/cobra"
)

// newHookCmd creates the hook subcommand.
// editPool accumulates tool-use events that feed commit attribution.
// sessionPool persists session→model mappings and is never cleared.
func newHookCmd(editPool store.EditPool, sessionPool store.SessionPool, poolPath string) *cobra.Command {
	var vendor, model, format string

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Receive an LLM tool hook event and add it to the pool",
		Long: `hook reads a JSON payload from stdin (sent by Claude Code, Copilot Chat,
or another supported tool) and records an entry.

SessionStart events are stored in the session store so the model is available
for later PostToolUse events even after 'scribe amend' has been run.
This command always exits 0 so it never blocks the calling tool.`,
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

			routeEntries(entries, editPool, sessionPool)
			return nil
		},
	}

	cmd.Flags().StringVar(&vendor, "vendor", "", "LLM vendor — used as fallback if not in payload (e.g. anthropic, github)")
	cmd.Flags().StringVar(&model, "model", "", "Model name — used as fallback if not in payload")
	cmd.Flags().StringVar(&format, "format", "claude", fmt.Sprintf("Hook payload format (available: %v)", hook.Names()))

	return cmd
}

func enrichEntriesWithSessionModel(sessionPool store.SessionPool, entries []store.Entry) {
	existing, err := sessionPool.Peek()
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
		if e.SessionID != "" && isUnknownModel(e.Model) {
			if model, ok := modelBySession[sessionKey(e.Vendor, e.SessionID)]; ok {
				e.Model = model
				e.ModelSource = "session"
			}
		}
		if e.SessionID != "" && !isUnknownModel(e.Model) {
			key := sessionKey(e.Vendor, e.SessionID)
			if _, already := modelBySession[key]; !already {
				// Persist newly discovered model to session pool so it
				// survives edit-pool drains (amend/clear).
				_ = sessionPool.Add(store.Entry{
					Timestamp:   e.Timestamp,
					Vendor:      e.Vendor,
					Model:       e.Model,
					ModelSource: e.ModelSource,
					SessionID:   e.SessionID,
				})
			}
			modelBySession[key] = e.Model
		}
	}
}

func sessionKey(vendor, sessionID string) string {
	return vendor + "|" + sessionID
}

func isUnknownModel(model string) bool {
	return model == "" || model == "claude" || model == "copilot"
}

// routeEntries sends SessionStart events to sessionPool and all other events
// to editPool, enriching them with the session-derived model first.
func routeEntries(entries []store.Entry, editPool store.EditPool, sessionPool store.SessionPool) {
	var editEntries []store.Entry
	for _, e := range entries {
		if e.EventName == "SessionStart" {
			if err := sessionPool.Add(e); err != nil {
				fmt.Fprintf(os.Stderr, "scribe hook: session pool error: %v\n", err)
			}
		} else {
			editEntries = append(editEntries, e)
		}
	}
	if len(editEntries) == 0 {
		return
	}
	enrichEntriesWithSessionModel(sessionPool, editEntries)
	if err := editPool.Add(editEntries...); err != nil {
		fmt.Fprintf(os.Stderr, "scribe hook: pool error: %v\n", err)
	}
}
