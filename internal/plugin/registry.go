package plugin

import (
	"context"
	"fmt"
)

type Factory func() Plugin

type Registry struct {
	factories map[ID]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: map[ID]Factory{}}
}

func (r *Registry) Register(f Factory) error {
	p := f()
	id := p.ID()
	if id == "" {
		return fmt.Errorf("plugin id cannot be empty")
	}
	if _, ok := r.factories[id]; ok {
		return fmt.Errorf("plugin %q already registered", id)
	}
	r.factories[id] = f
	return nil
}

func (r *Registry) Instantiate(ctx context.Context, enabled []string, cfgByID map[string]map[string]any) ([]Plugin, error) {
	if cfgByID == nil {
		cfgByID = map[string]map[string]any{}
	}

	// If enabled list is empty, start none (explicit).
	plugins := make([]Plugin, 0, len(enabled))
	for _, idStr := range enabled {
		id := ID(idStr)
		f, ok := r.factories[id]
		if !ok {
			return nil, fmt.Errorf("plugin %q not registered", id)
		}
		p := f()
		cfg := cfgByID[idStr]
		if validator, ok := p.(ConfigValidator); ok {
			if err := ValidateConfig(p.ID(), cfg, validator.AllowedConfigKeys()...); err != nil {
				return nil, err
			}
		}
		if err := p.Init(ctx, cfg); err != nil {
			return nil, fmt.Errorf("init plugin %q: %w", id, err)
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}
