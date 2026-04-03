package process

import (
	"sort"
	"strings"

	"github.com/kanozad/dtop/pkg/types"
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
		if ppid == 0 || ppid == processes[i].PID || procMap[ppid] == nil {
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

func filterFollow(processes []types.ProcessInfo, pid int, includeTree bool) []types.ProcessInfo {
	if pid <= 0 {
		return processes
	}
	if !includeTree {
		for _, proc := range processes {
			if proc.PID == pid {
				return []types.ProcessInfo{proc}
			}
		}
		return nil
	}

	children := make(map[int][]int)
	for _, proc := range processes {
		children[proc.PPID] = append(children[proc.PPID], proc.PID)
	}

	allowed := make(map[int]struct{})
	queue := []int{pid}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, ok := allowed[cur]; ok {
			continue
		}
		allowed[cur] = struct{}{}
		queue = append(queue, children[cur]...)
	}

	var filtered []types.ProcessInfo
	for _, proc := range processes {
		if _, ok := allowed[proc.PID]; ok {
			filtered = append(filtered, proc)
		}
	}
	return filtered
}

func applyTreeCollapse(processes []types.ProcessInfo, collapsed map[int]struct{}) []types.ProcessInfo {
	if len(collapsed) == 0 {
		return processes
	}
	result := make([]types.ProcessInfo, 0, len(processes))
	skipDepth := -1
	for _, proc := range processes {
		if skipDepth >= 0 {
			if proc.TreeDepth > skipDepth {
				continue
			}
			skipDepth = -1
		}
		if _, ok := collapsed[proc.PID]; ok {
			proc.TreeCollapsed = true
			result = append(result, proc)
			skipDepth = proc.TreeDepth
			continue
		}
		proc.TreeCollapsed = false
		result = append(result, proc)
	}
	return result
}

func pruneCollapsed(processes []types.ProcessInfo, collapsed map[int]struct{}) map[int]struct{} {
	if len(collapsed) == 0 {
		return collapsed
	}
	present := make(map[int]struct{}, len(processes))
	for _, proc := range processes {
		present[proc.PID] = struct{}{}
	}
	pruned := make(map[int]struct{})
	for pid := range collapsed {
		if _, ok := present[pid]; ok {
			pruned[pid] = struct{}{}
		}
	}
	return pruned
}

func indexByPID(processes []types.ProcessInfo, pid int) int {
	if pid == 0 {
		return -1
	}
	for i, proc := range processes {
		if proc.PID == pid {
			return i
		}
	}
	return -1
}

func hasChildren(processes []types.ProcessInfo, idx int) bool {
	if idx < 0 || idx+1 >= len(processes) {
		return false
	}
	return processes[idx+1].TreeDepth > processes[idx].TreeDepth
}
