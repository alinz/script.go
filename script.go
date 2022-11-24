package script

import (
	"fmt"
	"plugin"
)

func RunPlugins(workspace string, paths ...string) error {
	for _, path := range paths {
		p, err := plugin.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open plugin %s: %w", path, err)
		}
		symbol, err := p.Lookup("Runner")
		if err != nil {
			return fmt.Errorf("failed to lookup Runner in plugin %s: %w", path, err)
		}

		runner, ok := symbol.(func(string) error)
		if !ok {
			return fmt.Errorf("symbol Runner in plugin %s is not a 'func(string) error' type", path)
		}

		if err := runner(workspace); err != nil {
			return fmt.Errorf("failed to run plugin %s: %w", path, err)
		}
	}

	return nil
}
