//go:build linux

package gpu

import (
	"errors"
	"fmt"

	"github.com/ebitengine/purego"
	"mld.com/dtop/pkg/types"
)

// RSMI Constants
const (
	rsmiStatusSuccess = 0
)

// RSMI Types
type rsmiStatus int

// Function pointers
var (
	rsmi_init                 func(uint64) rsmiStatus
	rsmi_shut_down            func() rsmiStatus
	rsmi_num_monitor_devices  func(count *uint32) rsmiStatus
	rsmi_dev_name_get         func(dv_ind uint32, name *byte, len uint32) rsmiStatus
	rsmi_dev_busy_percent_get func(dv_ind uint32, busy *uint32) rsmiStatus
	rsmi_dev_memory_usage_get func(dv_ind uint32, mem_type int, used *uint64) rsmiStatus
	rsmi_dev_memory_total_get func(dv_ind uint32, mem_type int, total *uint64) rsmiStatus
	rsmi_dev_temp_metric_get  func(dv_ind uint32, sensor int, metric int, temp *int64) rsmiStatus
	rsmi_dev_power_ave_get    func(dv_ind uint32, sensor uint32, power *uint64) rsmiStatus
	rsmi_dev_power_cap_get    func(dv_ind uint32, sensor uint32, cap *uint64) rsmiStatus
)

var rocmLib uintptr
var rocmInitialized bool

func initROCm() error {
	if rocmInitialized {
		return nil
	}

	libs := []string{
		"librocm_smi64.so",
		"librocm_smi64.so.1",
		"/opt/rocm/lib/librocm_smi64.so",
	}
	var err error
	for _, lib := range libs {
		rocmLib, err = purego.Dlopen(lib, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("could not load ROCm SMI library: %w", err)
	}

	purego.RegisterLibFunc(&rsmi_init, rocmLib, "rsmi_init")
	purego.RegisterLibFunc(&rsmi_shut_down, rocmLib, "rsmi_shut_down")
	purego.RegisterLibFunc(&rsmi_num_monitor_devices, rocmLib, "rsmi_num_monitor_devices")
	purego.RegisterLibFunc(&rsmi_dev_name_get, rocmLib, "rsmi_dev_name_get")
	purego.RegisterLibFunc(&rsmi_dev_busy_percent_get, rocmLib, "rsmi_dev_busy_percent_get")
	purego.RegisterLibFunc(&rsmi_dev_memory_usage_get, rocmLib, "rsmi_dev_memory_usage_get")
	purego.RegisterLibFunc(&rsmi_dev_memory_total_get, rocmLib, "rsmi_dev_memory_total_get")
	purego.RegisterLibFunc(&rsmi_dev_temp_metric_get, rocmLib, "rsmi_dev_temp_metric_get")
	purego.RegisterLibFunc(&rsmi_dev_power_ave_get, rocmLib, "rsmi_dev_power_ave_get")
	purego.RegisterLibFunc(&rsmi_dev_power_cap_get, rocmLib, "rsmi_dev_power_cap_get")

	if rsmi_init(0) != rsmiStatusSuccess {
		return errors.New("rsmi_init failed")
	}

	rocmInitialized = true
	return nil
}

func shutdownROCm() {
	if rocmInitialized && rsmi_shut_down != nil {
		rsmi_shut_down()
		rocmInitialized = false
	}
}

func readROCmStats() ([]types.GPUInfo, bool) {
	if err := initROCm(); err != nil {
		return nil, false
	}

	var count uint32
	if rsmi_num_monitor_devices(&count) != rsmiStatusSuccess || count == 0 {
		return nil, false
	}

	var gpus []types.GPUInfo
	for i := uint32(0); i < count; i++ {
		info := types.GPUInfo{
			Index:         int(i),
			TemperatureC:  -1,
			PowerWatts:    -1,
			PowerCapWatts: -1,
		}

		var buf [128]byte
		if rsmi_dev_name_get(i, &buf[0], 128) == rsmiStatusSuccess {
			length := 0
			for length < 128 && buf[length] != 0 {
				length++
			}
			info.Name = string(buf[:length])
		}

		var busy uint32
		if rsmi_dev_busy_percent_get(i, &busy) == rsmiStatusSuccess {
			info.UtilizationPct = float64(busy)
		}

		var used uint64
		if rsmi_dev_memory_usage_get(i, 0, &used) == rsmiStatusSuccess { // 0 = RSMI_MEM_TYPE_VRAM
			info.MemoryUsed = used
			var total uint64
			if rsmi_dev_memory_total_get(i, 0, &total) == rsmiStatusSuccess {
				info.MemoryTotal = total
				if total > 0 {
					info.MemoryPct = float64(used) * 100.0 / float64(total)
				}
			}
		}

		var temp int64
		if rsmi_dev_temp_metric_get(i, 0, 0, &temp) == rsmiStatusSuccess { // 0, 0 = RSMI_TEMP_TYPE_EDGE, RSMI_TEMP_CURRENT
			info.TemperatureC = float64(temp) / 1000.0
		}

		var power uint64
		if rsmi_dev_power_ave_get(i, 0, &power) == rsmiStatusSuccess {
			info.PowerWatts = float64(power) / 1_000_000.0 // uW to W
		}

		var cap uint64
		if rsmi_dev_power_cap_get(i, 0, &cap) == rsmiStatusSuccess {
			info.PowerCapWatts = float64(cap) / 1_000_000.0 // uW to W
		}

		gpus = append(gpus, info)
	}

	return gpus, true
}
