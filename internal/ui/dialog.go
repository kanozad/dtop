package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DialogAction represents a button in a dialog.
type DialogAction struct {
	Label    string
	Selected bool
}

// DialogOpts configures a dialog overlay.
type DialogOpts struct {
	Title       string
	Body        string
	Actions     []DialogAction
	Width       int // desired inner width; clamped to terminal width
	BorderStyle lipgloss.Border
	BorderFg    lipgloss.TerminalColor
	TitleStyle  lipgloss.Style
	BodyStyle   lipgloss.Style
	ActionStyle lipgloss.Style // style for unselected actions
	ActiveStyle lipgloss.Style // style for the selected action
}

// RenderDialog renders a centered dialog overlay. The caller is responsible for
// compositing this on top of the main view using lipgloss.Place.
func RenderDialog(opts DialogOpts) string {
	if opts.Width <= 0 {
		opts.Width = 40
	}
	if opts.BorderStyle == (lipgloss.Border{}) {
		opts.BorderStyle = lipgloss.RoundedBorder()
	}

	var sections []string

	if opts.Title != "" {
		sections = append(sections, opts.TitleStyle.Render(opts.Title))
	}
	if opts.Body != "" {
		sections = append(sections, opts.BodyStyle.Render(opts.Body))
	}

	if len(opts.Actions) > 0 {
		var buttons []string
		for _, a := range opts.Actions {
			style := opts.ActionStyle
			if a.Selected {
				style = opts.ActiveStyle
			}
			buttons = append(buttons, style.Render(" "+a.Label+" "))
		}
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Center, strings.Join(buttons, "  ")))
	}

	content := strings.Join(sections, "\n\n")

	box := lipgloss.NewStyle().
		Border(opts.BorderStyle).
		BorderForeground(opts.BorderFg).
		Padding(1, 2).
		Width(opts.Width)

	return box.Render(content)
}

// PlaceOverlay centers overlay text within a viewport of the given dimensions.
func PlaceOverlay(overlay string, width, height int) string {
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		overlay)
}
