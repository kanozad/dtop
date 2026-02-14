package battery

// Config holds battery plugin configuration.
type Config struct{}

func parseConfig(base Config, _ map[string]any) Config {
	return base
}
