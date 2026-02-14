//go:build linux

package cpu

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"mld.com/dtop/pkg/types"
)

func readCPUStats(prev map[string]cpuTimes, opts collectOpts) (types.CPUStats, map[string]cpuTimes, raplState, error) {
	samples, err := readCPUSamples("/proc/stat")
	if err != nil {
		return types.CPUStats{}, prev, raplState{}, err
	}

	now := time.Now()
	stats := types.CPUStats{Timestamp: now}
	if total, ok := samples["cpu"]; ok {
		stats.Total = usagePercent(prev, "cpu", total)
	}

	stats.PerCore = perCorePercents(prev, samples)
	stats.Cores = len(stats.PerCore)

	if load1, load5, load15, ok := readLoadAvg("/proc/loadavg"); ok {
		stats.Load1 = load1
		stats.Load5 = load5
		stats.Load15 = load15
	}

	if opts.showTemp {
		stats.TemperatureC = readTemperature("/sys/class/thermal")
	}

	if opts.showFreq {
		stats.FrequencyMHz = readCoreFrequencies(stats.Cores)
		if len(stats.FrequencyMHz) > 0 {
			var sum float64
			for _, f := range stats.FrequencyMHz {
				sum += f
			}
			stats.FrequencyAvgMHz = sum / float64(len(stats.FrequencyMHz))
		}
	}

	var rs raplState
	if opts.showWatts {
		rs = readRAPLEnergy()
		if rs.energy > 0 && opts.prevRAPLEnergy > 0 && !opts.prevRAPLTime.IsZero() {
			deltaEnergy := rs.energy - opts.prevRAPLEnergy
			// Handle counter wrap-around (32-bit or implementation-specific).
			if rs.energy < opts.prevRAPLEnergy {
				deltaEnergy = rs.energy // skip this sample on wrap
			} else {
				deltaTime := now.Sub(opts.prevRAPLTime).Seconds()
				if deltaTime > 0 {
					watts := float64(deltaEnergy) / 1_000_000.0 / deltaTime
					stats.PowerWatts = &watts
				}
			}
		}
	}

	stats.ContainerType, stats.EffectiveCPUs = detectContainer()

	return stats, samples, rs, nil
}

func usagePercent(prev map[string]cpuTimes, key string, cur cpuTimes) float64 {
	if prev == nil {
		return 0
	}
	before, ok := prev[key]
	if !ok {
		return 0
	}
	totalDelta := cur.total - before.total
	idleDelta := cur.idle - before.idle
	if totalDelta == 0 {
		return 0
	}
	usage := 100 * (1 - float64(idleDelta)/float64(totalDelta))
	if usage < 0 {
		return 0
	}
	if usage > 100 {
		return 100
	}
	return usage
}

func perCorePercents(prev map[string]cpuTimes, samples map[string]cpuTimes) []float64 {
	type coreSample struct {
		idx    int
		sample cpuTimes
	}
	cores := make([]coreSample, 0, len(samples))
	for name, sample := range samples {
		if name == "cpu" || !strings.HasPrefix(name, "cpu") {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimPrefix(name, "cpu"))
		if err != nil {
			continue
		}
		cores = append(cores, coreSample{idx: idx, sample: sample})
	}
	sort.Slice(cores, func(i, j int) bool { return cores[i].idx < cores[j].idx })
	out := make([]float64, 0, len(cores))
	for _, core := range cores {
		key := "cpu" + strconv.Itoa(core.idx)
		out = append(out, usagePercent(prev, key, core.sample))
	}
	return out
}

func readCPUSamples(path string) (map[string]cpuTimes, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	samples := map[string]cpuTimes{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		name := fields[0]
		total, idle, err := parseCPUTimes(fields[1:])
		if err != nil {
			return nil, err
		}
		samples[name] = cpuTimes{total: total, idle: idle}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, errors.New("no cpu samples found")
	}
	return samples, nil
}

func parseCPUTimes(fields []string) (uint64, uint64, error) {
	var total uint64
	var idle uint64
	for i, field := range fields {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, err
		}
		total += value
		if i == 3 { // idle
			idle = value
		}
		if i == 4 { // iowait
			idle += value
		}
	}
	return total, idle, nil
}

