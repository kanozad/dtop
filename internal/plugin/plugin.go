package plugin

import (
	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/theme"
	"mld.com/dtop/internal/ui"
	"mld.com/dtop/pkg/collector"
	"mld.com/dtop/pkg/types"
)

type ID string

type Plugin interface {
	ID() ID
	Name() string
	collector.Collector
	Update(msg tea.Msg) tea.Cmd
	View(data collector.Data, width, height int, th theme.Theme) string
}

// SizeHinter is an optional interface a plugin may implement to declare
// its vertical sizing preferences. Plugins that do not implement it
// receive DefaultSizeHint().
type SizeHinter interface {
	SizeHint() ui.SizeHint
}

// HistoryAware is an optional interface a plugin may implement if it
// needs history buffers managed by the framework.
type HistoryAware interface {
	UpdateHistory(history *types.HistoryStore, data collector.Data, width int) collector.Data
}
