package theme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"

	"mld.com/dtop/internal/termcap"
)

var themeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type styleConfig struct {
	Fg        string `toml:"fg"`
	Bg        string `toml:"bg"`
	Fg256     string `toml:"fg_256"`
	Bg256     string `toml:"bg_256"`
	Fg16      string `toml:"fg_16"`
	Bg16      string `toml:"bg_16"`
	Bold      *bool  `toml:"bold"`
	Italic    *bool  `toml:"italic"`
	Underline *bool  `toml:"underline"`
}

type boxConfig struct {
	BorderFg    string `toml:"border_fg"`
	BorderBg    string `toml:"border_bg"`
	BorderStyle string `toml:"border"`
	Fg          string `toml:"fg"`
	Bg          string `toml:"bg"`
	Bold        *bool  `toml:"bold"`
	Italic      *bool  `toml:"italic"`
	Underline   *bool  `toml:"underline"`
	Padding     *int   `toml:"padding"`
	PaddingX    *int   `toml:"padding_x"`
	PaddingY    *int   `toml:"padding_y"`
}

type themeConfig struct {
	Header     styleConfig `toml:"header"`
	Text       styleConfig `toml:"text"`
	Muted      styleConfig `toml:"muted"`
	Error      styleConfig `toml:"error"`
	Box        boxConfig   `toml:"box"`
	BoxTitle   styleConfig `toml:"box_title"`
	GraphCPU   styleConfig `toml:"graph_cpu"`
	GraphMem   styleConfig `toml:"graph_mem"`
	GraphNet   styleConfig `toml:"graph_net"`
	MeterFill  styleConfig `toml:"meter_fill"`
	MeterEmpty styleConfig `toml:"meter_empty"`
	Highlight  styleConfig `toml:"highlight"`
}

type Theme struct {
	Header     lipgloss.Style
	Text       lipgloss.Style
	Muted      lipgloss.Style
	Error      lipgloss.Style
	Box        lipgloss.Style
	BoxTitle   lipgloss.Style
	GraphCPU   lipgloss.Style
	GraphMem   lipgloss.Style
	GraphNet   lipgloss.Style
	MeterFill  lipgloss.Style
	MeterEmpty lipgloss.Style
	Highlight  lipgloss.Style
	UTF8       bool
	ColorLevel termcap.ColorLevel

	// rawCfg is retained so WithCapabilities can re-apply styles with
	// color-level-appropriate fallback colors.
	rawCfg *themeConfig
}

func Default() Theme {
	boxBorder := lipgloss.RoundedBorder()
	return Theme{
		Header: lipgloss.NewStyle().Bold(true),
		Text:   lipgloss.NewStyle(),
		Muted:  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		Box: lipgloss.NewStyle().
			Border(boxBorder).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1),
		BoxTitle:   lipgloss.NewStyle().Bold(true),
		GraphCPU:   lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd7ff")),
		GraphMem:   lipgloss.NewStyle().Foreground(lipgloss.Color("#d7af5f")),
		GraphNet:   lipgloss.NewStyle().Foreground(lipgloss.Color("#87d787")),
		MeterFill:  lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafff")),
		MeterEmpty: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Highlight:  lipgloss.NewStyle().Bold(true).Reverse(true),
		UTF8:       true,
	}
}

