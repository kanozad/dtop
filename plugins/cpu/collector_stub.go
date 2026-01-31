//go:build !linux

package cpu

import (
	"errors"

	"mld.com/dtop/pkg/types"
)

func readCPUStats(prev map[string]cpuTimes, _ bool) (types.CPUStats, map[string]cpuTimes, error) {
	return types.CPUStats{}, prev, errors.New("cpu collector not supported on this platform")
}