func readLoadAvg(path string) (float64, float64, float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0, false
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return 0, 0, 0, false
	}
	load1, err1 := strconv.ParseFloat(fields[0], 64)
	load5, err5 := strconv.ParseFloat(fields[1], 64)
	load15, err15 := strconv.ParseFloat(fields[2], 64)
	if err1 != nil || err5 != nil || err15 != nil {
		return 0, 0, 0, false
	}
	return load1, load5, load15, true
}

func readTemperature(base string) *float64 {
	paths, err := filepath.Glob(filepath.Join(base, "thermal_zone*", "temp"))
	if err != nil {
		return nil
	}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			continue
		}
		if value > 1000 {
			value = value / 1000
		}
		return &value
	}
	return nil
}

// readCoreFrequencies reads per-core current frequency from cpufreq sysfs.
// Returns MHz for each core, or nil if cpufreq is unavailable.
func readCoreFrequencies(coreCount int) []float64 {
	if coreCount <= 0 {
		return nil
	}
	freqs := make([]float64, 0, coreCount)
	for i := 0; i < coreCount; i++ {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil // cpufreq not available
		}
		khz, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return nil
		}
		freqs = append(freqs, khz/1000.0)
	}
	return freqs
}

// readRAPLEnergy reads the package-level RAPL energy counter (microjoules).
// Returns zero-value raplState if RAPL is not available.
func readRAPLEnergy() raplState {
	// Try Intel RAPL first, then AMD equivalent.
	for _, path := range []string{
		"/sys/class/powercap/intel-rapl:0/energy_uj",
		"/sys/class/powercap/amd-rapl:0/energy_uj",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			continue
		}
		return raplState{energy: v, timestamp: time.Now()}
	}
	return raplState{}
}

// detectContainer checks whether we're running inside a container and, if so,
// reads the cgroup CPU quota to determine effective CPUs.
func detectContainer() (string, int) {
	ctype := detectContainerType()
	if ctype == "" {
		return "", 0
	}
	effective := readCgroupCPUQuota()
	return ctype, effective
}

func detectContainerType() string {
	// Check /.dockerenv (Docker creates this file).
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}

	// Check /proc/1/cgroup for container runtime signatures.
	b, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return ""
	}
	content := strings.ToLower(string(b))
	if strings.Contains(content, "docker") || strings.Contains(content, "containerd") {
		return "docker"
	}
	if strings.Contains(content, "lxc") {
		return "lxc"
	}

	// Check /proc/self/mountinfo for overlay filesystem (common in containers).
	mi, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	miContent := strings.ToLower(string(mi))
	if strings.Contains(miContent, "overlay") && strings.Contains(miContent, "/docker/") {
		return "docker"
	}

	return ""
}

// readCgroupCPUQuota reads the effective CPU count from cgroup v2 cpu.max or
// cgroup v1 cpu.cfs_quota_us/cpu.cfs_period_us. Returns 0 if no quota is set.
func readCgroupCPUQuota() int {
	// cgroup v2: /sys/fs/cgroup/cpu.max contains "<quota> <period>" or "max <period>".
	if b, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		fields := strings.Fields(strings.TrimSpace(string(b)))
		if len(fields) == 2 && fields[0] != "max" {
			quota, err1 := strconv.ParseFloat(fields[0], 64)
			period, err2 := strconv.ParseFloat(fields[1], 64)
			if err1 == nil && err2 == nil && period > 0 {
				cpus := int(quota / period)
				if cpus > 0 {
					return cpus
				}
			}
		}
	}

	// cgroup v1: separate files for quota and period.
	quotaB, err := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	if err != nil {
		return 0
	}
	periodB, err := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if err != nil {
		return 0
	}
	quota, err := strconv.ParseFloat(strings.TrimSpace(string(quotaB)), 64)
	if err != nil || quota <= 0 {
		return 0 // -1 means no limit
	}
	period, err := strconv.ParseFloat(strings.TrimSpace(string(periodB)), 64)
	if err != nil || period <= 0 {
		return 0
	}
	cpus := int(quota / period)
	if cpus > 0 {
		return cpus
	}
	return 0
}
