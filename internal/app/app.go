package app

import (
	"context"
	"errors"
	"time"

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
	// Shutdown gets its own bounded context: the one above is now cancelled,
	// so reusing it would defeat any ctx-aware cleanup a plugin performs.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	shutdownErr := plugin.ShutdownAll(shutdownCtx, plugins)
	return errors.Join(runErr, shutdownErr)
}
