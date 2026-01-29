package plugin

import (
	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/collector"
)

type ID string

type Plugin interface {
	ID() ID
	Name() string
	collector.Collector
	Update(msg tea.Msg) tea.Cmd
	View(data collector.Data, width, height int, th theme.Theme) string
}
