package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ── cobra command ────────────────────────────────────────────────────────────

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive wizard to configure Claude Code and Copilot Chat hooks",
		Long: `setup guides you through configuring scribe's hook integrations
for Claude Code CLI and GitHub Copilot Chat (VS Code).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := tea.NewProgram(newSetupModel()).Run()
			return err
		},
	}
}

// ── styles ───────────────────────────────────────────────────────────────────

var (
	boldStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // terminal green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // terminal red
	warnStyle    = lipgloss.NewStyle().Faint(true)
	choiceActive = lipgloss.NewStyle().Bold(true)
	choiceInact  = lipgloss.NewStyle().Faint(true)
)

// ── phases ───────────────────────────────────────────────────────────────────

type phase int

const (
	phaseInit phase = iota
	phaseSelect
	phaseClaudeCheck
	phaseClaudeConfirm
	phaseVSCodeCheck
	phaseVSCodeConfirm
	phaseDone
)

// ── messages ─────────────────────────────────────────────────────────────────

type prereqsMsg struct{ scribeOK, gitOK bool }
type claudeCheckMsg struct {
	path     string
	settings map[string]interface{}
	present  bool
}
type claudeApplyMsg struct{ err error }
type vscodeCheckMsg struct {
	path     string
	settings map[string]interface{}
	present  bool
}
type vscodeApplyMsg struct{ err error }

// ── model ────────────────────────────────────────────────────────────────────

type setupModel struct {
	phase  phase
	width  int
	lines  []string // accumulated output lines
	cursor int      // list position in phaseSelect; 0=Yes/1=No in confirm phases

	selectClaude bool
	selectVSCode bool

	claudePath     string
	claudeSettings map[string]interface{}
	vscodePath     string
	vscodeSettings map[string]interface{}
}

func newSetupModel() setupModel {
	return setupModel{
		width:        80,
		selectClaude: true,
		selectVSCode: true,
	}
}

// ── init / update ─────────────────────────────────────────────────────────────

func (m setupModel) Init() tea.Cmd { return runPrereqs }

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.phase == phaseDone {
				return m, tea.Quit
			}
			m.append(dimStyle.Render("  Cancelled."))
			m.phase = phaseDone
			return m, tea.Quit

		case "up", "k":
			if m.phase == phaseSelect && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.phase == phaseSelect && m.cursor < 1 {
				m.cursor++
			}

		case " ":
			if m.phase == phaseSelect {
				switch m.cursor {
				case 0:
					m.selectClaude = !m.selectClaude
				case 1:
					m.selectVSCode = !m.selectVSCode
				}
			} else if m.isConfirm() {
				return m.handleConfirm(m.cursor == 0)
			}

		case "left", "h":
			if m.isConfirm() {
				m.cursor = 0
			}
		case "right", "l":
			if m.isConfirm() {
				m.cursor = 1
			}

		case "y", "Y":
			return m.handleConfirm(true)
		case "n", "N":
			return m.handleConfirm(false)

		case "enter":
			if m.phase == phaseSelect {
				return m.startConfiguring()
			}
			if m.isConfirm() {
				return m.handleConfirm(m.cursor == 0)
			}
			if m.phase == phaseDone {
				return m, tea.Quit
			}
		}

	case prereqsMsg:
		return m.handlePrereqs(msg)

	case claudeCheckMsg:
		m.claudePath = msg.path
		m.claudeSettings = msg.settings
		m.appendSection("Claude Code")
		m.append(dimStyle.Render("  " + msg.path))
		if msg.present {
			m.append(successStyle.Render("  ✓  hook already configured"))
			m.append("")
			return m.nextAfterClaude()
		}
		m.append(warnStyle.Render("  ·  PostToolUse hook not found"))
		m.append("")
		m.phase = phaseClaudeConfirm
		m.cursor = 0

	case claudeApplyMsg:
		if msg.err != nil {
			m.append(errorStyle.Render("  ✗  " + msg.err.Error()))
			m.append(dimStyle.Render("  Add the hook manually — see README.md"))
		} else {
			m.append(successStyle.Render("  ✓  hook added"))
		}
		m.append("")
		return m.nextAfterClaude()

	case vscodeCheckMsg:
		m.vscodePath = msg.path
		m.vscodeSettings = msg.settings
		m.appendSection("VS Code — Copilot Chat")
		m.append(dimStyle.Render("  " + msg.path))
		if msg.present {
			m.append(successStyle.Render("  ✓  hook already configured"))
			m.append("")
			m.phase = phaseDone
			m.appendDone()
			return m, nil
		}
		m.append(warnStyle.Render("  ·  Copilot Chat Agent Hook not found"))
		m.append(dimStyle.Render("  (VS Code Agent Hooks are in preview — see README.md)"))
		m.append("")
		m.phase = phaseVSCodeConfirm
		m.cursor = 0

	case vscodeApplyMsg:
		if msg.err != nil {
			m.append(errorStyle.Render("  ✗  " + msg.err.Error()))
			m.append(dimStyle.Render("  Add the hook manually — see README.md"))
		} else {
			m.append(successStyle.Render("  ✓  hook added"))
		}
		m.append("")
		m.phase = phaseDone
		m.appendDone()
	}

	return m, nil
}

func (m setupModel) nextAfterClaude() (tea.Model, tea.Cmd) {
	if m.selectVSCode {
		m.phase = phaseVSCodeCheck
		return m, runVSCodeCheck
	}
	m.phase = phaseDone
	m.appendDone()
	return m, nil
}

func (m setupModel) startConfiguring() (tea.Model, tea.Cmd) {
	if !m.selectClaude && !m.selectVSCode {
		m.append(warnStyle.Render("  No integrations selected."))
		m.append("")
		m.phase = phaseDone
		m.appendDone()
		return m, nil
	}
	if m.selectClaude {
		m.phase = phaseClaudeCheck
		return m, runClaudeCheck
	}
	m.phase = phaseVSCodeCheck
	return m, runVSCodeCheck
}

func (m *setupModel) handlePrereqs(msg prereqsMsg) (setupModel, tea.Cmd) {
	if msg.scribeOK && msg.gitOK {
		m.phase = phaseSelect
		m.cursor = 0
		return *m, nil
	}
	m.appendSection("Prerequisites")
	if !msg.scribeOK {
		m.append(errorStyle.Render("  ✗  scribe not found in PATH"))
		m.append(dimStyle.Render("     run: go install github.com/jordi-jordi/scribe"))
	}
	if !msg.gitOK {
		m.append(errorStyle.Render("  ✗  git not found in PATH"))
	}
	m.append("")
	m.append(errorStyle.Render("  Resolve the issues above and run 'scribe setup' again."))
	m.phase = phaseDone
	return *m, nil
}

func (m setupModel) handleConfirm(yes bool) (tea.Model, tea.Cmd) {
	label := "Yes"
	if !yes {
		label = "No, skip"
	}
	m.append(dimStyle.Render("  " + label))
	m.append("")

	switch m.phase {
	case phaseClaudeConfirm:
		if yes {
			m.phase = phaseClaudeCheck
			return m, applyClaudeHook(m.claudePath, m.claudeSettings)
		}
		return m.nextAfterClaude()

	case phaseVSCodeConfirm:
		if yes {
			m.phase = phaseVSCodeCheck
			return m, applyVSCodeHook(m.vscodePath, m.vscodeSettings)
		}
		m.phase = phaseDone
		m.appendDone()
	}
	return m, nil
}

func (m *setupModel) isConfirm() bool {
	return m.phase == phaseClaudeConfirm || m.phase == phaseVSCodeConfirm
}

// ── view ──────────────────────────────────────────────────────────────────────

func (m setupModel) View() string {
	var b strings.Builder

	// Accumulated log.
	for _, line := range m.lines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Current prompt.
	switch m.phase {
	case phaseSelect:
		b.WriteString(boldStyle.Render("  Select integrations to configure:"))
		b.WriteString("\n\n")
		b.WriteString(m.renderSelectItem(0, "Claude Code CLI"))
		b.WriteString("\n")
		b.WriteString(m.renderSelectItem(1, "VS Code — Copilot Chat"))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  ↑↓ to move  space to toggle  enter to configure"))
		b.WriteString("\n")

	case phaseClaudeCheck, phaseVSCodeCheck:
		b.WriteString(dimStyle.Render("  checking…"))
		b.WriteString("\n")

	case phaseClaudeConfirm:
		b.WriteString(boldStyle.Render("  Add hook to ~/.claude/settings.json?"))
		b.WriteString("  ")
		b.WriteString(m.renderChoices())
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  ← → to move  y/n  enter to confirm"))
		b.WriteString("\n")

	case phaseVSCodeConfirm:
		b.WriteString(boldStyle.Render("  Create ~/.copilot/hooks/scribe.json?"))
		b.WriteString("  ")
		b.WriteString(m.renderChoices())
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  ← → to move  y/n  enter to confirm"))
		b.WriteString("\n")

	case phaseDone:
		b.WriteString(dimStyle.Render("  press enter to exit"))
		b.WriteString("\n")
	}

	return b.String()
}

func (m setupModel) renderSelectItem(idx int, label string) string {
	check := "[ ]"
	selected := (idx == 0 && m.selectClaude) || (idx == 1 && m.selectVSCode)
	if selected {
		check = "[*]"
	}
	line := fmt.Sprintf("  %s  %s", check, label)
	if m.cursor == idx {
		return boldStyle.Render(line)
	}
	return dimStyle.Render(line)
}

func (m setupModel) renderChoices() string {
	yes, no := choiceInact.Render("Yes"), choiceInact.Render("No")
	if m.cursor == 0 {
		yes = choiceActive.Render("[ Yes ]")
	} else {
		no = choiceActive.Render("[ No ]")
	}
	return yes + "  " + no
}

// ── log helpers ──────────────────────────────────────────────────────────────

func (m *setupModel) append(line string) { m.lines = append(m.lines, line) }

func (m *setupModel) appendSection(title string) {
	m.append(boldStyle.Render("  " + title))
	m.append("")
}

func (m *setupModel) appendDone() {
	m.appendSection("Done")
	m.append(successStyle.Render("  ✓  all set"))
	m.append("")
	m.append(dimStyle.Render("  scribe pool   — inspect pending events"))
	m.append(dimStyle.Render("  scribe amend  — annotate a commit"))
	m.append("")
}

// ── tea.Cmd functions ─────────────────────────────────────────────────────────

func runPrereqs() tea.Msg {
	_, sErr := exec.LookPath("scribe")
	_, gErr := exec.LookPath("git")
	return prereqsMsg{scribeOK: sErr == nil, gitOK: gErr == nil}
}

func runClaudeCheck() tea.Msg {
	path := claudeSettingsPath()
	settings := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	return claudeCheckMsg{path: path, settings: settings, present: claudeHookPresent(settings)}
}

func applyClaudeHook(path string, settings map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		injectClaudeHook(settings)
		return claudeApplyMsg{err: writeJSON(path, settings)}
	}
}

func runVSCodeCheck() tea.Msg {
	path := copilotHookPath()
	settings := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	return vscodeCheckMsg{path: path, settings: settings, present: copilotHookPresent(settings)}
}

func applyVSCodeHook(path string, settings map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		injectCopilotHook(settings)
		return vscodeApplyMsg{err: writeJSON(path, settings)}
	}
}

// ── pure helpers (shared with tests) ─────────────────────────────────────────

const (
	claudeHookCommand  = "scribe hook --vendor anthropic"
	copilotHookCommand = "scribe hook --vendor github --format copilot"
)

func claudeHookPresent(settings map[string]interface{}) bool {
	hooks, _ := settings["hooks"].(map[string]interface{})
	postToolUse, _ := hooks["PostToolUse"].([]interface{})
	for _, item := range postToolUse {
		group, _ := item.(map[string]interface{})
		for _, h := range asSlice(group["hooks"]) {
			hmap, _ := h.(map[string]interface{})
			if cmd, _ := hmap["command"].(string); strings.Contains(cmd, "scribe hook") {
				return true
			}
		}
	}
	return false
}

func injectClaudeHook(settings map[string]interface{}) {
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
	}
	postToolUse, _ := hooks["PostToolUse"].([]interface{})
	postToolUse = append(postToolUse, map[string]interface{}{
		"matcher": "Write|Edit|MultiEdit",
		"hooks": []interface{}{
			map[string]interface{}{"type": "command", "command": claudeHookCommand},
		},
	})
	hooks["PostToolUse"] = postToolUse
	settings["hooks"] = hooks
}

func copilotHookPresent(settings map[string]interface{}) bool {
	hooks, _ := settings["hooks"].(map[string]interface{})
	postToolUse, _ := hooks["PostToolUse"].([]interface{})
	for _, item := range postToolUse {
		h, _ := item.(map[string]interface{})
		if cmd, _ := h["command"].(string); strings.Contains(cmd, "scribe hook") {
			return true
		}
	}
	return false
}

func injectCopilotHook(settings map[string]interface{}) {
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
	}
	postToolUse, _ := hooks["PostToolUse"].([]interface{})
	postToolUse = append(postToolUse, map[string]interface{}{
		"type":    "command",
		"command": copilotHookCommand,
	})
	hooks["PostToolUse"] = postToolUse
	settings["hooks"] = hooks
}

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func copilotHookPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilot", "hooks", "scribe.json")
}

func writeJSON(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func asSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
