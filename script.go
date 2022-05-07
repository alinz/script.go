package script

import (
	"fmt"
	"plugin"
)

func RunPlugins(paths ...string) error {
	for _, path := range paths {
		p, err := plugin.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open plugin %s: %w", path, err)
		}
		symbol, err := p.Lookup("Register")
		if err != nil {
			return fmt.Errorf("failed to lookup Register in plugin %s: %w", path, err)
		}

		register, ok := symbol.(func() func() error)
		if !ok {
			return fmt.Errorf("symbol Register in plugin %s is not a register function", path)
		}

		runner := register()
		if err := runner(); err != nil {
			return fmt.Errorf("failed to run plugin %s: %w", path, err)
		}
	}

	return nil
}
