package termcap

import "testing"

func envMapLookup(env map[string]string) func(string) string {
	return func(key string) string {
		return env[key]
	}
}

func TestDetectTrueColorUTF8(t *testing.T) {
	caps := detectWithLookup(envMapLookup(map[string]string{
		"COLORTERM": "truecolor",
		"LANG":      "en_US.UTF-8",
	}))
	if caps.Color != ColorTrueColor {
		t.Fatalf("expected truecolor, got %v", caps.Color)
	}
	if !caps.UTF8 {
		t.Fatalf("expected utf8 support")
	}
}

func TestDetectANSI256(t *testing.T) {
	caps := detectWithLookup(envMapLookup(map[string]string{
		"TERM": "xterm-256color",
		"LANG": "C.UTF-8",
	}))
	if caps.Color != ColorANSI256 {
		t.Fatalf("expected ansi256, got %v", caps.Color)
	}
}

func TestDetectNoColorAndNonUTF8(t *testing.T) {
	caps := detectWithLookup(envMapLookup(map[string]string{
		"NO_COLOR": "1",
		"LANG":     "C",
	}))
	if caps.Color != ColorANSI16 {
		t.Fatalf("expected ansi16 with NO_COLOR, got %v", caps.Color)
	}
	if caps.UTF8 {
		t.Fatalf("expected non-utf8 for C locale")
	}
}
