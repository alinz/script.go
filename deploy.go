package script

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alinz/script.go/pkg/ssh"
)

type Runner interface {
	ssh.Runner
	RunLocal(workspace string, cmds ...string) error
}

type runner struct {
	ssh ssh.Runner
}

var _ Runner = (*runner)(nil)

func (r *runner) RunLocal(workspace string, cmds ...string) error {
	// convert all availables env into map
	// map[${key}] = value
	allEnvMap := make(map[string]string)
	allEnvs := os.Environ()
	for _, env := range allEnvs {
		parts := strings.Split(env, "=")
		allEnvMap[fmt.Sprintf("${%s}", parts[0])] = parts[1]
	}

	for _, cmd := range cmds {

		// replace ${workspace} with the value
		cmd = strings.ReplaceAll(cmd, "${workspace}", workspace)
		for key, value := range allEnvMap {
			cmd = strings.ReplaceAll(cmd, key, value)
		}

		fmt.Printf("[ LOCAL RUN ]: %s\n", cmd)

		segments := strings.Split(cmd, " ")
		execPath, err := exec.LookPath(segments[0])
		if err != nil {
			return err
		}

		local := exec.Cmd{
			Path:   execPath,
			Args:   segments,
			Stderr: os.Stderr,
			Stdout: os.Stdout,
			Stdin:  os.Stdin,
		}

		err = local.Run()
		if err != nil {
			return err
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

type Config struct {
	User             string
	Host             string
	Port             int
	PrivateKeyEnv    string
	DefaultLocalPath string
}

func NewRunner(config *Config) (Runner, error) {
	ssh, err := ssh.Client(
		ssh.WithAddr(config.Host, config.Port),
		ssh.WithUser(config.User),
		ssh.WithPrivateKey(config.PrivateKeyEnv, config.DefaultLocalPath),
	)
	if err != nil {
		return nil, err
	}

	return &runner{
		ssh: ssh,
	}, nil
}
