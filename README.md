# DTOP

DTOP is a Go TUI system monitor inspired by btop++ with a plugin-based architecture built on Bubble Tea and lipgloss.

## Status

Phase 2 is complete. The current built-in plugins are:

- `clock`
- `cpu` (Linux collector; optional temperature if available)
- `memory`
- `network`
- `process` (Linux collector; process list with sorting, filtering, tree view, scrolling)

## Requirements

- Go 1.24+

## Run

```bash
go run ./cmd/dtop
```

DTOP reads configuration from `~/.config/dtop/dtop.conf` (or `$XDG_CONFIG_HOME/dtop/dtop.conf`).  
If that does not exist, it falls back to `~/.config/dtop/dtop.toml`.

An example config lives at `docs/dtop.toml.example`.

## Configuration

- Global interval: `update_interval = "2s"`
- Optional live reload: `live_reload = true` (reloads when config file changes)
- Theme: `theme.name = "default"` (custom themes in `~/.config/dtop/themes/<name>.toml`)
- Bundled themes: `tokyonight-moon`, `tokyonight-storm` (see `docs/theme-schema.md` for installation)
- Layout: `layout.mode` ("grid", "flow", "vertical"); `layout.columns` sets fixed columns for grid mode
- Plugins: list in `plugins.enabled`, config under `plugins.config.<plugin_id>`
- Presets: `[presets.<slot>]` for slot `0-9` with `layout_mode`, `layout_columns`, `update_interval`, `visible_boxes`

CPU plugin options:

- `per_core` (bool): show per-core utilization lines
- `show_temp` (bool): attempt to read CPU temperature (Linux)

Process plugin options:

- `tree_view` (bool): show process tree with parent-child relationships
- `sort_by` (string): sort by pid, name, cpu, mem, threads, or user
- `filter` (string): filter processes by name/command (case-insensitive)
- `max_display` (int): maximum number of processes to display
- `follow_pid` (int): follow a specific PID on startup (0 disables)
- `use_smaps` (bool): use /proc/[pid]/smaps for detailed memory (slower)

Process list controls: up/down to move selection (">" indicator), PgUp/PgDown to page, `f`/`F3` filter edit mode, `F` follow toggle, `Enter` detail view, `c` collapse/expand, `x`/`X` signal chooser, `r`/`R` renice +1/-1. Signal/renice actions are permission-gated and will report errors in the header.

Preset controls: press `p`, then slot `0-9` to load; `P` then slot to save; `D` then slot to delete; `E` then slot to export a single slot to `<config-dir>/presets/preset-<slot>.toml`; `I` then slot to import that exported slot file and apply it immediately.

## Tests and formatting

```bash
go test ./...
gofmt -w .
```

Tip: the test command is `go test ./...` with no space between `./` and the ellipsis; `go test ./ ...` is interpreted as two paths (`.` and `../..`) and will fail.

Common pitfalls:

- Example config: start from `docs/dtop.toml.example`, copy to `~/.config/dtop/dtop.toml`, then tweak.
- If you add a new plugin, remember to register its factory in `cmd/dtop/main.go` or it won’t be instantiated.

## Tooling

- Lint: `golangci-lint run` (config in `.golangci.yml`)
- Vet: `go vet ./...`
- Make targets: `make lint vet test fmt tidy`

## Install and release

- Install the CLI locally: `go install ./cmd/dtop@latest`
- Cutting a release: tag the repo (e.g., `v0.2.0`); users can then `go install mld.com/dtop/cmd/dtop@v0.2.0`.

## Docs

- `docs/architecture.md` — high-level architecture and data flow
- `docs/dtop-requirements.md` — roadmap and design goals
- `docs/theme-schema.md` — theme file schema and bundled themes
- `docs/plugins.md` — how to build/register plugins
