//go:build linux

package process

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kanozad/dtop/pkg/types"
)

// readProcessStats collects process information from /proc on Linux.
func readProcessStats(prevProc map[int]procStat, prevSys systemStat, cfg Config, uidCache map[int]string) (types.ProcessStats, map[int]procStat, systemStat, error) {
	stats := types.ProcessStats{
		Timestamp: time.Now(),
		SortBy:    parseSortField(cfg.SortBy),
		SortDesc:  true, // Most useful sorts are descending (CPU, mem)
		FilterStr: cfg.FilterString,
	}

	// Read current system CPU time
	sysStat, err := readSystemCPU()
	if err != nil {
		return stats, prevProc, prevSys, fmt.Errorf("failed to read system CPU: %w", err)
	}

	// Read all processes from /proc
	processes, nextProc, err := collectProcesses(prevProc, prevSys, sysStat, cfg.UseSmaps, uidCache)
	if err != nil {
		return stats, prevProc, prevSys, err
	}

	stats.TotalCount = len(processes)

	// Apply filtering
	if cfg.FilterString != "" {
		processes = filterProcesses(processes, cfg.FilterString)
	}
	stats.FilteredCount = len(processes)

	// Tree mode re-orders by parent-child-PID; sorting beforehand has no effect
	// on the output, so skip it to avoid wasted work.
	if cfg.TreeView {
		processes = buildProcessTree(processes)
	} else {
		sortProcesses(processes, stats.SortBy, stats.SortDesc)
	}

	stats.Processes = processes

	return stats, nextProc, sysStat, nil
}

// readSystemCPU reads total system CPU time from /proc/stat.
func readSystemCPU() (systemStat, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return systemStat{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var total uint64
	var haveCPU bool
	var bootTime time.Time

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				return systemStat{}, fmt.Errorf("insufficient fields in /proc/stat")
			}
			for i := 1; i < len(fields); i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					continue
				}
				total += val
			}
			haveCPU = true
			continue
		}
		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if secs, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					bootTime = time.Unix(secs, 0)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return systemStat{}, err
	}
	if !haveCPU {
		return systemStat{}, fmt.Errorf("unexpected /proc/stat format")
	}
	if up, err := readSystemUptime(); err == nil {
		if bootTime.IsZero() {
			bootTime = time.Now().Add(-up)
		}
	}

	// Linux always presents jiffies to userspace at USER_HZ = 100,
	// regardless of the kernel's internal CONFIG_HZ.
	const clockTicks int64 = 100

	return systemStat{
		totalTime:  total,
		timestamp:  time.Now(),
		bootTime:   bootTime,
		clockTicks: clockTicks,
	}, nil
}

// collectProcesses reads all processes from /proc and calculates CPU percentages.
func collectProcesses(prevProc map[int]procStat, prevSys systemStat, curSys systemStat, useSmaps bool, uidCache map[int]string) ([]types.ProcessInfo, map[int]procStat, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, nil, err
	}

	var processes []types.ProcessInfo
	nextProc := make(map[int]procStat)

	// System-level CPU time delta
	sysDelta := float64(curSys.totalTime - prevSys.totalTime)
	if sysDelta <= 0 {
		sysDelta = 1 // Avoid division by zero on first run
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a PID (numeric)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		proc, curStat, err := readProcInfo(pid, prevProc, sysDelta, curSys, useSmaps, uidCache)
		if err != nil {
			// Process may have exited, skip
			continue
		}

		processes = append(processes, proc)
		nextProc[pid] = curStat
	}

	return processes, nextProc, nil
}

// readProcInfo reads information for a single process.
func readProcInfo(pid int, prevProc map[int]procStat, sysDelta float64, curSys systemStat, useSmaps bool, uidCache map[int]string) (types.ProcessInfo, procStat, error) {
	procPath := filepath.Join("/proc", strconv.Itoa(pid))

	// Read /proc/[pid]/stat for most information
	stat, curStat, err := readProcStat(procPath, curSys.bootTime, curSys.clockTicks)
	if err != nil {
		return types.ProcessInfo{}, procStat{}, err
	}

	// Calculate CPU percentage
	cpuPercent := 0.0
	if prev, ok := prevProc[pid]; ok {
		procDelta := float64((curStat.utime + curStat.stime) - (prev.utime + prev.stime))
		if sysDelta > 0 {
			cpuPercent = (procDelta / sysDelta) * 100.0
		}
	}

	// Read /proc/[pid]/status for additional details
	status, err := readProcStatus(procPath)
	if err != nil {
		// Some fields may be unavailable, continue with what we have
		status = make(map[string]string)
	}

	// Get username, using the per-collector cache to avoid a lookup syscall on every tick.
	username, ok := uidCache[stat.uid]
	if !ok {
		username = getUserName(stat.uid)
		uidCache[stat.uid] = username
	}

	// Read command line
	cmdline := readProcCmdline(procPath)
	command := stat.comm
	if cmdline != "" {
		command = filepath.Base(strings.Fields(cmdline)[0])
	}

	// Parse memory from status, optionally override with smaps
	memBytes := parseMemory(status)
	if useSmaps {
		if smapsBytes, err := readProcSmaps(procPath); err == nil && smapsBytes > 0 {
			memBytes = smapsBytes
		}
	}

	proc := types.ProcessInfo{
		PID:        pid,
		PPID:       stat.ppid,
		User:       username,
		State:      types.ProcessState(stat.state),
		Command:    command,
		FullCmd:    cmdline,
		Threads:    stat.numThreads,
		Nice:       stat.nice,
		CPUPercent: cpuPercent,
		MemBytes:   memBytes,
		StartTime:  stat.startTime,
	}

	return proc, curStat, nil
}

