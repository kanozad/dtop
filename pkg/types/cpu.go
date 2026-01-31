package types

import "time"

// CPUStats captures CPU utilization and load averages.
type CPUStats struct {
	Cores        int
	PerCore      []float64
	Total        float64
	Load1        float64
	Load5        float64
	Load15       float64
	TemperatureC *float64
	Timestamp    time.Time

	// History buffers sized to the current viewport width (trimmed/padded on resize).
	TotalHistory   []float64
	PerCoreHistory [][]float64
}
