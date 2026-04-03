//go:build linux

package gpu

import (
	"errors"
	"fmt"

	"github.com/ebitengine/purego"
	"github.com/kanozad/dtop/pkg/types"
)

// NVML Constants
const (
	nvmlSuccess = 0
)

// NVML Types
type nvmlDevice struct{}
type nvmlReturn int

type nvmlUtilization struct {
	GPU    uint32
	Memory uint32
}

type nvmlMemory struct {
	Total uint64
	Free  uint64
	Used  uint64
}

// Function pointers
var (
	nvmlInit_v2                     func() nvmlReturn
	nvmlShutdown                    func() nvmlReturn
	nvmlDeviceGetCount_v2           func(count *uint32) nvmlReturn
	nvmlDeviceGetHandleByIndex_v2   func(index uint32, device **nvmlDevice) nvmlReturn
	nvmlDeviceGetName               func(device *nvmlDevice, name *byte, length uint32) nvmlReturn
	nvmlDeviceGetUtilizationRates   func(device *nvmlDevice, utilization *nvmlUtilization) nvmlReturn
	nvmlDeviceGetMemoryInfo         func(device *nvmlDevice, memory *nvmlMemory) nvmlReturn
	nvmlDeviceGetTemperature        func(device *nvmlDevice, sensorType int, temp *uint32) nvmlReturn
	nvmlDeviceGetPowerUsage         func(device *nvmlDevice, power *uint32) nvmlReturn
	nvmlDeviceGetEnforcedPowerLimit func(device *nvmlDevice, limit *uint32) nvmlReturn
)

var nvmlLib uintptr
var nvmlInitialized bool

func initNVML() error {
	if nvmlInitialized {
		return nil
	}

	// Try common NVML library names
	libs := []string{"libnvidia-ml.so", "libnvidia-ml.so.1"}
	var err error
	for _, lib := range libs {
		nvmlLib, err = purego.Dlopen(lib, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("could not load NVML library: %w", err)
	}

	purego.RegisterLibFunc(&nvmlInit_v2, nvmlLib, "nvmlInit_v2")
	purego.RegisterLibFunc(&nvmlShutdown, nvmlLib, "nvmlShutdown")
	purego.RegisterLibFunc(&nvmlDeviceGetCount_v2, nvmlLib, "nvmlDeviceGetCount_v2")
	purego.RegisterLibFunc(&nvmlDeviceGetHandleByIndex_v2, nvmlLib, "nvmlDeviceGetHandleByIndex_v2")
	purego.RegisterLibFunc(&nvmlDeviceGetName, nvmlLib, "nvmlDeviceGetName")
	purego.RegisterLibFunc(&nvmlDeviceGetUtilizationRates, nvmlLib, "nvmlDeviceGetUtilizationRates")
	purego.RegisterLibFunc(&nvmlDeviceGetMemoryInfo, nvmlLib, "nvmlDeviceGetMemoryInfo")
	purego.RegisterLibFunc(&nvmlDeviceGetTemperature, nvmlLib, "nvmlDeviceGetTemperature")
	purego.RegisterLibFunc(&nvmlDeviceGetPowerUsage, nvmlLib, "nvmlDeviceGetPowerUsage")
	purego.RegisterLibFunc(&nvmlDeviceGetEnforcedPowerLimit, nvmlLib, "nvmlDeviceGetEnforcedPowerLimit")

	if nvmlInit_v2() != nvmlSuccess {
		return errors.New("nvmlInit failed")
	}

	nvmlInitialized = true
	return nil
}

func shutdownNVML() {
	if nvmlInitialized && nvmlShutdown != nil {
		nvmlShutdown()
		nvmlInitialized = false
	}
}

func getNvmlString(device *nvmlDevice, fn func(*nvmlDevice, *byte, uint32) nvmlReturn) string {
	var buf [128]byte
	if fn(device, &buf[0], 128) == nvmlSuccess {
		length := 0
		for length < 128 && buf[length] != 0 {
			length++
		}
		return string(buf[:length])
	}
	return ""
}

func readNVMLStats() ([]types.GPUInfo, bool) {
	if err := initNVML(); err != nil {
		return nil, false
	}

	var count uint32
	if nvmlDeviceGetCount_v2(&count) != nvmlSuccess || count == 0 {
		return nil, false
	}

	var gpus []types.GPUInfo
	for i := uint32(0); i < count; i++ {
		var device *nvmlDevice
		if nvmlDeviceGetHandleByIndex_v2(i, &device) != nvmlSuccess {
			continue
		}

		info := types.GPUInfo{
			Index:         int(i),
			Name:          getNvmlString(device, nvmlDeviceGetName),
			TemperatureC:  -1,
			PowerWatts:    -1,
			PowerCapWatts: -1,
		}

		var util nvmlUtilization
		if nvmlDeviceGetUtilizationRates(device, &util) == nvmlSuccess {
			info.UtilizationPct = float64(util.GPU)
		}

		var mem nvmlMemory
		if nvmlDeviceGetMemoryInfo(device, &mem) == nvmlSuccess {
			info.MemoryTotal = mem.Total
			info.MemoryUsed = mem.Used
			if mem.Total > 0 {
				info.MemoryPct = float64(mem.Used) * 100.0 / float64(mem.Total)
			}
		}

		var temp uint32
		if nvmlDeviceGetTemperature(device, 0, &temp) == nvmlSuccess { // 0 = NVML_TEMPERATURE_GPU
			info.TemperatureC = float64(temp)
		}

		var power uint32
		if nvmlDeviceGetPowerUsage(device, &power) == nvmlSuccess {
			info.PowerWatts = float64(power) / 1000.0 // mW to W
		}

		var limit uint32
		if nvmlDeviceGetEnforcedPowerLimit(device, &limit) == nvmlSuccess {
			info.PowerCapWatts = float64(limit) / 1000.0 // mW to W
		}

		gpus = append(gpus, info)
	}

	return gpus, true
}
