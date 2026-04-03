//go:build !linux

package battery

import "github.com/kanozad/dtop/pkg/types"

func readBatteryStats() (types.BatteryStats, error) {
	return types.BatteryStats{Present: false}, nil
}
