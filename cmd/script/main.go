package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alinz/script.go"
)

func main() {
	workspace := os.Args[1]
	arg := strings.Join(os.Args[2:], "")

	pluginSrcPaths := strings.Split(arg, ",")
	for i, path := range pluginSrcPaths {
		pluginSrcPaths[i] = strings.TrimSpace(path)
	}

	pluginPaths := make([]string, len(pluginSrcPaths))
	env := os.Environ()
	env = append(env, fmt.Sprintf("GOOS=%s", runtime.GOOS))

	for i, pluginSrcPath := range pluginSrcPaths {
		pluginPath := filepath.Join(os.TempDir(), "plugin.so")

		inputDir := filepath.Join(workspace, pluginSrcPath)

		cmd := exec.Cmd{
			Path: "go",
			Args: []string{
				"go", "build", "-buildmode=plugin", "-o", pluginPath,
			},
			Env: env,
			Dir: inputDir,
		}

		if filepath.Base(cmd.Path) == cmd.Path {
			if lp, err := exec.LookPath(cmd.Path); err != nil {
				log.Fatalf("failed to find go binary: %s", err)
			} else {
				cmd.Path = lp
			}
		}

		output, err := cmd.Output()
		if exiterr, ok := err.(*exec.ExitError); ok {
			log.Fatalf("failed to execute build plugin:\n%s", string(exiterr.Stderr))
		} else if err != nil {
			log.Fatalf("failed to execute build plugin: %v", err)
		}

		if len(output) > 0 {
			log.Fatalf("failed to build plugin %s: %s", pluginSrcPath, string(output))
			return
		}

		pluginPaths[i] = pluginPath
	}

	err := script.RunPlugins(pluginPaths...)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
