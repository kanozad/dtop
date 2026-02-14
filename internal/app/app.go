package app

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mld.com/dtop/internal/config"
	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/termcap"
	"mld.com/dtop/internal/theme"
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
