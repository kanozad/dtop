package clock

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/collector"
)

type Clock struct{}

func New() *Clock { return &Clock{} }

func (c *Clock) ID() plugin.ID                  { return "clock" }
func (c *Clock) Name() string                   { return "Clock" }
func (c *Clock) AllowedConfigKeys() []string    { return nil }
func (c *Clock) Shutdown(context.Context) error { return nil }

func (c *Clock) Init(context.Context, map[string]any) error {
	return nil
}
func (c *Clock) Collect(context.Context) (collector.Data, error) {
	return time.Now(), nil
}

func (c *Clock) Update(tea.Msg) tea.Cmd { return nil }

func (c *Clock) View(data collector.Data, width, height int, th theme.Theme) string {
	value, ok := data.(time.Time)
	if !ok {
		return th.RenderBox("Clock", th.Muted.Render("Collecting..."), width, height)
	}
	body := th.Text.Render(value.Format(time.RFC1123))
	return th.RenderBox("Clock", body, width, height)
}
