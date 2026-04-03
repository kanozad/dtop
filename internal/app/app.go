package app

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kanozad/dtop/internal/config"
	"github.com/kanozad/dtop/internal/plugin"
	"github.com/kanozad/dtop/internal/termcap"
	"github.com/kanozad/dtop/internal/theme"
)

func Run(ctx context.Context, cfg config.Config, plugins []plugin.Plugin, configPath string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	caps := termcap.Detect()
	lipgloss.SetColorProfile(caps.ColorProfile())

	th, err := theme.FromName(cfg.Theme.Name)
	if err != nil {
		return err
	}
	th = th.WithCapabilities(caps)
	m := NewModel(ctx, cancel, cfg, th, plugins, configPath)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, runErr := p.Run()

	cancel()
	shutdownErr := plugin.ShutdownAll(ctx, plugins)
	return errors.Join(runErr, shutdownErr)
}
