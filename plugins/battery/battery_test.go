package battery

import (
	"testing"
	"time"

	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/types"
)

func TestViewNoBattery(t *testing.T) {
	t.Parallel()
	b := New()
	th := theme.Default()
	stats := types.BatteryStats{Present: false}
	out := b.View(stats, 60, 10, th)
	if out == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestViewWithBattery(t *testing.T) {
	t.Parallel()
	b := New()
	th := theme.Default()
	pw := 15.5
	enow := 30.0
	efull := 50.0
	tte := 2*time.Hour + 30*time.Minute
	stats := types.BatteryStats{
		Present:       true,
		Capacity:      60,
		Status:        "Discharging",
		PowerNowWatts: &pw,
		EnergyNowWh:   &enow,
		EnergyFullWh:  &efull,
		TimeToEmpty:   &tte,
	}
	out := b.View(stats, 60, 10, th)
	if out == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    time.Duration
		want string
	}{
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{45 * time.Minute, "45m"},
		{0, "0m"},
	}
	for _, tc := range tests {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestCollectReturnsStats(t *testing.T) {
	t.Parallel()
	b := New()
	data, err := b.Collect(nil)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	stats, ok := data.(types.BatteryStats)
	if !ok {
		t.Fatalf("expected BatteryStats, got %T", data)
	}
	// On desktop machines, Present may be false — that's fine.
	if stats.Present && stats.Capacity < 0 {
		t.Fatalf("unexpected negative capacity: %f", stats.Capacity)
	}
}
