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
- `fg_256` (string): foreground fallback for 256-color terminals. Used instead of `fg` when truecolor is not available.
- `bg_256` (string): background fallback for 256-color terminals.
- `fg_16` (string): foreground fallback for 16-color terminals. Used instead of `fg`/`fg_256` when only ANSI 16 colors are available.
- `bg_16` (string): background fallback for 16-color terminals.
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

## Bundled themes
The `docs/` directory includes ready-to-use themes inspired by the Tokyo Night color palette:
- `tokyonight-moon.toml` — dark indigo base (`#222436`), soft lavender text, cyan/green/amber accents
- `tokyonight-storm.toml` — slightly warmer dark base (`#24283b`), teal borders, green/amber/pink accents

Both themes include truecolor, 256-color, and 16-color fallbacks.

### Installation
Copy the theme file to the dtop themes directory and set the name in your config:
```
mkdir -p ~/.config/dtop/themes
cp docs/tokyonight-storm.toml ~/.config/dtop/themes/
```
Then in `~/.config/dtop/dtop.conf`:
```
[theme]
name = "tokyonight-storm"
```

## Notes

- Colors are passed directly to `lipgloss.Color`, so any value it supports should work.
- If both `padding` and `padding_x`/`padding_y` are set, the axis values win for that axis.
- Fallback color resolution: when the terminal supports fewer colors, the most specific fallback is used. For example, on a 16-color terminal, `fg_16` is preferred; if absent, `fg` is used (lipgloss will auto-downgrade). Providing explicit fallbacks ensures the best appearance at each color depth.
- Terminal capabilities are auto-detected from `$COLORTERM`, `$TERM`, `$NO_COLOR`, and locale variables. The detected level is stored in `Theme.ColorLevel`.
