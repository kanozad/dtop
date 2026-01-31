package process

import (
	"sort"
	"strings"

	"mld.com/dtop/pkg/types"
)

// filterProcesses filters processes by name or command.
func filterProcesses(processes []types.ProcessInfo, filter string) []types.ProcessInfo {
	if filter == "" {
		return processes
	}

	filter = strings.ToLower(filter)
	var filtered []types.ProcessInfo
	for _, p := range processes {
		if strings.Contains(strings.ToLower(p.Command), filter) ||
			strings.Contains(strings.ToLower(p.FullCmd), filter) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// sortProcesses sorts processes by the specified field.
func sortProcesses(processes []types.ProcessInfo, sortBy types.ProcessSortField, desc bool) {
	sort.Slice(processes, func(i, j int) bool {
		var less bool
		switch sortBy {
		case types.SortByName:
			less = processes[i].Command < processes[j].Command
		case types.SortByCPU:
			less = processes[i].CPUPercent < processes[j].CPUPercent
		case types.SortByMemory:
			less = processes[i].MemBytes < processes[j].MemBytes
		case types.SortByThreads:
			less = processes[i].Threads < processes[j].Threads
		case types.SortByUser:
			less = processes[i].User < processes[j].User
		default: // SortByPID
			less = processes[i].PID < processes[j].PID
		}
		if desc {
			return !less
		}
		return less
	})
}

// buildProcessTree constructs a process tree and adds tree metadata.
func buildProcessTree(processes []types.ProcessInfo) []types.ProcessInfo {
	// Build a map of PID -> Process for quick lookup
	procMap := make(map[int]*types.ProcessInfo)
	for i := range processes {
		procMap[processes[i].PID] = &processes[i]
	}

	// Build a map of PPID -> children
	children := make(map[int][]int)
	var roots []int
	for i := range processes {
		ppid := processes[i].PPID
		if ppid == 0 || procMap[ppid] == nil {
			// Root process or parent not in list
			roots = append(roots, processes[i].PID)
		} else {
			children[ppid] = append(children[ppid], processes[i].PID)
		}
	}

	// Sort children by PID for consistent ordering
	for _, kids := range children {
		sort.Ints(kids)
	}

	// Traverse tree and assign depth/prefix
	var result []types.ProcessInfo
	for _, rootPID := range roots {
		traverseTree(rootPID, 0, "", procMap, children, &result, true)
	}

	return result
}

// traverseTree recursively traverses the process tree and builds the display list.
func traverseTree(pid int, depth int, prefix string, procMap map[int]*types.ProcessInfo,
	children map[int][]int, result *[]types.ProcessInfo, isLast bool) {

	proc := procMap[pid]
	if proc == nil {
		return
	}

	// Set tree metadata
	proc.TreeDepth = depth
	proc.TreePrefix = prefix

	// Add to result
	*result = append(*result, *proc)

	// Process children
	kids := children[pid]
	for i, childPID := range kids {
		childIsLast := i == len(kids)-1

		// Build prefix for child
		var childPrefix string
		if depth == 0 {
			if childIsLast {
				childPrefix = "└─ "
			} else {
				childPrefix = "├─ "
			}
		} else {
			if isLast {
				childPrefix = prefix + "   "
			} else {
				childPrefix = prefix + "│  "
			}
			if childIsLast {
				childPrefix += "└─ "
			} else {
				childPrefix += "├─ "
			}
		}

		traverseTree(childPID, depth+1, childPrefix, procMap, children, result, childIsLast)
	}
}
