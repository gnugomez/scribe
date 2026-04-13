package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── claudeHookPresent ────────────────────────────────────────────────────────

func TestClaudeHookPresent_Empty(t *testing.T) {
	if claudeHookPresent(map[string]interface{}{}) {
		t.Error("expected false for empty settings")
	}
}

func TestClaudeHookPresent_HookAlreadyThere(t *testing.T) {
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Write|Edit|MultiEdit",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "scribe hook --vendor anthropic",
						},
					},
				},
			},
		},
	}
	if !claudeHookPresent(settings) {
		t.Error("expected true when scribe hook is configured")
	}
}

func TestClaudeHookPresent_OtherHookPresent(t *testing.T) {
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"hooks": []interface{}{
						map[string]interface{}{"command": "some-other-tool"},
					},
				},
			},
		},
	}
	if claudeHookPresent(settings) {
		t.Error("expected false when only other hooks are present")
	}
}

// ── injectClaudeHook ─────────────────────────────────────────────────────────

func TestInjectClaudeHook_IntoEmpty(t *testing.T) {
	settings := map[string]interface{}{}
	injectClaudeHook(settings)

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatal("expected hooks key to be created")
	}
	postToolUse, _ := hooks["PostToolUse"].([]interface{})
	if len(postToolUse) != 1 {
		t.Fatalf("expected 1 PostToolUse entry, got %d", len(postToolUse))
	}
	group := postToolUse[0].(map[string]interface{})
	innerHooks := group["hooks"].([]interface{})
	cmd := innerHooks[0].(map[string]interface{})["command"].(string)
	if cmd != claudeHookCommand {
		t.Errorf("unexpected command: %q", cmd)
	}
}

func TestInjectClaudeHook_PreservesExistingHooks(t *testing.T) {
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{"hooks": []interface{}{
					map[string]interface{}{"type": "command", "command": "other-tool"},
				}},
			},
		},
	}
	injectClaudeHook(settings)

	hooks := settings["hooks"].(map[string]interface{})
	postToolUse := hooks["PostToolUse"].([]interface{})
	if len(postToolUse) != 2 {
		t.Fatalf("expected 2 PostToolUse entries (existing + new), got %d", len(postToolUse))
	}
}

// ── copilotHookPresent ───────────────────────────────────────────────────────

func TestCopilotHookPresent_Empty(t *testing.T) {
	if copilotHookPresent(map[string]interface{}{}) {
		t.Error("expected false for empty settings")
	}
}

func TestCopilotHookPresent_HookAlreadyThere(t *testing.T) {
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": "scribe hook --vendor github --format copilot",
				},
			},
		},
	}
	if !copilotHookPresent(settings) {
		t.Error("expected true when copilot hook is configured")
	}
}

func TestCopilotHookPresent_DifferentCommand(t *testing.T) {
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{"type": "command", "command": "some-other-tool"},
			},
		},
	}
	if copilotHookPresent(settings) {
		t.Error("expected false when a different command is set")
	}
}

// ── injectCopilotHook ────────────────────────────────────────────────────────

func TestInjectCopilotHook(t *testing.T) {
	settings := map[string]interface{}{}
	injectCopilotHook(settings)

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatal("expected hooks to be set")
	}
	postToolUse, _ := hooks["PostToolUse"].([]interface{})
	if len(postToolUse) != 1 {
		t.Fatalf("expected 1 PostToolUse entry, got %d", len(postToolUse))
	}
	h, _ := postToolUse[0].(map[string]interface{})
	if h["command"] != copilotHookCommand {
		t.Errorf("unexpected command: %q", h["command"])
	}
}

func TestInjectCopilotHook_PreservesExistingHooks(t *testing.T) {
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{"type": "command", "command": "other-tool"},
			},
		},
	}
	injectCopilotHook(settings)
	hooks := settings["hooks"].(map[string]interface{})
	postToolUse := hooks["PostToolUse"].([]interface{})
	if len(postToolUse) != 2 {
		t.Fatalf("expected 2 PostToolUse entries, got %d", len(postToolUse))
	}
}

// ── writeJSON ────────────────────────────────────────────────────────────────

func TestWriteJSON_CreatesFileAndDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")

	if err := writeJSON(path, map[string]interface{}{"key": "value"}); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("unexpected content: %v", result)
	}
}

// ── Bubbletea model unit tests ────────────────────────────────────────────────

