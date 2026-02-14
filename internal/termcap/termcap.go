package termcap

import (
	"os"
	"strings"

	"github.com/muesli/termenv"
)

type ColorLevel int

const (
	ColorANSI16 ColorLevel = iota
	ColorANSI256
	ColorTrueColor
)

type Capabilities struct {
	Color ColorLevel
	UTF8  bool
}

func Detect() Capabilities {
	return detectWithLookup(os.Getenv)
}

func detectWithLookup(getenv func(string) string) Capabilities {
	term := strings.ToLower(strings.TrimSpace(getenv("TERM")))
	colorTerm := strings.ToLower(strings.TrimSpace(getenv("COLORTERM")))
	noColor := strings.TrimSpace(getenv("NO_COLOR")) != ""

	locale := strings.TrimSpace(getenv("LC_ALL"))
	if locale == "" {
		locale = strings.TrimSpace(getenv("LC_CTYPE"))
	}
	if locale == "" {
		locale = strings.TrimSpace(getenv("LANG"))
	}
	locale = strings.ToLower(locale)

	return Capabilities{
		Color: detectColorLevel(term, colorTerm, noColor),
		UTF8:  detectUTF8(locale),
	}
}

func detectColorLevel(term, colorTerm string, noColor bool) ColorLevel {
	if noColor {
		return ColorANSI16
	}
	if strings.Contains(colorTerm, "truecolor") || strings.Contains(colorTerm, "24bit") {
		return ColorTrueColor
	}
	if strings.Contains(term, "truecolor") || strings.Contains(term, "24bit") {
		return ColorTrueColor
	}
	if strings.Contains(term, "256color") || strings.Contains(colorTerm, "256") {
		return ColorANSI256
	}
	return ColorANSI16
}

func detectUTF8(locale string) bool {
	if locale == "" {
		// Most modern environments are UTF-8 by default; keep the rich path.
		return true
	}
	return strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8")
}

func (c Capabilities) ColorProfile() termenv.Profile {
	switch c.Color {
	case ColorTrueColor:
		return termenv.TrueColor
	case ColorANSI256:
		return termenv.ANSI256
	default:
		return termenv.ANSI
	}
}
