package cpu

import "time"

type cpuTimes struct {
	total uint64
	idle  uint64
}

// collectOpts bundles feature flags and prior RAPL state for readCPUStats.
type collectOpts struct {
	showTemp       bool
	showFreq       bool
	showWatts      bool
	prevRAPLEnergy uint64
	prevRAPLTime   time.Time
}

// raplState captures the current RAPL energy reading so the caller can
// persist it across collections for delta calculation.
type raplState struct {
	energy    uint64
	timestamp time.Time
}
