package runner

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alinz/script.go/v2"
	"github.com/alinz/script.go/v2/internal/build"
	"github.com/alinz/script.go/v2/internal/plugins"
)

// Config holds SSH and execution configuration.
type Config struct {
	Host          string
	Port          int
	User          string
	PrivateKeyEnv string
	PrivateKeyPath string
	HostPublicKey string
	Timeout       time.Duration
}

// Run builds and executes the deployment scripts with the given configuration.
func Run(ctx context.Context, workspace string, scriptDirs []string, cfg Config) error {
	// Build scripts as plugins
	pluginPaths, err := build.Scripts(scriptDirs)
	if err != nil {
		return err
	}
	defer build.Cleanup(pluginPaths)

	// Set SSH configuration in environment for the script to use
	if cfg.PrivateKeyPath != "" {
		// Load key from file and set as environment variable
		keyBytes, err := os.ReadFile(os.ExpandEnv(cfg.PrivateKeyPath))
		if err != nil {
			return fmt.Errorf("failed to read SSH key file: %w", err)
		}
		if err := os.Setenv(cfg.PrivateKeyEnv, string(keyBytes)); err != nil {
			return fmt.Errorf("failed to set SSH key env var: %w", err)
		}
	}

	// Set other SSH config in environment for the script to use
	if err := os.Setenv("SCRIPT_SSH_HOST", cfg.Host); err != nil {
		return err
	}
	if err := os.Setenv("SCRIPT_SSH_PORT", fmt.Sprintf("%d", cfg.Port)); err != nil {
		return err
	}
	if err := os.Setenv("SCRIPT_SSH_USER", cfg.User); err != nil {
		return err
	}
	if cfg.HostPublicKey != "" {
		if err := os.Setenv("SCRIPT_SSH_HOST_KEY", cfg.HostPublicKey); err != nil {
			return err
		}
	}
	if cfg.Timeout > 0 {
		if err := os.Setenv("SCRIPT_SSH_TIMEOUT", cfg.Timeout.String()); err != nil {
			return err
		}
	}

	// Run the plugins
	if err := plugins.Run(workspace, pluginPaths...); err != nil {
		return err
	}

	return nil
}

// GetConfigFromEnv creates a Config from environment variables.
// This allows scripts to easily read the SSH configuration.
func GetConfigFromEnv() (*script.Config, error) {
	host := os.Getenv("SCRIPT_SSH_HOST")
	if host == "" {
		return nil, fmt.Errorf("SCRIPT_SSH_HOST not set")
	}

	user := os.Getenv("SCRIPT_SSH_USER")
	if user == "" {
		return nil, fmt.Errorf("SCRIPT_SSH_USER not set")
	}

	port := 22
	if p := os.Getenv("SCRIPT_SSH_PORT"); p != "" {
		if _, err := fmt.Sscanf(p, "%d", &port); err != nil {
			return nil, fmt.Errorf("invalid SCRIPT_SSH_PORT: %w", err)
		}
	}

	timeout := 30 * time.Second
	if t := os.Getenv("SCRIPT_SSH_TIMEOUT"); t != "" {
		var err error
		timeout, err = time.ParseDuration(t)
		if err != nil {
			return nil, fmt.Errorf("invalid SCRIPT_SSH_TIMEOUT: %w", err)
		}
	}

	return &script.Config{
		Host:          host,
		Port:          port,
		User:          user,
		PrivateKeyEnv: os.Getenv("SSH_PRIVATE_KEY_ENV"),
		HostPublicKey: os.Getenv("SCRIPT_SSH_HOST_KEY"),
		Timeout:       timeout,
	}, nil
}
