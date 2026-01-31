package process

import "mld.com/dtop/pkg/types"

// Config holds configuration options for the process plugin.
type Config struct {
	TreeView     bool   // Show process tree view
	SortBy       string // Sort field: pid, name, cpu, mem, threads, user
	FilterString string // Filter processes by name/command
	MaxDisplay   int    // Maximum number of processes to display
}

func parseConfig(defaults Config, cfg map[string]any) Config {
	out := defaults
	if cfg == nil {
		return out
	}
	if v, ok := cfg["tree_view"]; ok {
		if b, ok := v.(bool); ok {
			out.TreeView = b
		}
	}
	if v, ok := cfg["sort_by"]; ok {
		if s, ok := v.(string); ok {
			out.SortBy = s
		}
	}
	if v, ok := cfg["filter"]; ok {
		if s, ok := v.(string); ok {
			out.FilterString = s
		}
	}
	if v, ok := cfg["max_display"]; ok {
		if i, ok := v.(int64); ok {
			out.MaxDisplay = int(i)
		} else if i, ok := v.(int); ok {
			out.MaxDisplay = i
		}
	}
	return out
}

// parseSortField converts a config string to ProcessSortField.
func parseSortField(s string) types.ProcessSortField {
	switch s {
	case "name":
		return types.SortByName
	case "cpu":
		return types.SortByCPU
	case "mem", "memory":
		return types.SortByMemory
	case "threads":
		return types.SortByThreads
	case "user":
		return types.SortByUser
	default:
		return types.SortByPID
	}
}
