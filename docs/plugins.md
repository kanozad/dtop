# Plugin development guide

This project loads plugins via `internal/plugin.Registry`. Each plugin implements `internal/plugin.Plugin`, which embeds the `collector.Collector` lifecycle and adds rendering hooks.

## Plugin interface

```go
// internal/plugin/plugin.go
type Plugin interface {
    ID() ID
    Name() string
    collector.Collector   // Init, Collect, Shutdown
    Update(msg tea.Msg) tea.Cmd
    View(data collector.Data, width, height int, th theme.Theme) string
}

Optional: implement `SizeHinter` to declare vertical sizing preferences:

```go
type SizeHinter interface {
    SizeHint() ui.SizeHint
}
```

Implement `ConfigValidator` to advertise allowed config keys:

```go
type ConfigValidator interface {
    AllowedConfigKeys() []string
}
```

## Sizing and Layout

Plugins can influence how much vertical space they receive by implementing the `SizeHinter` interface. The layout engine uses these hints to distribute the available terminal height.

```go
func (c *Clock) SizeHint() ui.SizeHint {
    return ui.SizeHint{
        MinH:   1, // Minimum 1 row of content
        MaxH:   1, // Never grow beyond 1 row
        Weight: 0, // No weight for surplus distribution
    }
}
```

`ui.SizeHint` fields:
- `MinH`: Absolute minimum content rows. Below this, the plugin is hidden.
- `PrefH`: Preferred content rows (currently informational).
- `MaxH`: Maximum content rows. The plugin won't grow beyond this even if space is available.
- `Weight`: Relative priority when distributing surplus height. High weight plugins (like `Process`) expand to fill space, while low weight or capped plugins (like `Battery` or `Clock`) stay small.

Plugins that don't implement `SizeHinter` receive a default hint: `{MinH: 3, Weight: 1}`.

## Lifecycle

1. **Registration** — A factory function is registered with the `Registry` in `cmd/dtop/main.go`. The factory is called to probe the plugin's `ID()` and stored for later instantiation.

2. **Instantiation** — When the app starts, `Registry.Instantiate()` calls the factory for each ID listed in `plugins.enabled`. If the plugin implements `ConfigValidator`, unknown config keys are rejected. Then `Init(ctx, cfg)` is called with the plugin's config map.

3. **Collection** — The scheduler calls `Collect(ctx)` on a goroutine at the plugin's configured interval (global `update_interval` or per-plugin `interval` override). Collections are non-overlapping: if a previous collect is still running, the current tick is skipped. Return an immutable data snapshot implementing `collector.Data` (which is `any`).

4. **Rendering** — `View(data, width, height, theme)` is called on the main goroutine each frame. It receives the latest snapshot from `Collect`, the available box dimensions (including chrome), and the current theme. Return a string to be rendered inside the plugin's box.

5. **Input** — `Update(msg)` is called for each `tea.Msg` the model dispatches to plugins. Return a `tea.Cmd` if the plugin needs to trigger async work, or `nil`.

6. **Shutdown** — `Shutdown(ctx)` is called when the app exits. Clean up goroutines, file handles, or external resources.

## Data contracts

Define your data type in `pkg/types/`:

```go
// pkg/types/battery.go
type BatteryStats struct {
    Present       bool
    Capacity      float64
    Status        string
    PowerNowWatts *float64
    Error         string
}
```

Key rules:
- Use pointer types (`*float64`, `*time.Duration`) for values that may be unavailable.
- Include an `Error string` field to surface collection problems without breaking the UI.
- Keep types simple structs — they cross the goroutine boundary from collector to renderer.
- On collection failure, return the error to the framework; it will keep the last good snapshot.

## Config validation

Implement `AllowedConfigKeys()` to return the keys your plugin accepts. The `interval` key is always allowed automatically (reserved for the per-plugin scheduler cadence).

```go
func (b *Battery) AllowedConfigKeys() []string {
    return nil // no custom keys; only "interval" is allowed
}
```

Config values arrive as `map[string]any` from TOML parsing. Use type assertions:

```go
if v, ok := cfg["per_core"].(bool); ok {
    out.PerCore = v
}
```

Document your keys in `docs/dtop.toml.example` under `[plugins.config.<id>]`.

## View rendering

Use `internal/ui` helpers for consistent rendering:

- `ui.ContentWidth(boxWidth)` — subtract box chrome to get inner content width
- `ui.ContentHeight(boxHeight)` — subtract box chrome to get inner content height
- `ui.RenderMeter(label, pct, width, opts)` — full meter bar with label
- `ui.RenderMiniMeter(label, pct, width, opts)` — compact meter without brackets
- `ui.RenderGraph(data, width, height, opts)` — braille sparkline graph
- `ui.Truncate(s, width)` — truncate with ellipsis
- `ui.PushAndClamp(history, value, maxLen)` — append to ring buffer
- `ui.ResizeHistory(history, newLen)` — resize ring buffer on viewport change

Theme integration:
- Use `th.MeterFill` / `th.MeterEmpty` for meter styles
- Use `th.GraphCPU` / `th.GraphMem` / `th.GraphNet` for graph colors
- Use `th.Muted` for secondary text, `th.Error` for errors
- Use `th.RenderBox(title, body, width, height)` to wrap content in a themed box
- Check `th.UTF8` to fall back to ASCII-safe rendering when needed

Color fallbacks: themes may define `fg_256` and `fg_16` fallbacks alongside truecolor `fg`. The theme system auto-selects based on terminal capabilities — your View code doesn't need to handle this.

## Build tags and platform stubs

Use build tags to separate platform-specific collectors:

```
plugins/battery/
├── battery.go              # shared plugin struct, View, Update
├── collector_linux.go      # //go:build linux
├── collector_stub.go       # //go:build !linux
├── config.go               # config parsing
└── battery_test.go         # tests
```

The stub should return a graceful fallback (e.g. `Present: false`, or an error explaining the platform isn't supported). The View should handle these cases:

```go
if !stats.Present {
    body := th.Muted.Render("No battery detected")
    return th.RenderBox("Battery", body, width, height)
}
```

## Testing patterns

### View tests
Pass synthetic data directly to `View()` — no real collector needed:

```go
func TestViewWithBattery(t *testing.T) {
    b := New()
    th := theme.Default()
    stats := types.BatteryStats{Present: true, Capacity: 60, Status: "Discharging"}
    out := b.View(stats, 60, 10, th)
    if out == "" {
        t.Fatal("expected non-empty view")
    }
}
```

### Collector tests (Linux sysfs)
For collectors that read `/proc` or `/sys`, use `t.TempDir()` with synthetic fixture files:

```go
func TestReadLoadAvg(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "loadavg")
    os.WriteFile(path, []byte("0.50 1.00 1.50 1/100 123\n"), 0o644)
    load1, load5, load15, ok := readLoadAvg(path)
    // assert values...
}
```

For functions that read hardcoded paths (e.g. `/sys/class/power_supply/BAT*`), test at a higher level with `Collect()` and accept that the result depends on the environment. Guard assertions with the data's availability flags.

### Integration tests
Call `Collect(nil)` and verify the returned type and basic invariants. Don't assert specific values that depend on hardware.

## Creating a new plugin — step by step

1. Create `plugins/<id>/` with the files shown above.
2. Define your data type in `pkg/types/<id>.go`.
3. Implement the `Plugin` interface.
4. Register the factory in `cmd/dtop/main.go`:
   ```go
   reg.Register(func() plugin.Plugin { return myplugin.New() })
   ```
5. Add a config section to `docs/dtop.toml.example`.
6. Write tests.
7. Add your plugin ID to the `plugins.enabled` list in the example and your own config.

## Example: battery plugin

The `plugins/battery/` package is a minimal but complete reference implementation:

- **Types**: `pkg/types/battery.go` — `BatteryStats` with `Present`, `Capacity`, `Status`, energy, power, and time estimates.
- **Linux collector**: reads `/sys/class/power_supply/BAT*` sysfs attributes, computes time-to-empty/full from power draw.
- **Stub collector**: returns `Present: false` on non-Linux.
- **View**: meter bar for capacity, status/power/time info line, energy detail line. Handles no-battery and error states gracefully.
- **Tests**: view rendering with synthetic data, `formatDuration` unit test, `Collect()` integration test.
