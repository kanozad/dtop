//go:build linux

package battery

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kanozad/dtop/pkg/types"
)

func readBatteryStats() (types.BatteryStats, error) {
	entries, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err != nil || len(entries) == 0 {
		return types.BatteryStats{Present: false}, nil
	}

	// Use the first battery found.
	dir := entries[0]

	stats := types.BatteryStats{Present: true}

	// Capacity (0-100).
	if v, ok := readSysfsInt(dir, "capacity"); ok {
		stats.Capacity = float64(v)
	}

	// Status: Charging, Discharging, Full, Not charging, Unknown.
	if s := readSysfsString(dir, "status"); s != "" {
		stats.Status = s
	} else {
		stats.Status = "Unknown"
	}

	// Power draw (microwatts → watts).
	if pw, ok := readSysfsInt(dir, "power_now"); ok && pw > 0 {
		watts := float64(pw) / 1_000_000.0
		stats.PowerNowWatts = &watts
	}

	// Energy now and full (microwatt-hours → watt-hours).
	var energyNow, energyFull float64
	if en, ok := readSysfsInt(dir, "energy_now"); ok {
		wh := float64(en) / 1_000_000.0
		stats.EnergyNowWh = &wh
		energyNow = wh
	}
	if ef, ok := readSysfsInt(dir, "energy_full"); ok {
		wh := float64(ef) / 1_000_000.0
		stats.EnergyFullWh = &wh
		energyFull = wh
	}

	// Time estimates based on power draw and energy.
	if stats.PowerNowWatts != nil && *stats.PowerNowWatts > 0 {
		pw := *stats.PowerNowWatts
		switch stats.Status {
		case "Discharging":
			if energyNow > 0 {
				d := time.Duration(energyNow / pw * float64(time.Hour))
				stats.TimeToEmpty = &d
			}
		case "Charging":
			if energyFull > energyNow {
				d := time.Duration((energyFull - energyNow) / pw * float64(time.Hour))
				stats.TimeToFull = &d
			}
		}
	}

	return stats, nil
}

func readSysfsString(dir, name string) string {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readSysfsInt(dir, name string) (int64, bool) {
	s := readSysfsString(dir, name)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
