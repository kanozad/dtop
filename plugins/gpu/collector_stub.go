//go:build !linux

package gpu

import "github.com/kanozad/dtop/pkg/types"

func readGPUStats() (types.GPUStats, error) {
	return types.GPUStats{Error: "GPU monitoring not supported on this platform"}, nil
}

func shutdownNVML() {}
func shutdownROCm() {}
