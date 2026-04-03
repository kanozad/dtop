package plugin

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/pkg/collector"
)

type stubPlugin struct {
	id          ID
	name        string
	shutdownErr error
}

func (p *stubPlugin) ID() ID       { return p.id }
func (p *stubPlugin) Name() string { return p.name }
func (p *stubPlugin) Init(context.Context, map[string]any) error {
	return nil
}
func (p *stubPlugin) Collect(context.Context) (collector.Data, error) {
	return nil, nil
}
func (p *stubPlugin) Shutdown(context.Context) error { return p.shutdownErr }
func (p *stubPlugin) Update(tea.Msg) tea.Cmd         { return nil }
func (p *stubPlugin) View(collector.Data, int, int, theme.Theme) string {
	return ""
}

func TestValidateConfigUnknownKeys(t *testing.T) {
	t.Parallel()

	cfg := map[string]any{"good": 1, "bad": true}
	err := ValidateConfig("demo", cfg, "good")
	if err == nil {
		t.Fatalf("expected error for unknown keys")
	}
}

func TestValidateConfigEmpty(t *testing.T) {
	t.Parallel()

	if err := ValidateConfig("demo", nil, "a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigAllowsGlobalIntervalKey(t *testing.T) {
	t.Parallel()

	cfg := map[string]any{GlobalPluginIntervalKey: "1s"}
	if err := ValidateConfig("demo", cfg); err != nil {
		t.Fatalf("expected interval key to be accepted, got %v", err)
	}
}

func TestShutdownAllJoinsErrors(t *testing.T) {
	t.Parallel()

	errA := errors.New("a")
	errB := errors.New("b")
	plugins := []Plugin{
		&stubPlugin{id: "a", name: "A", shutdownErr: errA},
		nil,
		&stubPlugin{id: "b", name: "B", shutdownErr: errB},
	}

	err := ShutdownAll(context.Background(), plugins)
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("expected joined errors, got %v", err)
	}
}