func FromName(name string) (Theme, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "", "default":
		return Default(), nil
	}
	if !themeNamePattern.MatchString(normalized) {
		return Theme{}, fmt.Errorf("invalid theme name %q", name)
	}
	path, err := themePath(normalized)
	if err != nil {
		return Theme{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Theme{}, fmt.Errorf("unknown theme %q", name)
		}
		return Theme{}, fmt.Errorf("load theme %q: %w", name, err)
	}
	var cfg themeConfig
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return Theme{}, fmt.Errorf("parse theme %q: %w", name, err)
	}

	th := Default()
	th.rawCfg = &cfg
	applyStyle(&th.Header, cfg.Header, termcap.ColorTrueColor)
	applyStyle(&th.Text, cfg.Text, termcap.ColorTrueColor)
	applyStyle(&th.Muted, cfg.Muted, termcap.ColorTrueColor)
	applyStyle(&th.Error, cfg.Error, termcap.ColorTrueColor)
	applyBox(&th.Box, cfg.Box)
	applyStyle(&th.BoxTitle, cfg.BoxTitle, termcap.ColorTrueColor)
	applyStyle(&th.GraphCPU, cfg.GraphCPU, termcap.ColorTrueColor)
	applyStyle(&th.GraphMem, cfg.GraphMem, termcap.ColorTrueColor)
	applyStyle(&th.GraphNet, cfg.GraphNet, termcap.ColorTrueColor)
	applyStyle(&th.MeterFill, cfg.MeterFill, termcap.ColorTrueColor)
	applyStyle(&th.MeterEmpty, cfg.MeterEmpty, termcap.ColorTrueColor)
	applyStyle(&th.Highlight, cfg.Highlight, termcap.ColorTrueColor)
	return th, nil
}

func (t Theme) WithCapabilities(caps termcap.Capabilities) Theme {
	t.UTF8 = caps.UTF8
	t.ColorLevel = caps.Color
	if t.rawCfg != nil && caps.Color < termcap.ColorTrueColor {
		// Re-apply styles using fallback colors appropriate for the
		// detected color level.
		cfg := t.rawCfg
		applyStyle(&t.Header, cfg.Header, caps.Color)
		applyStyle(&t.Text, cfg.Text, caps.Color)
		applyStyle(&t.Muted, cfg.Muted, caps.Color)
		applyStyle(&t.Error, cfg.Error, caps.Color)
		applyStyle(&t.BoxTitle, cfg.BoxTitle, caps.Color)
		applyStyle(&t.GraphCPU, cfg.GraphCPU, caps.Color)
		applyStyle(&t.GraphMem, cfg.GraphMem, caps.Color)
		applyStyle(&t.GraphNet, cfg.GraphNet, caps.Color)
		applyStyle(&t.MeterFill, cfg.MeterFill, caps.Color)
		applyStyle(&t.MeterEmpty, cfg.MeterEmpty, caps.Color)
		applyStyle(&t.Highlight, cfg.Highlight, caps.Color)
	}
	return t
}

// BoxChrome returns the total vertical and horizontal non-content overhead
// added by RenderBox (border + padding). Callers use this to reserve space
// for box chrome when splitting heights/widths across multiple boxes.
func (t Theme) BoxChrome() (vertical, horizontal int) {
	v := t.Box.GetBorderTopSize() + t.Box.GetBorderBottomSize() +
		t.Box.GetPaddingTop() + t.Box.GetPaddingBottom()
	h := t.Box.GetBorderLeftSize() + t.Box.GetBorderRightSize() +
		t.Box.GetPaddingLeft() + t.Box.GetPaddingRight()
	return v, h
}

func (t Theme) RenderBox(title, body string, width, height int) string {
	content := strings.TrimRight(body, "\n")
	if title != "" {
		content = t.BoxTitle.Render(title) + "\n" + content
	}

	vChrome, _ := t.BoxChrome()
	box := t.Box
	if width > 0 {
		// width is the outer box width. Width() in lipgloss sets the content area including
		// padding but not borders. So subtract only the border widths to get outer = width.
		borderH := box.GetBorderLeftSize() + box.GetBorderRightSize()
		cw := width - borderH
		if cw < 1 {
			cw = 1
		}
		box = box.Width(cw)
	}
	if height > 0 {
		// height is inner content rows; outer = height + vChrome. Cap outer at that value
		// without forcing Height() padding, so the bottom border is never clipped.
		box = box.MaxHeight(height + vChrome)
		lines := strings.Split(content, "\n")
		if len(lines) > height {
			content = strings.Join(lines[:height], "\n")
		}
	}
	return box.Render(content)
}

