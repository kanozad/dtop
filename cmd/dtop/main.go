package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kanozad/dtop/internal/app"
	"github.com/kanozad/dtop/internal/config"
	"github.com/kanozad/dtop/internal/plugin"
	"github.com/kanozad/dtop/plugins/battery"
	"github.com/kanozad/dtop/plugins/clock"
	"github.com/kanozad/dtop/plugins/cpu"
	"github.com/kanozad/dtop/plugins/gpu"
	"github.com/kanozad/dtop/plugins/memory"
	"github.com/kanozad/dtop/plugins/network"
	"github.com/kanozad/dtop/plugins/process"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	var (
		configPath  string
		themeName   string
		layoutMode  string
		presetSlot  string
		showVersion bool
		listThemes  bool
	)
	flag.StringVar(&configPath, "config", "", "path to config TOML (defaults to user config dir)")
	flag.StringVar(&themeName, "theme", "", "override theme name")
	flag.StringVar(&layoutMode, "layout", "", "override layout mode (vertical, grid, flow)")
	flag.StringVar(&presetSlot, "preset", "", "load preset slot on startup (0-9)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&listThemes, "list-themes", false, "list available themes and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("dtop %s\n", Version)
		return
	}

	if listThemes {
		printThemes()
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	resolvedConfigPath, err := config.ResolvePath(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx := context.Background()
	reg := plugin.NewRegistry()
	if err := reg.Register(func() plugin.Plugin { return clock.New() }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := reg.Register(func() plugin.Plugin { return cpu.New() }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := reg.Register(func() plugin.Plugin { return memory.New() }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := reg.Register(func() plugin.Plugin { return network.New() }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := reg.Register(func() plugin.Plugin { return process.New() }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := reg.Register(func() plugin.Plugin { return gpu.New() }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := reg.Register(func() plugin.Plugin { return battery.New() }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Apply CLI overrides.
	if themeName != "" {
		cfg.Theme.Name = themeName
	}
	if layoutMode != "" {
		switch layoutMode {
		case "vertical", "grid", "flow":
			cfg.Layout.Mode = layoutMode
		default:
			fmt.Fprintf(os.Stderr, "invalid layout mode %q (use vertical, grid, or flow)\n", layoutMode)
			os.Exit(1)
		}
	}
	if presetSlot != "" {
		if preset, ok := cfg.Presets[presetSlot]; ok {
			if preset.LayoutMode != "" {
				cfg.Layout.Mode = preset.LayoutMode
			}
			if preset.LayoutColumns > 0 {
				cfg.Layout.Columns = preset.LayoutColumns
			}
			if preset.UpdateInterval.Duration > 0 {
				cfg.UpdateInterval = preset.UpdateInterval
			}
		} else {
			fmt.Fprintf(os.Stderr, "preset %q not configured\n", presetSlot)
			os.Exit(1)
		}
	}

	plugins, err := reg.Instantiate(ctx, cfg.Plugins.Enabled, cfg.Plugins.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := app.Run(ctx, cfg, plugins, resolvedConfigPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printThemes() {
	fmt.Println("default (built-in)")
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	themesDir := filepath.Join(dir, "dtop", "themes")
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".toml") {
			fmt.Println(strings.TrimSuffix(name, ".toml"))
		}
	}
}