// statInfo holds parsed /proc/[pid]/stat data.
type statInfo struct {
	pid        int
	comm       string
	state      string
	ppid       int
	utime      uint64
	stime      uint64
	nice       int
	numThreads int
	startTime  time.Time
	uid        int
}

// readProcStat parses /proc/[pid]/stat.
func readProcStat(procPath string, bootTime time.Time, clockTicks int64) (statInfo, procStat, error) {
	data, err := os.ReadFile(filepath.Join(procPath, "stat"))
	if err != nil {
		return statInfo{}, procStat{}, err
	}

	line := string(data)

	// Parse the stat line - format is complex due to comm field possibly containing spaces
	// Find the last ')' to locate the end of the comm field
	commEnd := strings.LastIndex(line, ")")
	if commEnd == -1 {
		return statInfo{}, procStat{}, fmt.Errorf("invalid stat format")
	}

	// Extract comm (between first '(' and last ')')
	commStart := strings.Index(line, "(")
	if commStart == -1 {
		return statInfo{}, procStat{}, fmt.Errorf("invalid stat format")
	}
	comm := line[commStart+1 : commEnd]

	// Parse PID before '('
	pidStr := strings.TrimSpace(line[:commStart])
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return statInfo{}, procStat{}, err
	}

	// Parse fields after ')'
	fields := strings.Fields(line[commEnd+1:])
	if len(fields) < 20 {
		return statInfo{}, procStat{}, fmt.Errorf("insufficient fields in stat")
	}

	state := fields[0]
	ppid, _ := strconv.Atoi(fields[1])
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)
	nice, _ := strconv.Atoi(fields[16])
	numThreads, _ := strconv.Atoi(fields[17])
	starttime, _ := strconv.ParseUint(fields[19], 10, 64)

	// Get UID from file stat
	fileInfo, err := os.Stat(procPath)
	uid := 0
	if err == nil {
		if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			uid = int(stat.Uid)
		}
	}

	// Convert starttime (clock ticks since boot) to actual time. starttime is
	// measured from boot, so without a known boot time we can't derive a real
	// wall-clock value; leave it zero (rendered as "n/a") rather than guess.
	startTime := time.Time{}
	if !bootTime.IsZero() && clockTicks > 0 {
		seconds := float64(starttime) / float64(clockTicks)
		startTime = bootTime.Add(time.Duration(seconds * float64(time.Second)))
	}

	info := statInfo{
		pid:        pid,
		comm:       comm,
		state:      state,
		ppid:       ppid,
		utime:      utime,
		stime:      stime,
		nice:       nice,
		numThreads: numThreads,
		startTime:  startTime,
		uid:        uid,
	}

	curStat := procStat{
		utime:     utime,
		stime:     stime,
		timestamp: time.Now(),
	}

	return info, curStat, nil
}

// readProcStatus parses /proc/[pid]/status.
func readProcStatus(procPath string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(procPath, "status"))
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}
	return result, nil
}

// readProcCmdline reads /proc/[pid]/cmdline.
func readProcCmdline(procPath string) string {
	data, err := os.ReadFile(filepath.Join(procPath, "cmdline"))
	if err != nil {
		return ""
	}
	// Replace null bytes with spaces
	cmdline := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.TrimSpace(cmdline)
}

// getUserName converts UID to username.
func getUserName(uid int) string {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return strconv.Itoa(uid)
	}
	return u.Username
}

// parseMemory extracts memory usage from /proc/[pid]/status.
func parseMemory(status map[string]string) uint64 {
	// VmRSS is the resident set size (actual physical memory used)
	if rss, ok := status["VmRSS"]; ok {
		// Value is in kB, e.g., "1234 kB"
		fields := strings.Fields(rss)
		if len(fields) > 0 {
			if val, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
				return val * 1024 // Convert kB to bytes
			}
		}
	}
	return 0
}

func readSystemUptime() (time.Duration, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("invalid /proc/uptime format")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func readProcSmaps(procPath string) (uint64, error) {
	file, err := os.Open(filepath.Join(procPath, "smaps"))
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var pss uint64
	var rss uint64
	var havePss bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Pss:") {
			if val, ok := parseSmapsValue(line); ok {
				pss += val
				havePss = true
			}
			continue
		}
		if strings.HasPrefix(line, "Rss:") {
			if val, ok := parseSmapsValue(line); ok {
				rss += val
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if havePss {
		return pss * 1024, nil
	}
	if rss > 0 {
		return rss * 1024, nil
	}
	return 0, nil
}

func parseSmapsValue(line string) (uint64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	val, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}
