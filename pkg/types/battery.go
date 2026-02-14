package types

import "time"

// BatteryStats captures battery charge level, status, and power draw.
type BatteryStats struct {
	Present       bool
	Capacity      float64 // 0-100 percent
	Status        string  // Charging, Discharging, Full, Not charging, Unknown
	TimeToEmpty   *time.Duration
	TimeToFull    *time.Duration
	PowerNowWatts *float64
	EnergyNowWh  *float64
	EnergyFullWh *float64
	Error         string
}
