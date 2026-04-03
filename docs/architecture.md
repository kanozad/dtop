# Architecture

This document summarizes the major components and data flow in DTOP, aligned with the reverse‑engineered btop++ spec while remaining language‑neutral for future ports.

## Runtime model
- `cmd/dtop/main.go` loads config, registers plugins, instantiates enabled plugins, and calls `app.Run`.
- `internal/app/app.go` constructs the Bubble Tea program, loads the selected theme, and orchestrates shutdown.
- `internal/app/model.go` owns the Bubble Tea model and schedules periodic ticks (global `update_ms`, per‑plugin overrides). On each tick it:
  - fan‑outs a collection request to enabled plugins (goroutine per plugin);
  - gathers immutable snapshots (drop partials, keep last good data if a plugin errors);
  - triggers a redraw flag for the view.
- Shutdown: cancel the shared context, wait for plugin drains, then restore terminal.

## Shared data contracts (language‑neutral)
- Buffers sized to the current viewport width; trim oldest on resize.
- CPU snapshot: total % history, per‑core % histories, optional temps/temps_max, load averages, optional watts, frequency (per‑core and avg MHz), container type and effective CPUs.
- Memory snapshot: mem/swap stats, disk list with capacities and I/O deltas, historical % for mem/swap/zfs_arc.
- Network snapshot: per‑direction bandwidth history, per‑iface totals/peaks, IPv4/IPv6, link state.
- Process snapshot: pid, ppid, user, state, cmd (full/short), threads, nice, cpu%, mem bytes, start time, tree metadata (depth/prefix/collapsed), optional smaps detail cache.
- GPU snapshot (optional): per‑GPU utilization %, mem %, temps, clocks, power, PCIe tx/rx, encoder/decoder % plus supported feature flags.
- Battery snapshot (optional): charge %, status (charging/discharging/full), capacity health, power supply path.
- Errors are attached to the plugin snapshot, not global state, to avoid overwriting unrelated modules.

## UI loop and rendering
- Layout supports vertical, grid, and flow modes via `internal/ui` helpers; flow auto-computes columns to fit height while enforcing per-box minimums.
- Reactive sizing: each plugin declares a `SizeHint` (MinH, PrefH, MaxH, Weight). The layout engine allocates minimums first, then distributes surplus height proportionally by weight, capping at MaxH. Compact plugins (battery: MaxH=3, clock: MaxH=1) stop growing early; data-heavy plugins (process, CPU) absorb remaining space. This replaces the previous equal-split approach.
- Column pinning: in grid/flow modes, plugins can declare a preferred column via `column = <N>` (1-based) in their `[plugins.config.<id>]` section. `groupPluginsByColumn` honours these preferences; unpinned plugins are distributed evenly in declaration order.
- Box chrome accounting: each plugin box has non-content overhead (border + padding) exposed via `Theme.BoxChrome()`. The layout engine subtracts chrome before allocation. Plugins that cannot meet their MinH are hidden, and a muted warning is displayed.
- Render order: clear region → borders → text/graphs → highlights → flush; prefer alt‑screen and synchronized output when supported.
- Graphs auto‑scale; net graphs never below 10 KiB/s; clamp history length to drawn width.
- Color downgrade: truecolor → 256 → 16‑color; themes must provide fallbacks.
- UTF‑8 only; width measured via wcwidth; truncate with ellipsis on overflow.

### Rendering components (`internal/ui`)
- `graph.go`: braille sparkline renderer (`RenderGraph`) using Unicode braille (U+2800–U+28FF). Each cell encodes a 2×4 dot grid. Supports configurable min/max scale, fill vs line mode, and per-graph lipgloss styling.
- `meter.go`: percentage bar renderer using Unicode block elements (█░). `RenderMeter` for full bars (label + [bar] + pct) and `RenderMiniMeter` for compact bars without brackets. Both accept `MeterOpts` with fill/empty styles.
- `layout.go`: height/width splitting, grid column distribution, flow column computation, `SizeHint`-based reactive allocation (`AllocateHeights`).
- `dialog.go`: generic bordered dialog overlay (`RenderDialog`) with title, body, and optional action buttons. `PlaceOverlay` centers content in the viewport via `lipgloss.Place`.
- `menu.go`: scrollable vertical menu (`MenuState`, `RenderMenu`) with cursor prefix, clamped selection, and offset-based scroll window.

### Global interaction (`internal/app/model.go`)
- Header shows app name, uptime, current update interval, and keybinding hints.
- `q` / Ctrl+C quits; `1`–`4` toggle CPU/Memory/Network/Process boxes; `+`/`-` adjust update interval; `?`/`h` toggle help overlay; `t` opens theme picker.
- Mouse SGR support enabled (`tea.WithMouseCellMotion()`); `tea.MouseMsg` delegated to plugins (e.g. process list wheel scroll ±3 rows).
- Box visibility tracked in `hiddenBoxes` map; rendering filters to visible plugins before layout.
- Theme picker: `openThemePicker()` scans `~/.config/dtop/themes/` for `.toml` files; `applySelectedTheme()` loads and applies the chosen theme live.

## Plugin system
- `internal/plugin` defines plugin interface, registry, config validation, and shutdown fan‑out.
- Plugins are enabled via `plugins.enabled` and validated against allowed keys.
- Shared data contracts live in `pkg/types` and are re‑exported through `pkg/collector`.
- Per‑plugin goroutine model mirrors btop’s runner: no overlapping collects; skip a tick if a collect is still running.

## Configuration and themes
- Config is TOML loaded from the user config directory (`dtop.conf`, legacy `dtop.toml` fallback).
- Themes: built‑in default or TOML in `~/.config/dtop/themes/<name>.toml`; downgrade path must be honored (truecolor → 256 → 16).
- Theme schema: see `docs/theme-schema.md`; ensure all graph/box colors have fallbacks.
- Runtime terminal capability detection applies color profile fallback (truecolor/256/16) and UTF-8 awareness for ASCII-safe rendering.

## Error handling and fallbacks
- Missing capabilities (temps, watts, GPU libs) → mark values “N/A”, log once at INFO then DEBUG.
- Permission issues (signals, wattage, GPU) → degrade feature, surface hint in UI/help.
- Terminal lacks UTF‑8/colors → force TTY/ASCII graph set.
- If collection fails in a tick, reuse last good snapshot; never render partial data.

## Performance and acceptance targets
- Input latency <100 ms at 2s update interval.
- Collector overhead <2% single core with ~1k processes.
- Memory budget <60 MB without smaps; <120 MB with smaps.
- Render budget <30 ms/frame at 120 columns; first frame <500 ms on Linux.

## Platform and portability
- Platform separation via build tags per plugin collector (linux/darwin/bsd) with identical contracts.
- Avoid global mutable state; hand off snapshots immutably to the renderer.
- Optional GPU support behind build tags/feature flags; FFI shims for NVML/ROCm if added.

## Extending the system
1. Implement `plugin.Plugin` (collector + Bubble Tea view/update) and optional `ConfigValidator`.
2. Register in `cmd/dtop/main.go`.
3. Document config keys in `docs/dtop.toml.example`.
4. Provide viewport‑width‑bounded histories and color fallbacks consistent with the rules above.
