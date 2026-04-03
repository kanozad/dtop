//go:build linux

package process

import (
	"testing"

	"github.com/kanozad/dtop/pkg/types"
)

func TestFilterProcesses(t *testing.T) {
	processes := []types.ProcessInfo{
		{PID: 1, Command: "systemd", FullCmd: "/sbin/systemd"},
		{PID: 2, Command: "kthreadd", FullCmd: "kthreadd"},
		{PID: 100, Command: "python3", FullCmd: "/usr/bin/python3 script.py"},
		{PID: 200, Command: "go", FullCmd: "/usr/local/go/bin/go build"},
	}

	tests := []struct {
		name     string
		filter   string
		expected int
	}{
		{"no filter", "", 4},
		{"filter python", "python", 1},
		{"filter go", "go", 1},
		{"filter system", "system", 1},
		{"filter nonexistent", "nonexistent", 0},
		{"case insensitive", "PYTHON", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterProcesses(processes, tt.filter)
			if len(filtered) != tt.expected {
				t.Errorf("filterProcesses(%q) returned %d processes, expected %d",
					tt.filter, len(filtered), tt.expected)
			}
		})
	}
}

func TestSortProcesses(t *testing.T) {
	processes := []types.ProcessInfo{
		{PID: 3, Command: "c_process", CPUPercent: 10.0, MemBytes: 2000, User: "bob"},
		{PID: 1, Command: "a_process", CPUPercent: 50.0, MemBytes: 1000, User: "alice"},
		{PID: 2, Command: "b_process", CPUPercent: 25.0, MemBytes: 3000, User: "charlie"},
	}

	tests := []struct {
		name     string
		sortBy   types.ProcessSortField
		desc     bool
		firstPID int
	}{
		{"sort by PID asc", types.SortByPID, false, 1},
		{"sort by PID desc", types.SortByPID, true, 3},
		{"sort by Name asc", types.SortByName, false, 1},
		{"sort by Name desc", types.SortByName, true, 3},
		{"sort by CPU desc", types.SortByCPU, true, 1},
		{"sort by CPU asc", types.SortByCPU, false, 3},
		{"sort by Memory desc", types.SortByMemory, true, 2},
		{"sort by Memory asc", types.SortByMemory, false, 1},
		{"sort by User asc", types.SortByUser, false, 1},
		{"sort by User desc", types.SortByUser, true, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			procs := make([]types.ProcessInfo, len(processes))
			copy(procs, processes)

			sortProcesses(procs, tt.sortBy, tt.desc)

			if procs[0].PID != tt.firstPID {
				t.Errorf("after sort, first process PID = %d, expected %d",
					procs[0].PID, tt.firstPID)
			}
		})
	}
}

func TestBuildProcessTree(t *testing.T) {
	processes := []types.ProcessInfo{
		{PID: 1, PPID: 0, Command: "init"},
		{PID: 10, PPID: 1, Command: "child1"},
		{PID: 11, PPID: 1, Command: "child2"},
		{PID: 100, PPID: 10, Command: "grandchild1"},
		{PID: 101, PPID: 10, Command: "grandchild2"},
	}

	result := buildProcessTree(processes)

	// Check that tree structure is maintained
	if len(result) != len(processes) {
		t.Errorf("buildProcessTree returned %d processes, expected %d",
			len(result), len(processes))
	}

	// Verify tree metadata is set
	foundRoot := false
	foundChildren := 0
	foundGrandchildren := 0

	for _, p := range result {
		if p.PID == 1 {
			foundRoot = true
			if p.TreeDepth != 0 {
				t.Errorf("root process depth = %d, expected 0", p.TreeDepth)
			}
		}
		if p.PPID == 1 {
			foundChildren++
			if p.TreeDepth != 1 {
				t.Errorf("child process %d depth = %d, expected 1", p.PID, p.TreeDepth)
			}
			if p.TreePrefix == "" {
				t.Errorf("child process %d has empty TreePrefix", p.PID)
			}
		}
		if p.PPID == 10 {
			foundGrandchildren++
			if p.TreeDepth != 2 {
				t.Errorf("grandchild process %d depth = %d, expected 2", p.PID, p.TreeDepth)
			}
		}
	}

	if !foundRoot {
		t.Error("root process not found in tree")
	}
	if foundChildren != 2 {
		t.Errorf("found %d children, expected 2", foundChildren)
	}
	if foundGrandchildren != 2 {
		t.Errorf("found %d grandchildren, expected 2", foundGrandchildren)
	}
}

func TestBuildProcessTreeSelfReference(t *testing.T) {
	// A process whose PPID == PID must not cause infinite recursion.
	processes := []types.ProcessInfo{
		{PID: 1, PPID: 0, Command: "init"},
		{PID: 2, PPID: 2, Command: "self-ref"}, // PPID == PID
	}

	result := buildProcessTree(processes)

	if len(result) != len(processes) {
		t.Errorf("buildProcessTree returned %d processes, expected %d", len(result), len(processes))
	}
}

func TestParseSortField(t *testing.T) {
	tests := []struct {
		input    string
		expected types.ProcessSortField
	}{
		{"pid", types.SortByPID},
		{"name", types.SortByName},
		{"cpu", types.SortByCPU},
		{"mem", types.SortByMemory},
		{"memory", types.SortByMemory},
		{"threads", types.SortByThreads},
		{"user", types.SortByUser},
		{"invalid", types.SortByPID}, // defaults to PID
		{"", types.SortByPID},        // defaults to PID
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSortField(tt.input)
			if result != tt.expected {
				t.Errorf("parseSortField(%q) = %v, expected %v",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0B"},
		{100, "100B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1048576, "1.0MB"},
		{1073741824, "1.0GB"},
		{1099511627776, "1.0TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %q, expected %q",
					tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestFilterFollow(t *testing.T) {
	processes := []types.ProcessInfo{
		{PID: 1, PPID: 0, Command: "init"},
		{PID: 10, PPID: 1, Command: "child1"},
		{PID: 11, PPID: 1, Command: "child2"},
		{PID: 100, PPID: 10, Command: "grandchild1"},
		{PID: 101, PPID: 10, Command: "grandchild2"},
	}
	tree := buildProcessTree(processes)

	filtered := filterFollow(tree, 10, true)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(filtered))
	}
	if filtered[0].PID != 10 || filtered[1].PID != 100 || filtered[2].PID != 101 {
		t.Fatalf("unexpected follow order: %+v", filtered)
	}

	single := filterFollow(tree, 10, false)
	if len(single) != 1 || single[0].PID != 10 {
		t.Fatalf("expected single PID 10, got %+v", single)
	}
}

func TestApplyTreeCollapse(t *testing.T) {
	processes := []types.ProcessInfo{
		{PID: 1, PPID: 0, Command: "init"},
		{PID: 10, PPID: 1, Command: "child1"},
		{PID: 11, PPID: 1, Command: "child2"},
		{PID: 100, PPID: 10, Command: "grandchild1"},
		{PID: 101, PPID: 10, Command: "grandchild2"},
	}
	tree := buildProcessTree(processes)
	collapsed := map[int]struct{}{10: {}}

	result := applyTreeCollapse(tree, collapsed)
	if len(result) != 3 {
		t.Fatalf("expected 3 processes after collapse, got %d", len(result))
	}
	if result[1].PID != 10 || !result[1].TreeCollapsed {
		t.Fatalf("expected PID 10 collapsed, got %+v", result[1])
	}
	if result[2].PID != 11 {
		t.Fatalf("expected PID 11 after collapsed subtree, got %+v", result[2])
	}
}
