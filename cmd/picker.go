// Copyright (c) 2026 Eclipse Foundation AISBL
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// picker renders an interactive multi-select list in the terminal.
// Items are pre-selected by default. The user toggles with Space and
// confirms with Enter.
type picker struct {
	items    []string
	selected []bool
	cursor   int
	out      io.Writer
	header   string
}

func newPicker(items []string) *picker {
	sel := make([]bool, len(items))
	for i := range sel {
		sel[i] = true
	}
	return &picker{items: items, selected: sel, out: os.Stderr}
}

// newPickerWithDefaults creates a picker with explicit default selections.
func newPickerWithDefaults(items []string, defaults []bool) *picker {
	sel := make([]bool, len(items))
	copy(sel, defaults)
	return &picker{items: items, selected: sel, out: os.Stderr}
}

// Run displays the picker and returns the indices of selected items.
// Returns nil if the user cancels (Ctrl-C / q).
func (p *picker) Run() ([]int, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Not a terminal — fall back to selecting all.
		return p.allIndices(), nil
	}
	defer term.Restore(fd, oldState)

	p.render()

	reader := bufio.NewReader(os.Stdin)
	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return nil, err
		}

		switch r {
		case 'q', 3: // q or Ctrl-C
			p.clearDisplay()
			return nil, nil
		case '\r', '\n': // Enter
			p.clearDisplay()
			return p.selectedIndices(), nil
		case ' ': // toggle
			p.selected[p.cursor] = !p.selected[p.cursor]
			p.render()
		case 'a': // toggle all
			allSelected := p.allSelected()
			for i := range p.selected {
				p.selected[i] = !allSelected
			}
			p.render()
		case 27: // escape sequence
			if reader.Buffered() > 0 {
				next, _, _ := reader.ReadRune()
				if next == '[' {
					code, _, _ := reader.ReadRune()
					switch code {
					case 'A': // up
						if p.cursor > 0 {
							p.cursor--
						}
						p.render()
					case 'B': // down
						if p.cursor < len(p.items)-1 {
							p.cursor++
						}
						p.render()
					}
				}
			}
		case 'k': // vim up
			if p.cursor > 0 {
				p.cursor--
			}
			p.render()
		case 'j': // vim down
			if p.cursor < len(p.items)-1 {
				p.cursor++
			}
			p.render()
		}
	}
}

func (p *picker) render() {
	// Move cursor to start and clear lines.
	p.clearDisplay()
	header := p.header
	if header == "" {
		header = "Select models to include (space=toggle, a=all, enter=confirm):"
	}
	fmt.Fprintf(p.out, "\r%s\r\n", dim(header))
	for i, item := range p.items {
		cursor := "  "
		if i == p.cursor {
			cursor = dim("> ")
		}
		check := "◻ "
		if p.selected[i] {
			check = successStyle.Sprint("◼ ")
		}
		label := item
		if i == p.cursor {
			label = bold(item)
		}
		fmt.Fprintf(p.out, "\r%s%s%s\r\n", cursor, check, label)
	}
}

func (p *picker) clearDisplay() {
	// Move up len(items)+1 lines (items + header) and clear each.
	lines := len(p.items) + 1
	for i := 0; i < lines; i++ {
		fmt.Fprintf(p.out, "\033[A\033[2K")
	}
}

func (p *picker) selectedIndices() []int {
	var indices []int
	for i, s := range p.selected {
		if s {
			indices = append(indices, i)
		}
	}
	return indices
}

func (p *picker) allIndices() []int {
	indices := make([]int, len(p.items))
	for i := range indices {
		indices[i] = i
	}
	return indices
}

func (p *picker) allSelected() bool {
	for _, s := range p.selected {
		if !s {
			return false
		}
	}
	return true
}
