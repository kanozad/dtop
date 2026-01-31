//go:build !linux

package network

import (
	"fmt"

	"mld.com/dtop/pkg/types"
)

// readNetworkStats returns an error on non-Linux platforms.
func readNetworkStats(prev map[string]netDevCounters, cfg Config) (types.NetworkStats, map[string]netDevCounters, error) {
	return types.NetworkStats{}, prev, fmt.Errorf("network collection not implemented for this platform")
}
