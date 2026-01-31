//go:build !linux

package process

import (
	"fmt"

	"mld.com/dtop/pkg/types"
)

// readProcessStats is a stub for non-Linux platforms.
func readProcessStats(prevProc map[int]procStat, prevSys systemStat, cfg Config) (types.ProcessStats, map[int]procStat, systemStat, error) {
	return types.ProcessStats{}, prevProc, prevSys, fmt.Errorf("process monitoring not supported on this platform")
}
