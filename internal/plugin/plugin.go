package plugin

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/internal/ui"
	"github.com/kanozad/dtop/pkg/collector"
	"github.com/kanozad/dtop/pkg/types"
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

// InputCapturer is an optional interface a plugin may implement to signal
// that it is currently consuming raw keyboard input (e.g. a text entry field
// or a modal chooser). While any visible plugin reports true, the app must
// suspend global key shortcuts (quit, toggles, menus) and deliver keys to
// plugins unmodified; Ctrl+C remains the global escape hatch.
type InputCapturer interface {
	CapturingInput() bool
}

// Reconfigurable is an optional interface a plugin may implement to accept
// updated configuration at runtime, e.g. when live config reload detects a
// change to the plugin's config block. Implementations must be safe to call
// from the UI goroutine while the plugin's Collect may be running concurrently.
type Reconfigurable interface {
	Reconfigure(cfg map[string]any)
}
