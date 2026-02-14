package gpu

// Config holds GPU plugin configuration.
type Config struct {
	ShowEncoder bool
	ShowPCIe    bool
}

func parseConfig(base Config, raw map[string]any) Config {
	if raw == nil {
		return base
	}
	if v, ok := raw["show_encoder"].(bool); ok {
		base.ShowEncoder = v
	}
	if v, ok := raw["show_pcie"].(bool); ok {
		base.ShowPCIe = v
	}
	return base
}
