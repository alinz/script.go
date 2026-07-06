// Package plugins loads compiled Go plugins and invokes their exported
// Runner symbol, which must be a `func(workspace string) error`.
package plugins

import (
	"fmt"
	"plugin"
	"strings"
)

// Run opens each plugin in order and calls its Runner symbol with workspace.
// It stops at the first error.
func Run(workspace string, paths ...string) error {
	workspace = strings.TrimSuffix(workspace, "/")

	for _, path := range paths {
		p, err := plugin.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open plugin %s: %w", path, err)
		}
		symbol, err := p.Lookup("Runner")
		if err != nil {
			return fmt.Errorf("failed to lookup Runner in plugin %s: %w", path, err)
		}

		// a top-level `func Runner(...)` surfaces as the func value itself,
		// while `var Runner = func(...)` surfaces as a pointer to it
		var runner func(string) error
		switch v := symbol.(type) {
		case func(string) error:
			runner = v
		case *func(string) error:
			runner = *v
		default:
			return fmt.Errorf("symbol Runner in plugin %s is not a 'func(string) error' type", path)
		}

		if err := runner(workspace); err != nil {
			return fmt.Errorf("failed to run plugin %s: %w", path, err)
		}
	}

	return nil
}
