//go:build !linux

package cpu

import (
	"errors"

	"github.com/kanozad/dtop/pkg/types"
)

func readCPUStats(prev map[string]cpuTimes, _ collectOpts) (types.CPUStats, map[string]cpuTimes, raplState, error) {
	return types.CPUStats{}, prev, raplState{}, errors.New("cpu collector not supported on this platform")
}
