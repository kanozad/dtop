//go:build !linux

package memory

import (
	"fmt"

	"github.com/kanozad/dtop/pkg/types"
)

// readMemoryStats returns an error on non-Linux platforms.
func readMemoryStats(prev map[string]diskIOCounters, cfg Config) (types.MemoryStats, map[string]diskIOCounters, error) {
	return types.MemoryStats{}, prev, fmt.Errorf("memory collection not implemented for this platform")
}
