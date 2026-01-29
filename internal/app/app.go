package app

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/config"
	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
)

func Run(ctx context.Context, cfg config.Config, plugins []plugin.Plugin) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	th, err := theme.FromName(cfg.Theme.Name)
	if err != nil {
		return err
	}
	m := NewModel(ctx, cancel, cfg, th, plugins)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, runErr := p.Run()

	cancel()
	shutdownErr := plugin.ShutdownAll(ctx, plugins)
	return errors.Join(runErr, shutdownErr)
}
