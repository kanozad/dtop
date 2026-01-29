package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"mld.com/dtop/internal/app"
	"mld.com/dtop/internal/config"
	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/plugins/clock"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config TOML (defaults to user config dir)")
	flag.Parse()

	cfg, err := config.Load(configPath)
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

	plugins, err := reg.Instantiate(ctx, cfg.Plugins.Enabled, cfg.Plugins.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := app.Run(ctx, cfg, plugins); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
