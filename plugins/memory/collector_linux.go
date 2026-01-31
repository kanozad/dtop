//go:build linux

package memory

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mld.com/dtop/pkg/types"
)

// readMemoryStats collects memory, swap, disk, and I/O statistics on Linux.
func readMemoryStats(prev map[string]diskIOCounters, cfg Config) (types.MemoryStats, map[string]diskIOCounters, error) {
	stats := types.MemoryStats{
		Timestamp: time.Now(),
	}

	// Parse /proc/meminfo for RAM and swap stats
	meminfo, err := parseMeminfo()
	if err != nil {
		return stats, prev, fmt.Errorf("failed to parse /proc/meminfo: %w", err)
	}

	stats.RAMTotal = meminfo["MemTotal"]
	stats.RAMFree = meminfo["MemFree"]
	stats.RAMAvailable = meminfo["MemAvailable"]
	stats.RAMCached = meminfo["Cached"] + meminfo["SReclaimable"]
	// Calculate used: Total - Free - Buffers - Cached
	stats.RAMUsed = stats.RAMTotal - stats.RAMFree - meminfo["Buffers"] - stats.RAMCached

	stats.SwapTotal = meminfo["SwapTotal"]
	stats.SwapFree = meminfo["SwapFree"]
	stats.SwapUsed = stats.SwapTotal - stats.SwapFree

	// Check for ZFS ARC if enabled
	if cfg.ZFSARCCached {
		if arcSize, err := parseZFSARC(); err == nil {
			stats.ZFSARCSize = &arcSize
		}
		// Silently ignore ZFS ARC errors (not present on all systems)
	}

	// Collect disk information
	disks, err := collectDiskInfo(cfg.DisksFilter)
	if err != nil {
		return stats, prev, fmt.Errorf("failed to collect disk info: %w", err)
	}

	// Collect I/O statistics if enabled
	nextPrev := prev
	if cfg.ShowIOStat {
		diskIO, err := parseDiskstats()
		if err == nil {
			// Calculate deltas and rates
			for i := range disks {
				device := extractDeviceName(disks[i].Device)
				if io, ok := diskIO[device]; ok {
					if prevIO, hasPrev := prev[device]; hasPrev {
						elapsed := io.timestamp.Sub(prevIO.timestamp).Seconds()
						if elapsed > 0 {
							disks[i].ReadBytesPerSec = float64(io.readBytes-prevIO.readBytes) / elapsed
							disks[i].WriteBytesPerSec = float64(io.writeBytes-prevIO.writeBytes) / elapsed
						}
					}
				}
			}
			nextPrev = diskIO
		}
		// Silently ignore diskstats errors on first collection
	}

	stats.Disks = disks

	return stats, nextPrev, nil
}

// parseMeminfo parses /proc/meminfo and returns a map of key-value pairs (in bytes).
func parseMeminfo() (map[string]uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		valueKB, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		// Convert from KB to bytes
		result[key] = valueKB * 1024
	}
	return result, scanner.Err()
}

// parseZFSARC attempts to read ZFS ARC size from /proc/spl/kstat/zfs/arcstats.
func parseZFSARC() (uint64, error) {
	file, err := os.Open("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "size ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				size, err := strconv.ParseUint(parts[2], 10, 64)
				if err == nil {
					return size, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("ZFS ARC size not found")
}

// collectDiskInfo collects disk capacity information using syscall.Statfs.
func collectDiskInfo(filter []string) ([]types.DiskInfo, error) {
	// Parse /proc/mounts to get mount points
	mounts, err := parseMounts()
	if err != nil {
		return nil, err
	}

	// Apply filter if provided
	var filteredMounts []mountInfo
	if len(filter) > 0 {
		for _, m := range mounts {
			for _, f := range filter {
				if strings.Contains(m.mountPoint, f) {
					filteredMounts = append(filteredMounts, m)
					break
				}
			}
		}
	} else {
		// Default: filter out common virtual filesystems
		for _, m := range mounts {
			if isPhysicalFilesystem(m.fsType) {
				filteredMounts = append(filteredMounts, m)
			}
		}
	}

	var disks []types.DiskInfo
	for _, m := range filteredMounts {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(m.mountPoint, &stat); err != nil {
			continue // Skip mounts we can't stat
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free

		disks = append(disks, types.DiskInfo{
			MountPoint: m.mountPoint,
			Device:     m.device,
			Filesystem: m.fsType,
			Total:      total,
			Used:       used,
			Free:       free,
		})
	}

	return disks, nil
}

type mountInfo struct {
	device     string
	mountPoint string
	fsType     string
}

// parseMounts parses /proc/mounts to get mount point information.
func parseMounts() ([]mountInfo, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mounts []mountInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 3 {
			continue
		}
		mounts = append(mounts, mountInfo{
			device:     parts[0],
			mountPoint: parts[1],
			fsType:     parts[2],
		})
	}
	return mounts, scanner.Err()
}

// isPhysicalFilesystem returns true if the filesystem type is a physical disk.
func isPhysicalFilesystem(fsType string) bool {
	virtual := map[string]bool{
		"proc":       true,
		"sysfs":      true,
		"devpts":     true,
		"tmpfs":      true,
		"devtmpfs":   true,
		"cgroup":     true,
		"cgroup2":    true,
		"pstore":     true,
		"bpf":        true,
		"tracefs":    true,
		"debugfs":    true,
		"securityfs": true,
		"hugetlbfs":  true,
		"mqueue":     true,
		"autofs":     true,
		"autofs4":    true,
		"configfs":   true,
		"fusectl":    true,
		"fuse":       true,
		"fuse.gvfsd-fuse": true,
		"overlay":    true,
		"squashfs":   true,
	}
	return !virtual[fsType]
}

// parseDiskstats parses /proc/diskstats for I/O statistics.
func parseDiskstats() (map[string]diskIOCounters, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	now := time.Now()
	result := make(map[string]diskIOCounters)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		device := fields[2]
		
		// Read sectors (field 5, 0-indexed: field 6 in 1-indexed)
		readSectors, err1 := strconv.ParseUint(fields[5], 10, 64)
		// Write sectors (field 9, 0-indexed: field 10 in 1-indexed)
		writeSectors, err2 := strconv.ParseUint(fields[9], 10, 64)

		if err1 == nil && err2 == nil {
			// Convert sectors to bytes (sector size is typically 512 bytes)
			result[device] = diskIOCounters{
				readBytes:  readSectors * 512,
				writeBytes: writeSectors * 512,
				timestamp:  now,
			}
		}
	}

	return result, scanner.Err()
}

// extractDeviceName extracts the device name from a device path like /dev/sda1 -> sda1.
func extractDeviceName(device string) string {
	parts := strings.Split(device, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return device
}
