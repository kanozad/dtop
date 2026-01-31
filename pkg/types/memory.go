package types

import "time"

// MemoryStats captures RAM, swap, and optional ZFS ARC statistics.
type MemoryStats struct {
	// RAM stats (bytes)
	RAMTotal     uint64
	RAMUsed      uint64
	RAMAvailable uint64
	RAMCached    uint64
	RAMFree      uint64

	// Swap stats (bytes)
	SwapTotal uint64
	SwapUsed  uint64
	SwapFree  uint64

	// ZFS ARC stats (bytes, optional)
	ZFSARCSize *uint64
	ZFSARCMax  *uint64

	// Disk information
	Disks []DiskInfo

	Timestamp time.Time

	// History buffers sized to the current viewport width (trimmed/padded on resize).
	MemoryHistory []float64 // Memory usage percentage history
	SwapHistory   []float64 // Swap usage percentage history
}

// DiskInfo represents information about a single disk/mount point.
type DiskInfo struct {
	MountPoint string
	Device     string
	Filesystem string

	// Capacity (bytes)
	Total uint64
	Used  uint64
	Free  uint64

	// I/O stats (bytes per second since last collection)
	ReadBytesPerSec  float64
	WriteBytesPerSec float64

	// History buffers for I/O rates
	ReadHistory  []float64
	WriteHistory []float64
}
