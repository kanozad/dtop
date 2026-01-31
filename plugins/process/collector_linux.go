//go:build linux

package process

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mld.com/dtop/pkg/types"
)

// readProcessStats collects process information from /proc on Linux.
func readProcessStats(prevProc map[int]procStat, prevSys systemStat, cfg Config) (types.ProcessStats, map[int]procStat, systemStat, error) {
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
	processes, nextProc, err := collectProcesses(prevProc, prevSys, sysStat)
	if err != nil {
		return stats, prevProc, prevSys, err
	}

	stats.TotalCount = len(processes)

	// Apply filtering
	if cfg.FilterString != "" {
		processes = filterProcesses(processes, cfg.FilterString)
	}
	stats.FilteredCount = len(processes)

	// Sort processes
	sortProcesses(processes, stats.SortBy, stats.SortDesc)

	// Build tree view if enabled
	if cfg.TreeView {
		processes = buildProcessTree(processes)
	}

	// Limit display count
	if cfg.MaxDisplay > 0 && len(processes) > cfg.MaxDisplay {
		processes = processes[:cfg.MaxDisplay]
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
	if !scanner.Scan() {
		return systemStat{}, fmt.Errorf("failed to read /proc/stat")
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return systemStat{}, fmt.Errorf("unexpected /proc/stat format")
	}

	fields := strings.Fields(line)
	if len(fields) < 8 {
		return systemStat{}, fmt.Errorf("insufficient fields in /proc/stat")
	}

	// Sum all CPU time fields (user, nice, system, idle, iowait, irq, softirq, steal)
	var total uint64
	for i := 1; i < len(fields); i++ {
		val, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			continue
		}
		total += val
	}

	return systemStat{
		totalTime: total,
		timestamp: time.Now(),
	}, nil
}

// collectProcesses reads all processes from /proc and calculates CPU percentages.
func collectProcesses(prevProc map[int]procStat, prevSys systemStat, curSys systemStat) ([]types.ProcessInfo, map[int]procStat, error) {
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

		proc, curStat, err := readProcInfo(pid, prevProc, sysDelta)
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
func readProcInfo(pid int, prevProc map[int]procStat, sysDelta float64) (types.ProcessInfo, procStat, error) {
	procPath := filepath.Join("/proc", strconv.Itoa(pid))

	// Read /proc/[pid]/stat for most information
	stat, curStat, err := readProcStat(procPath)
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

	// Get username
	username := getUserName(stat.uid)

	// Read command line
	cmdline := readProcCmdline(procPath)
	command := stat.comm
	if cmdline != "" {
		command = filepath.Base(strings.Fields(cmdline)[0])
	}

	// Parse memory from status
	memBytes := parseMemory(status)

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
func readProcStat(procPath string) (statInfo, procStat, error) {
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
	if len(fields) < 50 {
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

	// Convert starttime (jiffies since boot) to actual time
	// This is approximate - requires reading /proc/uptime and boot time
	startTime := time.Now().Add(-time.Duration(starttime) * time.Millisecond / 100) // Rough approximation

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
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
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
