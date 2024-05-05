package ssh

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Runner interface {
	CreateEnvFile(path string, env map[string]string) error
	RunRemote(cmds ...string) error
	CopyFiles(permissions, remotePath string, workspace string, filepaths ...string) error
}

type client struct {
	sshClient *ssh.Client
}

var _ Runner = (*client)(nil)

func (c *client) CreateEnvFile(path string, envMap map[string]string) error {
	var sb strings.Builder

	// convert all availables env into map
	// map[${key}] = value
	allEnvMap := make(map[string]string)
	allEnvs := os.Environ()
	for _, env := range allEnvs {
		parts := strings.Split(env, "=")
		allEnvMap[fmt.Sprintf("${%s}", parts[0])] = parts[1]
	}

	// then try to replace all variables starts with ${key} with corresponding value
	for k, v := range envMap {
		for key, value := range allEnvMap {
			v = strings.ReplaceAll(v, key, value)
		}

		envMap[k] = v
	}

	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}

	// Sort the keys to make sure the order is consistent
	sort.Strings(keys)

	// write to file
	for _, k := range keys {
		v := envMap[k]
		sb.WriteString(fmt.Sprintf("%s=%s", k, v))
		sb.WriteString("\n")
	}

	return c.RunRemote(fmt.Sprintf(`echo "%s" > %s`, sb.String(), path))
}

func (c *client) RunRemote(cmds ...string) error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	go func() {
		out, err := session.StderrPipe()
		if err != nil {
			fmt.Println(err)
			return
		}
		io.Copy(os.Stderr, out)
	}()

	time.Sleep(1 * time.Second)

	for _, cmd := range cmds {
		fmt.Printf("[ REMOTE RUN ]: %s\n", cmd)
	}

	err = session.Run(strings.Join(cmds, "\n"))
	if err != nil {
		time.Sleep(1 * time.Second)
		return err
	}

	return nil
}

func (c *client) CopyFiles(permissions, remotePath, workspace string, filepaths ...string) error {
	filepathMap := make(map[string]int64)

	for i, path := range filepaths {
		filepaths[i] = filepath.Join(workspace, path)
	}

	for _, filepath := range filepaths {
		fileInfo, err := os.Stat(filepath)
		if err != nil {
			return err
		}
		filepathMap[filepath] = fileInfo.Size()
	}

	err := c.RunRemote("mkdir -p " + remotePath)
	if err != nil {
		return err
	}

	for filepath, size := range filepathMap {
		err := func(filepath string) error {
			filename := path.Base(filepath)

			r, err := os.Open(filepath)
			if err != nil {
				return err
			}
			defer r.Close()

			fmt.Printf("[ COPY FILE ]: '%s' -> '%s': %d bytes\n", filepath, remotePath, size)

			return copyToRemote(c.sshClient, r, path.Join(remotePath, filename), permissions, size)
		}(filepath)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyToRemote(client *ssh.Client, r io.Reader, remotePath string, permissions string, size int64) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	filename := path.Base(remotePath)

	errChan := make(chan error, 1)

	go func() {
		w, err := session.StdinPipe()
		if err != nil {
			errChan <- err
			return
		}
		defer w.Close()

		_, err = fmt.Fprintln(w, "C"+permissions, size, filename)
		if err != nil {
			errChan <- err
			return
		}

		if err = checkResponse(stdout); err != nil {
			errChan <- err
			return
		}

		_, err = io.Copy(w, r)
		if err != nil {
			errChan <- err
			return
		}

		_, err = fmt.Fprint(w, "\x00")
		if err != nil {
			errChan <- err
			return
		}

		if err = checkResponse(stdout); err != nil {
			errChan <- err
			return
		}

		errChan <- nil
	}()

	err = session.Run(fmt.Sprintf("%s -qt %q", "/usr/bin/scp", remotePath))
	if err != nil {
		return err
	}

	return <-errChan
}

type clientOptions struct {
	addr       string
	user       string
	privateKey string
}

type clientOptionsFunc func(opt *clientOptions) error

func WithAddr(addr string, port int) clientOptionsFunc {
	return func(opt *clientOptions) error {
		if opt.addr != "" {
			return fmt.Errorf("address already set to %s", opt.addr)
		}
		opt.addr = fmt.Sprintf("%s:%d", addr, port)
		return nil
	}
}

func WithUser(user string) clientOptionsFunc {
	return func(opt *clientOptions) error {
		if opt.user != "" {
			return fmt.Errorf("user already set to %s", opt.user)
		}

		opt.user = user
		return nil
	}
}

func WithPrivateKey(envKey string, path string) clientOptionsFunc {
	return func(opt *clientOptions) error {
		err := WithPrivateKeyFromEnvKey(envKey)(opt)
		if err == nil {
			return nil
		}

		return WithPrivateKeyFromFile(path)(opt)
	}
}

func WithPrivateKeyFromEnvKey(envKey string) clientOptionsFunc {
	return func(o *clientOptions) error {
		if o.privateKey != "" {
			return fmt.Errorf("private key already set")
		}

		value := os.Getenv(envKey)
		if value == "" {
			return fmt.Errorf("env key '%s' is not set for ssh provate key", envKey)
		}

		o.privateKey = value

		return nil
	}
}

func WithPrivateKeyFromFile(filepath string) clientOptionsFunc {
	return func(o *clientOptions) error {
		if o.privateKey != "" {
			return fmt.Errorf("private key already set")
		}

		if strings.HasPrefix(filepath, "~") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			filepath = strings.Replace(filepath, "~", homeDir, 1)
		}

		content, err := os.ReadFile(filepath)
		if err != nil {
			return fmt.Errorf("failed to read ssh private key: %w", err)
		} else if len(content) == 0 {
			return fmt.Errorf("the content of private key file %s is empty", filepath)
		}

		o.privateKey = string(content)

		return nil
	}
}

func Client(optsFns ...clientOptionsFunc) (Runner, error) {
	opts := clientOptions{}

	for _, optsFn := range optsFns {
		if err := optsFn(&opts); err != nil {
			return nil, err
		}
	}

	signer, _ := ssh.ParsePrivateKey([]byte(opts.privateKey))
	clientConfig := &ssh.ClientConfig{
		User: opts.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, err := ssh.Dial("tcp", opts.addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return &client{sshClient}, nil
}
