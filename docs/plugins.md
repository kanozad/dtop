# Plugin development guide

This project loads plugins via `internal/plugin.Registry`. Each plugin implements `internal/plugin.Plugin`, which embeds the `collector.Collector` lifecycle and adds rendering hooks.

## Minimal contract

- `ID() plugin.ID` and `Name() string`
- Collector lifecycle: `Init(ctx, cfg) error`, `Collect(ctx) (collector.Data, error)`, `Shutdown(ctx) error`
- UI hooks: `Update(msg tea.Msg) tea.Cmd` and `View(data collector.Data, width, height int, th theme.Theme) string`
- Optional config validation: implement `AllowedConfigKeys() []string`; unknown keys are rejected.

## Creating a plugin

1) Create a package under `plugins/<id>` and implement the interface:

```go path=null start=null
type MyPlugin struct{}

func New() *MyPlugin { return &MyPlugin{} }
func (p *MyPlugin) ID() plugin.ID   { return "myplugin" }
func (p *MyPlugin) Name() string    { return "My Plugin" }
func (p *MyPlugin) AllowedConfigKeys() []string { return []string{"interval"} }
func (p *MyPlugin) Init(ctx context.Context, cfg map[string]any) error { /* parse cfg */ return nil }
func (p *MyPlugin) Collect(ctx context.Context) (collector.Data, error) { /* return data */ }
func (p *MyPlugin) Update(tea.Msg) tea.Cmd { return nil }
func (p *MyPlugin) View(data collector.Data, w, h int, th theme.Theme) string { /* render */ }
func (p *MyPlugin) Shutdown(ctx context.Context) error { return nil }
```

2) Register the factory in `cmd/dtop/main.go`:

```go path=/home/kanozad/dev/workspaces/go-workspace/dtop/cmd/dtop/main.go start=21
reg := plugin.NewRegistry()
reg.Register(func() plugin.Plugin { return clock.New() })
reg.Register(func() plugin.Plugin { return cpu.New() })
// reg.Register(func() plugin.Plugin { return myplugin.New() })
```

3) Wire configuration: add defaults in `docs/dtop.toml.example` and use keys under `[plugins.config.<id>]`.

4) Keep `collector.Data` types simple (e.g., structs or primitive values). `View` should handle empty/invalid data gracefully.

5) Remember to clean up external resources in `Shutdown`.
