package memory

import "time"

// diskIOCounters holds raw I/O counters for a disk device.
type diskIOCounters struct {
	readBytes  uint64
	writeBytes uint64
	timestamp  time.Time
}
