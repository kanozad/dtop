package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// MenuItem represents a single menu entry.
type MenuItem struct {
	Label string
	Value string // opaque value returned on selection
}

// MenuState tracks the current selection and scroll offset.
type MenuState struct {
	Items    []MenuItem
	Selected int
	Offset   int // scroll offset for visible window
}

// MenuOpts configures menu rendering.
type MenuOpts struct {
	Width        int
	MaxVisible   int // max items visible at once; 0 = unlimited
	NormalStyle  lipgloss.Style
	ActiveStyle  lipgloss.Style
	CursorPrefix string // prefix for selected item, e.g. "> "
}

// NewMenuState creates a MenuState from items.
func NewMenuState(items []MenuItem) MenuState {
	return MenuState{Items: items, Selected: 0, Offset: 0}
}

// MoveSelection moves the selection by delta, clamping to bounds and adjusting
// the scroll window as needed.
func (s *MenuState) MoveSelection(delta int, maxVisible int) {
	if len(s.Items) == 0 {
		return
	}
	s.Selected += delta
	if s.Selected < 0 {
		s.Selected = 0
	}
	if s.Selected >= len(s.Items) {
		s.Selected = len(s.Items) - 1
	}
	if maxVisible > 0 {
		if s.Selected < s.Offset {
			s.Offset = s.Selected
		}
		if s.Selected >= s.Offset+maxVisible {
			s.Offset = s.Selected - maxVisible + 1
		}
	}
}

// SelectedItem returns the currently selected MenuItem, or an empty one.
func (s *MenuState) SelectedItem() MenuItem {
	if s.Selected >= 0 && s.Selected < len(s.Items) {
		return s.Items[s.Selected]
	}
	return MenuItem{}
}

// RenderMenu renders the menu list.
func RenderMenu(state MenuState, opts MenuOpts) string {
	if len(state.Items) == 0 {
		return opts.NormalStyle.Render("(empty)")
	}

	prefix := opts.CursorPrefix
	if prefix == "" {
		prefix = "> "
	}
	padLen := len(prefix)
	pad := strings.Repeat(" ", padLen)

	end := len(state.Items)
	start := state.Offset
	if opts.MaxVisible > 0 && end-start > opts.MaxVisible {
		end = start + opts.MaxVisible
	}
	if end > len(state.Items) {
		end = len(state.Items)
	}

	var lines []string
	for i := start; i < end; i++ {
		item := state.Items[i]
		label := Truncate(item.Label, opts.Width-padLen)
		if i == state.Selected {
			lines = append(lines, opts.ActiveStyle.Render(prefix+label))
		} else {
			lines = append(lines, opts.NormalStyle.Render(pad+label))
		}
	}

	return strings.Join(lines, "\n")
}
