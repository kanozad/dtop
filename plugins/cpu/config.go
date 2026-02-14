package cpu

func parseConfig(defaults Config, cfg map[string]any) Config {
	out := defaults
	if cfg == nil {
		return out
	}
	if v, ok := cfg["per_core"]; ok {
		if b, ok := v.(bool); ok {
			out.PerCore = b
		}
	}
	if v, ok := cfg["show_temp"]; ok {
		if b, ok := v.(bool); ok {
			out.ShowTemp = b
		}
	}
	if v, ok := cfg["show_freq"]; ok {
		if b, ok := v.(bool); ok {
			out.ShowFreq = b
		}
	}
	if v, ok := cfg["show_watts"]; ok {
		if b, ok := v.(bool); ok {
			out.ShowWatts = b
		}
	}
	return out
}
