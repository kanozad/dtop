package ui

import (
	"strings"
	"testing"
)

func TestRenderMeter_ZeroWidth(t *testing.T) {
	if out := RenderMeter("X", 50, 0, MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()}); out != "" {
		t.Errorf("expected empty, got %q", out)
	}
}

func TestRenderMeter_NarrowFallback(t *testing.T) {
	out := RenderMeter("RAM", 50, 12, MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()})
	if !strings.Contains(out, "RAM") {
		t.Errorf("expected label in output, got %q", out)
	}
	// Should not contain brackets in compact mode.
}

func TestRenderMeter_NormalWidth(t *testing.T) {
	opts := MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()}
	out := RenderMeter("RAM", 50, 40, opts)
	if !strings.Contains(out, "[") || !strings.Contains(out, "]") {
		t.Errorf("expected brackets in output, got %q", out)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("expected fill chars, got %q", out)
	}
	if !strings.Contains(out, "░") {
		t.Errorf("expected empty chars, got %q", out)
	}
	if !strings.Contains(out, "50.0%") {
		t.Errorf("expected percentage, got %q", out)
	}
}

func TestRenderMeter_ZeroPercent(t *testing.T) {
	opts := MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()}
	out := RenderMeter("CPU", 0, 40, opts)
	if strings.Contains(out, "█") {
		t.Errorf("0%% should have no fill chars, got %q", out)
	}
}

func TestRenderMeter_HundredPercent(t *testing.T) {
	opts := MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()}
	out := RenderMeter("CPU", 100, 40, opts)
	if strings.Contains(out, "░") {
		t.Errorf("100%% should have no empty chars, got %q", out)
	}
}

func TestRenderMeter_ClampNegative(t *testing.T) {
	opts := MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()}
	out := RenderMeter("X", -10, 40, opts)
	if !strings.Contains(out, "0.0%") {
		t.Errorf("negative should clamp to 0, got %q", out)
	}
}

func TestRenderMeter_ClampOver100(t *testing.T) {
	opts := MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()}
	out := RenderMeter("X", 150, 40, opts)
	if !strings.Contains(out, "100.0%") {
		t.Errorf("over 100 should clamp, got %q", out)
	}
}

func TestRenderMiniMeter_Normal(t *testing.T) {
	opts := MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle()}
	out := RenderMiniMeter("cpu0", 75, 30, opts)
	if !strings.Contains(out, "cpu0") {
		t.Errorf("expected label, got %q", out)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("expected fill chars, got %q", out)
	}
	if !strings.Contains(out, "75.0%") {
		t.Errorf("expected percentage, got %q", out)
	}
	// Should NOT contain brackets.
	if strings.Contains(out, "[") || strings.Contains(out, "]") {
		t.Errorf("mini meter should not have brackets, got %q", out)
	}
}

func TestRenderMiniMeter_ZeroWidth(t *testing.T) {
	if out := RenderMiniMeter("X", 50, 0, MeterOpts{}); out != "" {
		t.Errorf("expected empty, got %q", out)
	}
}

func TestRenderMeter_ASCII(t *testing.T) {
	opts := MeterOpts{FillStyle: plainStyle(), EmptyStyle: plainStyle(), ASCII: true}
	out := RenderMeter("CPU", 50, 30, opts)
	if strings.Contains(out, "█") || strings.Contains(out, "░") {
		t.Fatalf("expected ascii glyphs, got %q", out)
	}
	if !strings.Contains(out, "=") || !strings.Contains(out, "-") {
		t.Fatalf("expected ascii bar glyphs, got %q", out)
	}
}