func themePath(name string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dtop", "themes", name+".toml"), nil
}

// resolveFg picks the best foreground color for the given terminal color level.
// Fallback priority: fg_16 < fg_256 < fg (truecolor).
func resolveFg(cfg styleConfig, level termcap.ColorLevel) string {
	switch level {
	case termcap.ColorANSI16:
		if cfg.Fg16 != "" {
			return cfg.Fg16
		}
		return cfg.Fg
	case termcap.ColorANSI256:
		if cfg.Fg256 != "" {
			return cfg.Fg256
		}
		return cfg.Fg
	default:
		return cfg.Fg
	}
}

// resolveBg picks the best background color for the given terminal color level.
func resolveBg(cfg styleConfig, level termcap.ColorLevel) string {
	switch level {
	case termcap.ColorANSI16:
		if cfg.Bg16 != "" {
			return cfg.Bg16
		}
		return cfg.Bg
	case termcap.ColorANSI256:
		if cfg.Bg256 != "" {
			return cfg.Bg256
		}
		return cfg.Bg
	default:
		return cfg.Bg
	}
}

func applyStyle(target *lipgloss.Style, cfg styleConfig, level termcap.ColorLevel) {
	s := *target
	if fg := resolveFg(cfg, level); fg != "" {
		s = s.Foreground(lipgloss.Color(fg))
	}
	if bg := resolveBg(cfg, level); bg != "" {
		s = s.Background(lipgloss.Color(bg))
	}
	if cfg.Bold != nil {
		s = s.Bold(*cfg.Bold)
	}
	if cfg.Italic != nil {
		s = s.Italic(*cfg.Italic)
	}
	if cfg.Underline != nil {
		s = s.Underline(*cfg.Underline)
	}
	*target = s
}

func borderFromName(name string) (lipgloss.Border, bool) {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "rounded":
		return lipgloss.RoundedBorder(), true
	case "normal":
		return lipgloss.NormalBorder(), true
	case "thick":
		return lipgloss.ThickBorder(), true
	case "double":
		return lipgloss.DoubleBorder(), true
	case "hidden", "none":
		return lipgloss.HiddenBorder(), true
	default:
		return lipgloss.Border{}, false
	}
}

func applyBox(target *lipgloss.Style, cfg boxConfig) {
	s := *target
	if cfg.BorderStyle != "" {
		if border, ok := borderFromName(cfg.BorderStyle); ok {
			s = s.Border(border)
		}
	}
	if cfg.BorderFg != "" {
		s = s.BorderForeground(lipgloss.Color(cfg.BorderFg))
	}
	if cfg.BorderBg != "" {
		s = s.BorderBackground(lipgloss.Color(cfg.BorderBg))
	}
	if cfg.Fg != "" {
		s = s.Foreground(lipgloss.Color(cfg.Fg))
	}
	if cfg.Bg != "" {
		s = s.Background(lipgloss.Color(cfg.Bg))
	}
	if cfg.Bold != nil {
		s = s.Bold(*cfg.Bold)
	}
	if cfg.Italic != nil {
		s = s.Italic(*cfg.Italic)
	}
	if cfg.Underline != nil {
		s = s.Underline(*cfg.Underline)
	}
	if cfg.Padding != nil || cfg.PaddingX != nil || cfg.PaddingY != nil {
		top, right, bottom, left := 0, 1, 0, 1
		if cfg.Padding != nil {
			top, right, bottom, left = *cfg.Padding, *cfg.Padding, *cfg.Padding, *cfg.Padding
		}
		if cfg.PaddingY != nil {
			top, bottom = *cfg.PaddingY, *cfg.PaddingY
		}
		if cfg.PaddingX != nil {
			left, right = *cfg.PaddingX, *cfg.PaddingX
		}
		s = s.Padding(top, right, bottom, left)
	}
	*target = s
}
