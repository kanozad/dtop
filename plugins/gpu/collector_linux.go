//go:build linux

package gpu

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mld.com/dtop/pkg/types"
)

// readGPUStats probes for GPU devices via NVML (for NVIDIA) and /sys/class/drm
// (for Intel/AMD).
func readGPUStats() (types.GPUStats, error) {
	var gpus []types.GPUInfo
	var hasTemp, hasPower bool

	// Try NVML first for NVIDIA GPUs.
	hasNvidia := false
	if nvGpus, ok := readNVMLStats(); ok {
		gpus = append(gpus, nvGpus...)
		if len(nvGpus) > 0 {
			hasNvidia = true
		}
		for _, g := range nvGpus {
			if g.TemperatureC >= 0 {
				hasTemp = true
			}
			if g.PowerWatts >= 0 {
				hasPower = true
			}
		}
	}

	// Try ROCm for AMD GPUs.
	hasAMD := false
	if rocmGpus, ok := readROCmStats(); ok {
		gpus = append(gpus, rocmGpus...)
		if len(rocmGpus) > 0 {
			hasAMD = true
		}
		for _, g := range rocmGpus {
			if g.TemperatureC >= 0 {
				hasTemp = true
			}
			if g.PowerWatts >= 0 {
				hasPower = true
			}
		}
	}

	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		if len(gpus) > 0 {
			return types.GPUStats{GPUs: gpus, HasTemp: hasTemp, HasPower: hasPower}, nil
		}
		return types.GPUStats{Error: "no DRM devices: " + err.Error()}, nil
	}

	for _, e := range entries {
		name := e.Name()
		// Only look at card devices (card0, card1, ...), not render nodes.
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}

		idx, err := strconv.Atoi(strings.TrimPrefix(name, "card"))
		if err != nil {
			continue
		}

		deviceDir := filepath.Join("/sys/class/drm", name, "device")

		// Skip if already handled by specialized loaders.
		vendor := readSysfsString(deviceDir, "vendor")
		if (hasNvidia && vendor == "0x10de") || (hasAMD && vendor == "0x1002") {
			continue
		}

		info := types.GPUInfo{
			Index:         idx,
			Name:          readSysfsString(deviceDir, "label"),
			TemperatureC:  -1,
			PowerWatts:    -1,
			PowerCapWatts: -1,
			PCIeTxMBps:    -1,
			PCIeRxMBps:    -1,
			EncoderPct:    -1,
			DecoderPct:    -1,
		}

		if info.Name == "" {
			info.Name = fmt.Sprintf("GPU %d", idx)
		}

		// GPU busy percent (Intel/AMD sysfs)
		if v, ok := readSysfsUint(deviceDir, "gpu_busy_percent"); ok {
			info.UtilizationPct = float64(v)
		}

		// Memory (VRAM) via drm mem_info sysfs (AMD/Intel)
		if total, ok := readSysfsUint(deviceDir, "mem_info_vram_total"); ok {
			info.MemoryTotal = total
			if used, ok := readSysfsUint(deviceDir, "mem_info_vram_used"); ok {
				info.MemoryUsed = used
				if total > 0 {
					info.MemoryPct = float64(used) * 100.0 / float64(total)
				}
			}
		}

		// HWMON temperature and power
		hwmonDir := findHwmonDir(deviceDir)
		if hwmonDir != "" {
			if temp, ok := readSysfsUint(hwmonDir, "temp1_input"); ok {
				info.TemperatureC = float64(temp) / 1000.0
				hasTemp = true
			}
			if pw, ok := readSysfsUint(hwmonDir, "power1_average"); ok {
				info.PowerWatts = float64(pw) / 1_000_000.0
				hasPower = true
			}
			if cap, ok := readSysfsUint(hwmonDir, "power1_cap"); ok {
				info.PowerCapWatts = float64(cap) / 1_000_000.0
			}
		}

		gpus = append(gpus, info)
	}

	if len(gpus) == 0 {
		return types.GPUStats{Error: "no GPU devices found in /sys/class/drm"}, nil
	}

	return types.GPUStats{
		GPUs:     gpus,
		HasTemp:  hasTemp,
		HasPower: hasPower,
	}, nil
}

func readSysfsString(dir, name string) string {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readSysfsUint(dir, name string) (uint64, bool) {
	s := readSysfsString(dir, name)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func findHwmonDir(deviceDir string) string {
	base := filepath.Join(deviceDir, "hwmon")
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "hwmon") {
			return filepath.Join(base, e.Name())
		}
	}
	return ""
}
