package types

import "time"

// ProcessState represents the process state (R, S, D, Z, T, etc.)
type ProcessState string

const (
	StateRunning  ProcessState = "R" // Running
	StateSleeping ProcessState = "S" // Sleeping
	StateWaiting  ProcessState = "D" // Disk sleep
	StateZombie   ProcessState = "Z" // Zombie
	StateStopped  ProcessState = "T" // Stopped
	StateTracing  ProcessState = "t" // Tracing stop
	StateDead     ProcessState = "X" // Dead
)

// ProcessInfo represents a single process snapshot.
type ProcessInfo struct {
	PID        int
	PPID       int
	User       string
	State      ProcessState
	Command    string // Short command name
	FullCmd    string // Full command line
	Threads    int
	Nice       int
	CPUPercent float64   // CPU usage percentage
	MemBytes   uint64    // Memory usage in bytes
	StartTime  time.Time // Process start time

	// Tree view metadata
	TreeDepth     int    // Depth in process tree
	TreePrefix    string // Visual prefix for tree display (e.g., "├─ ", "└─ ")
	TreeCollapsed bool   // Whether this node's children are hidden
}

// ProcessStats captures all process information for display.
type ProcessStats struct {
	Processes     []ProcessInfo
	TotalCount    int
	FilteredCount int
	Timestamp     time.Time

	// Sort and filter state (maintained by plugin)
	SortBy    ProcessSortField
	SortDesc  bool
	FilterStr string
}

// ProcessSortField defines the field to sort processes by.
type ProcessSortField int

const (
	SortByPID ProcessSortField = iota
	SortByName
	SortByCPU
	SortByMemory
	SortByThreads
	SortByUser
)
