//go:build !linux

package battery

import "mld.com/dtop/pkg/types"

func readBatteryStats() (types.BatteryStats, error) {
	return types.BatteryStats{Present: false}, nil
}
