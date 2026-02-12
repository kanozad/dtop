# Theme schema

Theme files live at `~/.config/dtop/themes/<name>.toml`, where `<name>` matches:

```
^[a-z0-9][a-z0-9_-]*$
```

Unknown theme names fail at startup with a clear error.

## Schema overview

Each section below is optional. Any values omitted fall back to the built-in defaults.

### Common style fields

Supported in: `[header]`, `[text]`, `[muted]`, `[error]`, `[box_title]`, `[graph_cpu]`, `[graph_mem]`, `[graph_net]`, `[meter_fill]`, `[meter_empty]`, `[highlight]`

- `fg` (string): foreground color (e.g., `"7"`, `"15"`, or a hex value like `"#ffcc00"`).
- `bg` (string): background color.
- `bold` (bool)
- `italic` (bool)
- `underline` (bool)

### Box fields

Supported in: `[box]`

- `border` (string): one of `rounded`, `normal`, `thick`, `double`, `hidden`, `none`.
- `border_fg` (string)
- `border_bg` (string)
- `fg` (string)
- `bg` (string)
- `bold` (bool)
- `italic` (bool)
- `underline` (bool)
- `padding` (int): sets all sides.
- `padding_x` (int): sets left/right; overrides `padding` for X axis.
- `padding_y` (int): sets top/bottom; overrides `padding` for Y axis.

## Notes

- Colors are passed directly to `lipgloss.Color`, so any value it supports should work.
- If both `padding` and `padding_x`/`padding_y` are set, the axis values win for that axis.
