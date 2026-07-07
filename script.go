// Package script is the API surface for deployment scripts executed by the
// script.go GitHub Action. A script builds a Runner once and uses it to run
// local commands, run remote commands over SSH, copy files, and write env
// files on the remote host.
package script

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alinz/script.go/internal/expand"
	"github.com/alinz/script.go/pkg/ssh"
)

// Runner combines local and remote execution. Close it when the script is
// done to release the SSH connection.
type Runner interface {
	ssh.Runner

	// RunLocal executes cmds sequentially through `sh -c`, so shell quoting,
	// pipes, and redirection work. ${workspace} and ${ANY_ENV_VAR}
	// references are expanded before execution; unknown references are left
	// for the shell.
	RunLocal(workspace string, cmds ...string) error
}

type runner struct {
	ssh ssh.Runner
}

var _ Runner = (*runner)(nil)

func (r *runner) RunLocal(workspace string, cmds ...string) error {
	lookup := expand.OSLookup(map[string]string{"workspace": workspace})

	for _, cmd := range cmds {
		cmd = expand.Vars(cmd, lookup)

		fmt.Printf("[ LOCAL RUN ]: %s\n", cmd)

		local := exec.Command("sh", "-c", cmd)
		local.Stdout = os.Stdout
		local.Stderr = os.Stderr
		local.Stdin = os.Stdin

		if err := local.Run(); err != nil {
			return fmt.Errorf("local command %q failed: %w", cmd, err)
		}
	}

	return nil
}

func (r *runner) CreateEnvFile(path string, env map[string]string) error {
	return r.ssh.CreateEnvFile(path, env)
}

func (r *runner) RunRemote(cmds ...string) error {
	return r.ssh.RunRemote(cmds...)
}

func (r *runner) CopyFiles(permissions, remotePath, workspace string, filepaths ...string) error {
	return r.ssh.CopyFiles(permissions, remotePath, workspace, filepaths...)
}

func (r *runner) Close() error {
	return r.ssh.Close()
}

// Config describes how to reach the remote host.
type Config struct {
	// User is the SSH user. Required.
	User string
	// Host is the remote host. Required.
	Host string
	// Port is the SSH port. Defaults to 22.
	Port int
	// PrivateKeyEnv is the name of the environment variable holding the SSH
	// private key. Defaults to "SSH_PRIVATE_KEY".
	PrivateKeyEnv string
	// DefaultLocalPath is the key file used when PrivateKeyEnv is unset.
	// Defaults to "~/.ssh/id_rsa".
	DefaultLocalPath string
	// HostPublicKey optionally pins the remote host key, in authorized_keys
	// format (e.g. one line of `ssh-keyscan host` output). When empty, host
	// key verification is disabled.
	HostPublicKey string
	// Timeout bounds the connection attempt. Defaults to 30 seconds.
	Timeout time.Duration
}

// NewRunner validates config, connects to the remote host, and returns a
// Runner. Call Close on the Runner when the script is done.
func NewRunner(config *Config) (Runner, error) {
	if config == nil {
		return nil, fmt.Errorf("script: config is required")
	}
	if config.Host == "" {
		return nil, fmt.Errorf("script: Config.Host is required")
	}
	if config.User == "" {
		return nil, fmt.Errorf("script: Config.User is required")
	}

	privateKeyEnv := config.PrivateKeyEnv
	if privateKeyEnv == "" {
		privateKeyEnv = "SSH_PRIVATE_KEY"
	}

	defaultLocalPath := config.DefaultLocalPath
	if defaultLocalPath == "" {
		defaultLocalPath = "~/.ssh/id_rsa"
	}

	opts := []ssh.Option{
		ssh.WithAddr(config.Host, config.Port),
		ssh.WithUser(config.User),
		ssh.WithPrivateKey(privateKeyEnv, defaultLocalPath),
	}
	if config.HostPublicKey != "" {
		opts = append(opts, ssh.WithHostKey(config.HostPublicKey))
	}
	if config.Timeout > 0 {
		opts = append(opts, ssh.WithTimeout(config.Timeout))
	}

	sshRunner, err := ssh.Client(opts...)
	if err != nil {
		return nil, err
	}

	return &runner{
		ssh: sshRunner,
	}, nil
}
