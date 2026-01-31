package network

import "strings"

type Config struct {
	Interface string
	ShowIPv6  bool
}

func parseConfig(defaults Config, cfg map[string]any) Config {
	out := defaults
	if cfg == nil {
		return out
	}
	if v, ok := cfg["interface"]; ok {
		if s, ok := v.(string); ok {
			out.Interface = strings.TrimSpace(s)
		}
	}
	if v, ok := cfg["show_ipv6"]; ok {
		if b, ok := v.(bool); ok {
			out.ShowIPv6 = b
		}
	}
	return out
}
