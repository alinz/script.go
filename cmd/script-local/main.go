package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alinz/script.go/cmd/script-local/internal/runner"
)

func main() {
	log.SetFlags(0)

	fs := flag.NewFlagSet("script-local", flag.ExitOnError)

	workspace := fs.String("workspace", "", "Path to the workspace (required)")
	paths := fs.String("paths", "", "Comma-separated script paths relative to workspace (required)")
	sshHost := fs.String("host", "", "SSH host (required)")
	sshUser := fs.String("user", "", "SSH user (required)")
	sshPort := fs.Int("port", 22, "SSH port")
	sshKeyPath := fs.String("key", "", "Path to SSH private key (optional, defaults to ~/.ssh/id_rsa)")
	sshKeyEnv := fs.String("key-env", "SSH_PRIVATE_KEY", "Environment variable containing SSH private key")
	hostKey := fs.String("host-key", "", "Host public key for verification (optional, disables verification if empty)")
	timeout := fs.Duration("timeout", 30*time.Second, "SSH connection timeout")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [flags]

Run deployment scripts locally with SSH configuration.

Flags:
`, filepath.Base(os.Args[0]))
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Environment Variables:
  SSH_PRIVATE_KEY   If --key-env points to this (default), the SSH private key can be
                    set via this environment variable instead of a file path.

Examples:
  # Using SSH key file
  %s -workspace /path/to/project -paths "deploy" \
    -host example.com -user deploy -key ~/.ssh/deploy_key

  # Using SSH key from environment
  export SSH_PRIVATE_KEY=$(cat ~/.ssh/deploy_key)
  %s -workspace /path/to/project -paths "deploy" \
    -host example.com -user deploy

  # Multiple scripts (comma-separated)
  %s -workspace /path/to/project \
    -paths "build,deploy" \
    -host example.com -user deploy
`, filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	// Validate required flags
	if *workspace == "" {
		log.Fatal("error: -workspace is required")
	}
	if *paths == "" {
		log.Fatal("error: -paths is required")
	}
	if *sshHost == "" {
		log.Fatal("error: -host is required")
	}
	if *sshUser == "" {
		log.Fatal("error: -user is required")
	}

	// Normalize workspace path
	absWorkspace, err := filepath.Abs(*workspace)
	if err != nil {
		log.Fatalf("error: invalid workspace path: %v", err)
	}
	if _, err := os.Stat(absWorkspace); err != nil {
		log.Fatalf("error: workspace path does not exist: %v", err)
	}

	// Parse script paths
	var scriptPaths []string
	for _, p := range strings.Split(*paths, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			scriptPaths = append(scriptPaths, p)
		}
	}
	if len(scriptPaths) == 0 {
		log.Fatal("error: no script paths provided")
	}

	// Build script paths and validate they exist
	var scriptDirs []string
	for _, scriptPath := range scriptPaths {
		scriptDir := filepath.Join(absWorkspace, scriptPath)
		if _, err := os.Stat(scriptDir); err != nil {
			log.Fatalf("error: script path does not exist: %s (%v)", scriptPath, err)
		}
		scriptDirs = append(scriptDirs, scriptDir)
	}

	// Validate go.mod exists in each script directory
	for i, scriptDir := range scriptDirs {
		gomod := filepath.Join(scriptDir, "go.mod")
		if _, err := os.Stat(gomod); err != nil {
			log.Fatalf("error: script %s has no go.mod", scriptPaths[i])
		}
	}

	fmt.Printf("Workspace: %s\n", absWorkspace)
	fmt.Printf("Scripts: %s\n", strings.Join(scriptPaths, ", "))
	fmt.Printf("Host: %s:%d\n", *sshHost, *sshPort)
	fmt.Printf("User: %s\n", *sshUser)
	if *sshKeyPath != "" {
		fmt.Printf("Key: %s\n", *sshKeyPath)
	} else {
		fmt.Printf("Key: from %s environment variable\n", *sshKeyEnv)
	}
	fmt.Println()

	// Run the scripts in sequence by importing and executing them
	if err := runScripts(context.Background(), absWorkspace, scriptDirs, scriptConfig{
		host:          *sshHost,
		port:          *sshPort,
		user:          *sshUser,
		privateKeyEnv: *sshKeyEnv,
		keyPath:       *sshKeyPath,
		hostKey:       *hostKey,
		timeout:       *timeout,
	}); err != nil {
		log.Fatal(err)
	}
}

type scriptConfig struct {
	host          string
	port          int
	user          string
	privateKeyEnv string
	keyPath       string
	hostKey       string
	timeout       time.Duration
}

func runScripts(ctx context.Context, workspace string, scriptDirs []string, cfg scriptConfig) error {
	return runner.Run(ctx, workspace, scriptDirs, runner.Config{
		Host:           cfg.host,
		Port:           cfg.port,
		User:           cfg.user,
		PrivateKeyEnv:  cfg.privateKeyEnv,
		PrivateKeyPath: cfg.keyPath,
		HostPublicKey:  cfg.hostKey,
		Timeout:        cfg.timeout,
	})
}
