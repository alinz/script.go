package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alinz/script.go/v2/internal/build"
	"github.com/alinz/script.go/v2/internal/plugins"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 3 {
		log.Fatalf("usage: %s <workspace> <comma-separated-script-paths>", filepath.Base(os.Args[0]))
	}

	workspace := os.Args[1]
	arg := strings.Join(os.Args[2:], "")

	var pluginSrcPaths []string
	for path := range strings.SplitSeq(arg, ",") {
		path = strings.TrimSpace(path)
		if path != "" {
			pluginSrcPaths = append(pluginSrcPaths, path)
		}
	}

	if len(pluginSrcPaths) == 0 {
		log.Fatal("no script paths provided")
	}

	// Build scripts relative to workspace
	var scriptDirs []string
	for _, scriptPath := range pluginSrcPaths {
		scriptDirs = append(scriptDirs, filepath.Join(workspace, scriptPath))
	}

	pluginPaths, err := build.Scripts(scriptDirs)
	if err != nil {
		log.Fatalf("failed to build scripts: %v", err)
	}
	defer build.Cleanup(pluginPaths)

	if err := plugins.Run(workspace, pluginPaths...); err != nil {
		log.Fatal(err)
	}
}
