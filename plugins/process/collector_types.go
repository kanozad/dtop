package process

import "time"

// procStat holds raw /proc/[pid]/stat data for calculating CPU percentages.
type procStat struct {
	utime     uint64    // CPU time spent in user mode
	stime     uint64    // CPU time spent in kernel mode
	timestamp time.Time // When this snapshot was taken
}

// systemStat holds overall system CPU time for calculating CPU percentages.
type systemStat struct {
	totalTime  uint64 // Total CPU time across all CPUs
	timestamp  time.Time
	bootTime   time.Time
	clockTicks int64
}
