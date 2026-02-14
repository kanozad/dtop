//go:build linux

package cpu

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func TestParseCPUTimes(t *testing.T) {
	t.Parallel()

	total, idle, err := parseCPUTimes([]string{"100", "200", "300", "400", "50", "6"})
	if err != nil {
		t.Fatalf("parseCPUTimes: %v", err)
	}
	if total != 1056 {
		t.Fatalf("total: got %d", total)
	}
	if idle != 450 {
		t.Fatalf("idle: got %d", idle)
	}
}

func TestReadCPUSamples(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeTempFile(t, dir, "stat", `cpu  100 200 300 400 50 0 0 0 0 0
cpu0 10 20 30 40 5 0 0 0 0 0
cpu1 20 30 40 50 5 0 0 0 0 0
intr 123
`)

	samples, err := readCPUSamples(path)
	if err != nil {
		t.Fatalf("readCPUSamples: %v", err)
	}
	if _, ok := samples["cpu"]; !ok {
		t.Fatalf("missing total cpu sample")
	}
	if _, ok := samples["cpu0"]; !ok {
		t.Fatalf("missing cpu0 sample")
	}
	if _, ok := samples["cpu1"]; !ok {
		t.Fatalf("missing cpu1 sample")
	}
}

func TestUsagePercent(t *testing.T) {
	t.Parallel()

	prev := map[string]cpuTimes{
		"cpu0": {total: 100, idle: 50},
	}
	cur := cpuTimes{total: 200, idle: 60}
	got := usagePercent(prev, "cpu0", cur)
	if got < 89.9 || got > 90.1 {
		t.Fatalf("usage: got %.2f", got)
	}
}

func TestReadLoadAvg(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeTempFile(t, dir, "loadavg", "0.50 1.00 1.50 1/100 123\n")

	load1, load5, load15, ok := readLoadAvg(path)
	if !ok {
		t.Fatalf("expected loadavg to parse")
	}
	if load1 != 0.5 || load5 != 1.0 || load15 != 1.5 {
		t.Fatalf("unexpected load values: %v %v %v", load1, load5, load15)
	}
}

func TestReadCoreFrequencies(t *testing.T) {
	t.Parallel()

	// readCoreFrequencies reads from /sys which may or may not exist.
	// We test that the function returns a valid slice on this machine
	// (could be nil on VMs without cpufreq).
	freqs := readCoreFrequencies(2)
	if freqs != nil {
		if len(freqs) != 2 {
			t.Fatalf("expected 2 frequencies, got %d", len(freqs))
		}
		for i, f := range freqs {
			if f <= 0 || f > 10000 {
				t.Fatalf("core %d: frequency %f out of plausible range", i, f)
			}
		}
	}
}

func TestReadCoreFrequenciesZeroCores(t *testing.T) {
	t.Parallel()
	if freqs := readCoreFrequencies(0); freqs != nil {
		t.Fatalf("expected nil for 0 cores, got %v", freqs)
	}
}

func TestReadCPUStatsWithFreqAndWatts(t *testing.T) {
	t.Parallel()

	// First collect to establish baselines.
	opts := collectOpts{showTemp: false, showFreq: true, showWatts: true}
	stats, prev, rs, err := readCPUStats(nil, opts)
	if err != nil {
		t.Fatalf("first collect: %v", err)
	}
	if stats.Cores == 0 {
		t.Fatal("expected at least 1 core")
	}

	// Frequency may or may not be available depending on environment.
	if stats.FrequencyMHz != nil {
		if len(stats.FrequencyMHz) != stats.Cores {
			t.Fatalf("frequency slice length %d != cores %d", len(stats.FrequencyMHz), stats.Cores)
		}
		if stats.FrequencyAvgMHz <= 0 {
			t.Fatalf("expected positive avg frequency, got %f", stats.FrequencyAvgMHz)
		}
	}

	// PowerWatts should be nil on first collect (no previous RAPL baseline).
	if stats.PowerWatts != nil {
		t.Fatalf("expected nil PowerWatts on first collect")
	}

	// Second collect with prior RAPL state.
	time.Sleep(50 * time.Millisecond)
	opts.prevRAPLEnergy = rs.energy
	opts.prevRAPLTime = rs.timestamp
	stats2, _, _, err := readCPUStats(prev, opts)
	if err != nil {
		t.Fatalf("second collect: %v", err)
	}

	// If RAPL is available, watts should now be set.
	if rs.energy > 0 && stats2.PowerWatts != nil {
		if *stats2.PowerWatts < 0 || math.IsNaN(*stats2.PowerWatts) {
			t.Fatalf("implausible watts: %f", *stats2.PowerWatts)
		}
	}
}
