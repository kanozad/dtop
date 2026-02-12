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

func TestModelRenderPluginsVerticalChromeAccounting(t *testing.T) {
	t.Parallel()

	// 3 plugins, 24 rows. Each box has 2 lines of chrome.
	// Content budget = 24 - 2*3 = 18, split 3 ways = 6 each.
	// Total rendered per box = 6 (content) + 2 (chrome) = 8; 8*3 = 24. Fits exactly.
	plugins := []plugin.Plugin{
		&modelPlugin{id: "a", name: "A"},
		&modelPlugin{id: "b", name: "B"},
		&modelPlugin{id: "c", name: "C"},
	}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), plugins)
	m.width = 80
	m.height = 26 // 24 body + 2 header

	view := m.View()
	if strings.Contains(view, "hidden") {
		t.Fatalf("expected no hidden boxes, got:\n%s", view)
	}
}

func TestModelRenderPluginsHiddenWarning(t *testing.T) {
	t.Parallel()

	// 5 plugins in vertical mode, only 12 body rows.
	// Content budget = 12 - 2*5 = 2, minPluginHeight=3 per plugin.
	// 2 total content rows can't give 3 to any of the 5 plugins.
	cfg := config.Default()
	cfg.Layout.Mode = "vertical"
	plugins := []plugin.Plugin{
		&modelPlugin{id: "a", name: "A"},
		&modelPlugin{id: "b", name: "B"},
		&modelPlugin{id: "c", name: "C"},
		&modelPlugin{id: "d", name: "D"},
		&modelPlugin{id: "e", name: "E"},
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), plugins)
	m.width = 80
	m.height = 14 // 12 body + 2 header

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Fatalf("expected hidden-box warning, got:\n%s", view)
	}
}

func TestModelViewNoPlugins(t *testing.T) {
	t.Parallel()

	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), nil)
	m.width = 80
	m.height = 10

	view := m.View()
	if !strings.Contains(view, "No plugins visible") {
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
