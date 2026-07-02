package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alinz/script.go/v2/cmd/script/internal/plugins"
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

	buildDir, err := os.MkdirTemp("", "script-go-plugins-")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(buildDir)

	pluginPaths := make([]string, len(pluginSrcPaths))

	for i, pluginSrcPath := range pluginSrcPaths {
		// each plugin needs its own output file; sharing one path would make
		// every entry point at the last plugin built
		pluginPath := filepath.Join(buildDir, fmt.Sprintf("plugin-%d.so", i))
		inputDir := filepath.Join(workspace, pluginSrcPath)

		fmt.Printf("[ BUILD ]: %s\n", pluginSrcPath)

		cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", pluginPath, ".")
		cmd.Dir = inputDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			log.Fatalf("failed to build plugin %s: %v", pluginSrcPath, err)
		}

		pluginPaths[i] = pluginPath
	}

	if err := plugins.Run(workspace, pluginPaths...); err != nil {
		log.Fatal(err)
	}
}
