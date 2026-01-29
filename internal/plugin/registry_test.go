package plugin

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/collector"
)

type registryPlugin struct {
	id          ID
	name        string
	allowed     []string
	initErr     error
	initCalled  bool
	receivedCfg map[string]any
}

func (p *registryPlugin) ID() ID   { return p.id }
func (p *registryPlugin) Name() string { return p.name }
func (p *registryPlugin) AllowedConfigKeys() []string {
	return p.allowed
}
func (p *registryPlugin) Init(_ context.Context, cfg map[string]any) error {
	p.initCalled = true
	p.receivedCfg = cfg
	return p.initErr
}
func (p *registryPlugin) Collect(context.Context) (collector.Data, error) {
	return nil, nil
}
func (p *registryPlugin) Shutdown(context.Context) error { return nil }
func (p *registryPlugin) Update(tea.Msg) tea.Cmd         { return nil }
func (p *registryPlugin) View(collector.Data, int, int, theme.Theme) string {
	return ""
}

func TestRegistryInstantiateOrder(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	if err := reg.Register(func() Plugin { return &registryPlugin{id: "a", name: "A"} }); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Register(func() Plugin { return &registryPlugin{id: "b", name: "B"} }); err != nil {
		t.Fatalf("register: %v", err)
	}

	plugins, err := reg.Instantiate(context.Background(), []string{"b", "a"}, nil)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].ID() != "b" || plugins[1].ID() != "a" {
		t.Fatalf("order not preserved: got %v, %v", plugins[0].ID(), plugins[1].ID())
	}
}

func TestRegistryInstantiateUnknown(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	if err := reg.Register(func() Plugin { return &registryPlugin{id: "a", name: "A"} }); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := reg.Instantiate(context.Background(), []string{"missing"}, nil); err == nil {
		t.Fatalf("expected error for unknown plugin")
	}
}

func TestRegistryInstantiateValidateConfig(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	if err := reg.Register(func() Plugin {
		return &registryPlugin{id: "a", name: "A", allowed: []string{"ok"}}
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := map[string]map[string]any{"a": {"bad": true}}
	if _, err := reg.Instantiate(context.Background(), []string{"a"}, cfg); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestRegistryInstantiateInitError(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	initErr := errors.New("init failed")
	if err := reg.Register(func() Plugin {
		return &registryPlugin{id: "a", name: "A", initErr: initErr}
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := reg.Instantiate(context.Background(), []string{"a"}, nil); err == nil {
		t.Fatalf("expected init error")
	}
}