func TestSetupModel_PrereqsOK_AdvancesToSelect(t *testing.T) {
	m := newSetupModel()
	result, _ := m.Update(prereqsMsg{scribeOK: true, gitOK: true})
	updated := result.(setupModel)
	if updated.phase != phaseSelect {
		t.Errorf("expected phaseSelect after prereqs pass, got %v", updated.phase)
	}
}

func TestSetupModel_Select_EnterStartsConfiguring(t *testing.T) {
	m := newSetupModel()
	m.phase = phaseSelect
	m.selectClaude = true
	m.selectVSCode = false

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(setupModel)
	if updated.phase != phaseClaudeCheck {
		t.Errorf("expected phaseClaudeCheck after enter with claude selected, got %v", updated.phase)
	}
	if cmd == nil {
		t.Error("expected a command to be returned")
	}
}

func TestSetupModel_Select_NoneSelected_GoesToDone(t *testing.T) {
	m := newSetupModel()
	m.phase = phaseSelect
	m.selectClaude = false
	m.selectVSCode = false

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(setupModel)
	if updated.phase != phaseDone {
		t.Errorf("expected phaseDone when nothing selected, got %v", updated.phase)
	}
}

func TestSetupModel_Select_SpaceToggles(t *testing.T) {
	m := newSetupModel()
	m.phase = phaseSelect
	m.cursor = 0
	m.selectClaude = true

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	updated := result.(setupModel)
	if updated.selectClaude {
		t.Error("expected selectClaude to be toggled off")
	}
}

func TestSetupModel_PrereqsFail_GoesToDone(t *testing.T) {
	m := newSetupModel()
	result, _ := m.Update(prereqsMsg{scribeOK: false, gitOK: true})
	updated := result.(setupModel)
	if updated.phase != phaseDone {
		t.Errorf("expected phaseDone when prereqs fail, got %v", updated.phase)
	}
}

func TestSetupModel_ClaudePresent_SkipsToVSCodeCheck(t *testing.T) {
	m := newSetupModel()
	result, _ := m.Update(claudeCheckMsg{
		path:     "/fake/settings.json",
		settings: map[string]interface{}{},
		present:  true,
	})
	updated := result.(setupModel)
	if updated.phase != phaseVSCodeCheck {
		t.Errorf("expected phaseVSCodeCheck when hook already present, got %v", updated.phase)
	}
}

func TestSetupModel_ClaudeNotPresent_GoesToConfirm(t *testing.T) {
	m := newSetupModel()
	result, _ := m.Update(claudeCheckMsg{
		path:     "/fake/settings.json",
		settings: map[string]interface{}{},
		present:  false,
	})
	updated := result.(setupModel)
	if updated.phase != phaseClaudeConfirm {
		t.Errorf("expected phaseClaudeConfirm, got %v", updated.phase)
	}
}

func TestSetupModel_VSCodePresent_GoesToDone(t *testing.T) {
	m := newSetupModel()
	result, _ := m.Update(vscodeCheckMsg{
		path:     "/fake/settings.json",
		settings: map[string]interface{}{},
		present:  true,
	})
	updated := result.(setupModel)
	if updated.phase != phaseDone {
		t.Errorf("expected phaseDone when vscode hook present, got %v", updated.phase)
	}
}

func TestSetupModel_CursorMovement(t *testing.T) {
	m := newSetupModel()
	m.phase = phaseClaudeConfirm
	m.cursor = 0

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	updated := result.(setupModel)
	if updated.cursor != 1 {
		t.Errorf("expected cursor 1 after right, got %d", updated.cursor)
	}

	result, _ = updated.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated = result.(setupModel)
	if updated.cursor != 0 {
		t.Errorf("expected cursor 0 after left, got %d", updated.cursor)
	}
}

func TestSetupModel_ApplyError_LogsError(t *testing.T) {
	m := newSetupModel()
	m.phase = phaseClaudeCheck
	result, _ := m.Update(claudeApplyMsg{err: fmt.Errorf("disk full")})
	updated := result.(setupModel)

	found := false
	for _, line := range updated.lines {
		if containsStripped(line, "disk full") || containsStripped(line, "Failed") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error message in log lines, got: %v", updated.lines)
	}
}

// containsStripped checks if s is in text after stripping ANSI escape codes.
func containsStripped(text, s string) bool {
	// Simple check — lipgloss renders ANSI codes we can't easily strip in tests,
	// so we search for the plain substring across the raw string too.
	return strings.Contains(text, s)
}
