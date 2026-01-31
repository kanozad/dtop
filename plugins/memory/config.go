package memory

func parseConfig(defaults Config, cfg map[string]any) Config {
	out := defaults
	if cfg == nil {
		return out
	}
	if v, ok := cfg["show_swap"]; ok {
		if b, ok := v.(bool); ok {
			out.ShowSwap = b
		}
	}
	if v, ok := cfg["show_disks"]; ok {
		if b, ok := v.(bool); ok {
			out.ShowDisks = b
		}
	}
	if v, ok := cfg["show_io_stat"]; ok {
		if b, ok := v.(bool); ok {
			out.ShowIOStat = b
		}
	}
	if v, ok := cfg["base_10_sizes"]; ok {
		if b, ok := v.(bool); ok {
			out.Base10Sizes = b
		}
	}
	if v, ok := cfg["zfs_arc_cached"]; ok {
		if b, ok := v.(bool); ok {
			out.ZFSARCCached = b
		}
	}
	if v, ok := cfg["disks_filter"]; ok {
		if arr, ok := v.([]any); ok {
			out.DisksFilter = make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					out.DisksFilter = append(out.DisksFilter, s)
				}
			}
		}
	}
	return out
}
