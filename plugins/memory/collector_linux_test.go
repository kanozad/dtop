//go:build linux

package memory

import (
	"strings"
	"testing"
)

func TestParseMeminfo(t *testing.T) {
	// This test will use the real /proc/meminfo, so we just verify it doesn't error
	// and returns reasonable values
	meminfo, err := parseMeminfo()
	if err != nil {
		t.Fatalf("parseMeminfo failed: %v", err)
	}

	// Check that essential fields are present
	if meminfo["MemTotal"] == 0 {
		t.Error("MemTotal should not be zero")
	}
	if meminfo["MemFree"] > meminfo["MemTotal"] {
		t.Error("MemFree should not exceed MemTotal")
	}
}

func TestParseMounts(t *testing.T) {
	mounts, err := parseMounts()
	if err != nil {
		t.Fatalf("parseMounts failed: %v", err)
	}

	if len(mounts) == 0 {
		t.Error("Expected at least one mount point")
	}

	// Verify structure
	for _, m := range mounts {
		if m.device == "" || m.mountPoint == "" || m.fsType == "" {
			t.Errorf("Invalid mount entry: %+v", m)
		}
	}
}

func TestIsPhysicalFilesystem(t *testing.T) {
	tests := []struct {
		fsType   string
		expected bool
	}{
		{"ext4", true},
		{"xfs", true},
		{"btrfs", true},
		{"proc", false},
		{"sysfs", false},
		{"tmpfs", false},
		{"devpts", false},
	}

	for _, tt := range tests {
		result := isPhysicalFilesystem(tt.fsType)
		if result != tt.expected {
			t.Errorf("isPhysicalFilesystem(%q) = %v, want %v", tt.fsType, result, tt.expected)
		}
	}
}

func TestExtractDeviceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/dev/sda1", "sda1"},
		{"/dev/nvme0n1p1", "nvme0n1p1"},
		{"sdb", "sdb"},
		{"/dev/mapper/vg-lv", "vg-lv"},
	}

	for _, tt := range tests {
		result := extractDeviceName(tt.input)
		if result != tt.expected {
			t.Errorf("extractDeviceName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseDiskstats(t *testing.T) {
	// This test uses the real /proc/diskstats
	diskIO, err := parseDiskstats()
	if err != nil {
		t.Fatalf("parseDiskstats failed: %v", err)
	}

	// Should have at least one device
	if len(diskIO) == 0 {
		t.Error("Expected at least one disk device")
	}

	// Verify structure
	for device, io := range diskIO {
		if device == "" {
			t.Error("Device name should not be empty")
		}
		if io.timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
		// Read/write bytes can be zero for some devices, so we don't check those
	}
}

func TestCollectDiskInfo(t *testing.T) {
	// Test with no filter (default physical filesystems only)
	disks, err := collectDiskInfo(nil)
	if err != nil {
		t.Fatalf("collectDiskInfo failed: %v", err)
	}

	// Should have at least one physical disk
	if len(disks) == 0 {
		t.Skip("No physical disks found (possibly running in a container)")
	}

	// Verify structure
	for _, disk := range disks {
		if disk.MountPoint == "" {
			t.Error("Mount point should not be empty")
		}
		// Some special filesystems might have zero total, skip the check
		if disk.Total > 0 && disk.Used > disk.Total {
			t.Errorf("Disk %s used (%d) should not exceed total (%d)", disk.MountPoint, disk.Used, disk.Total)
		}
	}
}

func TestCollectDiskInfoWithFilter(t *testing.T) {
	// Test with filter for common mount points
	filter := []string{"/", "/home"}
	disks, err := collectDiskInfo(filter)
	if err != nil {
		t.Fatalf("collectDiskInfo failed: %v", err)
	}

	// Verify all returned disks match the filter
	for _, disk := range disks {
		matched := false
		for _, f := range filter {
			if strings.Contains(disk.MountPoint, f) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("Disk %s does not match filter %v", disk.MountPoint, filter)
		}
	}
}

func TestReadMemoryStats(t *testing.T) {
	cfg := Config{
		ShowSwap:     true,
		ShowDisks:    true,
		ShowIOStat:   true,
		Base10Sizes:  false,
		ZFSARCCached: false,
	}

	prev := make(map[string]diskIOCounters)
	stats, nextPrev, err := readMemoryStats(prev, cfg)
	if err != nil {
		t.Fatalf("readMemoryStats failed: %v", err)
	}

	// Verify memory stats
	if stats.RAMTotal == 0 {
		t.Error("RAMTotal should not be zero")
	}
	if stats.RAMUsed > stats.RAMTotal {
		t.Error("RAMUsed should not exceed RAMTotal")
	}

	// Verify timestamp
	if stats.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}

	// Verify nextPrev is returned
	if cfg.ShowIOStat && nextPrev == nil {
		t.Error("nextPrev should be populated when ShowIOStat is enabled")
	}
}
