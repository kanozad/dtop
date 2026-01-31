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
	return out
}
