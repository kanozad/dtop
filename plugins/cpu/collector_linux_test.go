//go:build linux

package cpu

import (
	"os"
	"path/filepath"
	"testing"
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
