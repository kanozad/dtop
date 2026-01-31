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
- CPU snapshot: total % history, per‑core % histories, optional temps/temps_max, load averages, optional watts.
- Memory snapshot: mem/swap stats, disk list with capacities and I/O deltas, historical % for mem/swap/zfs_arc.
- Network snapshot: per‑direction bandwidth history, per‑iface totals/peaks, IPv4/IPv6, link state.
- Process snapshot: pid, ppid, user, state, cmd (full/short), threads, nice, cpu%, mem bytes, start time, tree metadata (depth/prefix/collapsed), optional smaps detail cache.
- GPU snapshot (optional): per‑GPU utilization %, mem %, temps, clocks, power, PCIe tx/rx, encoder/decoder % plus supported feature flags.
- Errors are attached to the plugin snapshot, not global state, to avoid overwriting unrelated modules.

## UI loop and rendering
- Layout currently vertical via `internal/ui.SplitHeights`; future layouts should still enforce per‑box minimums and hide boxes that do not fit.
- Render order: clear region → borders → text/graphs → highlights → flush; prefer alt‑screen and synchronized output when supported.
- Graphs auto‑scale; net graphs never below 10 KiB/s; clamp history length to drawn width.
- Color downgrade: truecolor → 256 → 16‑color; themes must provide fallbacks.
- UTF‑8 only; width measured via wcwidth; truncate with ellipsis on overflow.

## Plugin system
- `internal/plugin` defines plugin interface, registry, config validation, and shutdown fan‑out.
- Plugins are enabled via `plugins.enabled` and validated against allowed keys.
- Shared data contracts live in `pkg/types` and are re‑exported through `pkg/collector`.
- Per‑plugin goroutine model mirrors btop’s runner: no overlapping collects; skip a tick if a collect is still running.

## Configuration and themes
- Config is TOML loaded from the user config directory (`dtop.conf`, legacy `dtop.toml` fallback).
- Themes: built‑in default or TOML in `~/.config/dtop/themes/<name>.toml`; downgrade path must be honored (truecolor → 256 → 16).
- Theme schema: see `docs/theme-schema.md`; ensure all graph/box colors have fallbacks.

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
