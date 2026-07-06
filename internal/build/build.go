package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Scripts compiles Go scripts as plugins and returns their paths.
// Each script directory must have a go.mod file.
func Scripts(scriptDirs []string) ([]string, error) {
	buildDir, err := os.MkdirTemp("", "script-go-plugins-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	var pluginPaths []string

	for i, scriptDir := range scriptDirs {
		// Each plugin needs its own output file; sharing one path would make
		// every entry point at the last plugin built
		pluginPath := filepath.Join(buildDir, fmt.Sprintf("plugin-%d.so", i))

		fmt.Printf("[ BUILD ]: %s\n", scriptDir)

		cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", pluginPath, ".")
		cmd.Dir = scriptDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			os.RemoveAll(buildDir)
			return nil, fmt.Errorf("failed to build plugin %s: %w", scriptDir, err)
		}

		pluginPaths = append(pluginPaths, pluginPath)
	}

	return pluginPaths, nil
}

// Cleanup removes the temporary build directory containing the plugins.
// The caller should defer this to ensure cleanup.
func Cleanup(pluginPaths []string) error {
	if len(pluginPaths) == 0 {
		return nil
	}
	// All plugins are in the same temp directory; remove it
	buildDir := filepath.Dir(pluginPaths[0])
	return os.RemoveAll(buildDir)
}
