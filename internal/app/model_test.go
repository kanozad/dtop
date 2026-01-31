package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/config"
	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/collector"
)

type modelPlugin struct {
	id   plugin.ID
	name string
}

func (p *modelPlugin) ID() plugin.ID { return p.id }
func (p *modelPlugin) Name() string  { return p.name }
func (p *modelPlugin) Init(context.Context, map[string]any) error {
	return nil
}
func (p *modelPlugin) Collect(context.Context) (collector.Data, error) {
	return nil, nil
}
func (p *modelPlugin) Shutdown(context.Context) error { return nil }
func (p *modelPlugin) Update(tea.Msg) tea.Cmd         { return nil }
func (p *modelPlugin) View(collector.Data, int, int, theme.Theme) string {
	return "ok"
}

func TestModelViewNoPlugins(t *testing.T) {
	t.Parallel()

	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), nil)
	m.width = 80
	m.height = 10

	view := m.View()
	if !strings.Contains(view, "No plugins enabled") {
		t.Fatalf("expected empty plugin placeholder, got:\n%s", view)
	}
}

func TestModelViewPluginErrors(t *testing.T) {
	t.Parallel()

	p := &modelPlugin{id: "clock", name: "Clock"}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), []plugin.Plugin{p})
	m.width = 80
	m.height = 10
	m.pluginErrs = map[plugin.ID]error{"clock": errors.New("boom")}

	view := m.View()
	if !strings.Contains(view, "Clock: boom") {
		t.Fatalf("expected error line, got:\n%s", view)
	}
}
