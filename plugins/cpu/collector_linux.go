//go:build linux

package cpu

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"mld.com/dtop/pkg/types"
)

func readCPUStats(prev map[string]cpuTimes, showTemp bool) (types.CPUStats, map[string]cpuTimes, error) {
	samples, err := readCPUSamples("/proc/stat")
	if err != nil {
		return types.CPUStats{}, prev, err
	}

	stats := types.CPUStats{Timestamp: time.Now()}
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

	if showTemp {
		stats.TemperatureC = readTemperature("/sys/class/thermal")
	}

	return stats, samples, nil
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
