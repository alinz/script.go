// Package ssh provides a small SSH client for running remote commands,
// copying files, and writing env files on a remote host.
package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/alinz/script.go/internal/expand"
)

// Runner is the remote half of the script API.
type Runner interface {
	// CreateEnvFile renders env as KEY=value lines (sorted by key, with
	// ${VAR} references resolved from the local environment) and uploads it
	// to path on the remote host with 0600 permissions.
	CreateEnvFile(path string, env map[string]string) error

	// RunRemote executes cmds sequentially in a single remote shell session,
	// streaming stdout and stderr locally. It returns the first error.
	RunRemote(cmds ...string) error

	// CopyFiles uploads each of filepaths (resolved relative to workspace)
	// into remotePath, creating remotePath if needed. permissions is an
	// octal mode such as "0644"; empty defaults to "0644". Each filepath
	// may contain ${workspace} and ${ENV_VAR} references, which are expanded
	// before glob pattern matching. Filepaths may also be glob patterns
	// (e.g., "*.txt", "bin/*", "${BUILD_DIR}/*.so").
	CopyFiles(permissions, remotePath string, workspace string, filepaths ...string) error

	// Close terminates the underlying SSH connection.
	Close() error
}

type client struct {
	sshClient *ssh.Client
}

var _ Runner = (*client)(nil)

// renderEnvFile produces the env file content: sorted KEY=value lines with
// ${VAR} references resolved via lookup.
func renderEnvFile(envMap map[string]string, lookup func(string) (string, bool)) string {
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(expand.Vars(envMap[k], lookup))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (c *client) CreateEnvFile(remotePath string, envMap map[string]string) error {
	content := renderEnvFile(envMap, expand.OSLookup(nil))

	if dir := path.Dir(remotePath); dir != "." && dir != "/" {
		if err := c.RunRemote("mkdir -p " + shellQuote(dir)); err != nil {
			return fmt.Errorf("failed to create directory for env file: %w", err)
		}
	}

	fmt.Printf("[ ENV FILE ]: writing %d entries -> '%s'\n", len(envMap), remotePath)

	// Uploading through scp instead of `echo "..." > path` keeps values with
	// quotes, $ or backticks intact and avoids shell injection.
	return copyToRemote(c.sshClient, strings.NewReader(content), remotePath, "0600", int64(len(content)))
}

func (c *client) RunRemote(cmds ...string) error {
	if len(cmds) == 0 {
		return nil
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create ssh session: %w", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	for _, cmd := range cmds {
		fmt.Printf("[ REMOTE RUN ]: %s\n", cmd)
	}

	if err := session.Run(strings.Join(cmds, "\n")); err != nil {
		return fmt.Errorf("remote command failed: %w", err)
	}

	return nil
}

func (c *client) CopyFiles(permissions, remotePath, workspace string, filepaths ...string) error {
	permissions, err := normalizePermissions(permissions)
	if err != nil {
		return err
	}

	if err := c.RunRemote("mkdir -p " + shellQuote(remotePath)); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", remotePath, err)
	}

	// Expand glob patterns and collect all files to copy.
	lookup := expand.OSLookup(map[string]string{"workspace": workspace})
	var filesToCopy []string
	for _, fp := range filepaths {
		// Expand environment variables in the filepath.
		expandedFp := expand.Vars(fp, lookup)
		pattern := filepath.Join(workspace, expandedFp)
		// Check if the pattern contains glob characters.
		if strings.ContainsAny(expandedFp, "*?[]") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("failed to expand glob %q: %w", expandedFp, err)
			}
			if len(matches) == 0 {
				return fmt.Errorf("glob pattern %q matched no files", expandedFp)
			}
			filesToCopy = append(filesToCopy, matches...)
		} else {
			filesToCopy = append(filesToCopy, pattern)
		}
	}

	for _, localPath := range filesToCopy {
		info, err := os.Stat(localPath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", localPath, err)
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory; CopyFiles only supports files", localPath)
		}

		if err := func() error {
			r, err := os.Open(localPath)
			if err != nil {
				return err
			}
			defer r.Close()

			fmt.Printf("[ COPY FILE ]: '%s' -> '%s': %d bytes\n", localPath, remotePath, info.Size())

			return copyToRemote(c.sshClient, r, path.Join(remotePath, filepath.Base(localPath)), permissions, info.Size())
		}(); err != nil {
			return fmt.Errorf("failed to copy %s: %w", localPath, err)
		}
	}

	return nil
}

func (c *client) Close() error {
	return c.sshClient.Close()
}

// normalizePermissions validates an octal file mode for the scp protocol,
// accepting "644" or "0644" style values. Empty defaults to "0644".
func normalizePermissions(permissions string) (string, error) {
	if permissions == "" {
		return "0644", nil
	}
	if len(permissions) == 3 {
		permissions = "0" + permissions
	}
	if len(permissions) != 4 {
		return "", fmt.Errorf("invalid permissions %q: expected an octal mode like 0644", permissions)
	}
	for _, ch := range permissions {
		if ch < '0' || ch > '7' {
			return "", fmt.Errorf("invalid permissions %q: expected an octal mode like 0644", permissions)
		}
	}
	return permissions, nil
}

// shellQuote wraps s in single quotes so it is safe to interpolate into a
// remote shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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

	if err := session.Run(fmt.Sprintf("/usr/bin/scp -qt %s", shellQuote(path.Dir(remotePath)))); err != nil {
		// prefer the protocol-level error from the writer goroutine when
		// one is available; it usually explains why scp exited non-zero.
		select {
		case werr := <-errChan:
			if werr != nil {
				return fmt.Errorf("scp upload of %s failed: %w", remotePath, werr)
			}
		default:
		}
		return fmt.Errorf("scp upload of %s failed: %w", remotePath, err)
	}

	if err := <-errChan; err != nil {
		return fmt.Errorf("scp upload of %s failed: %w", remotePath, err)
	}

	return nil
}

type clientOptions struct {
	addr            string
	user            string
	privateKey      string
	timeout         time.Duration
	hostKeyCallback ssh.HostKeyCallback
}

// Option configures the SSH client.
type Option func(opt *clientOptions) error

// WithAddr sets the host and port to connect to. A port of 0 or less
// defaults to 22.
func WithAddr(addr string, port int) Option {
	return func(opt *clientOptions) error {
		if opt.addr != "" {
			return fmt.Errorf("address already set to %s", opt.addr)
		}
		if port <= 0 {
			port = 22
		}
		opt.addr = fmt.Sprintf("%s:%d", addr, port)
		return nil
	}
}

// WithUser sets the SSH user.
func WithUser(user string) Option {
	return func(opt *clientOptions) error {
		if opt.user != "" {
			return fmt.Errorf("user already set to %s", opt.user)
		}

		opt.user = user
		return nil
	}
}

// WithTimeout bounds the TCP connection attempt. The default is 30 seconds.
func WithTimeout(d time.Duration) Option {
	return func(opt *clientOptions) error {
		opt.timeout = d
		return nil
	}
}

// WithHostKey pins the remote host key. publicKey is a single line in
// authorized_keys format (e.g. the output of `ssh-keyscan -t ed25519 host`,
// host prefix included or not). When this option is not supplied, host key
// verification is DISABLED, which is only acceptable for hosts you already
// trust the network path to.
func WithHostKey(publicKey string) Option {
	return func(opt *clientOptions) error {
		fields := strings.Fields(publicKey)
		// tolerate a leading hostname column, as produced by ssh-keyscan
		if len(fields) >= 3 || (len(fields) == 2 && !strings.HasPrefix(fields[0], "ssh-") && !strings.HasPrefix(fields[0], "ecdsa-")) {
			publicKey = strings.Join(fields[1:], " ")
		}

		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
		if err != nil {
			return fmt.Errorf("failed to parse host public key: %w", err)
		}

		opt.hostKeyCallback = ssh.FixedHostKey(key)
		return nil
	}
}

// WithPrivateKey loads the private key from the environment variable envKey,
// falling back to the file at path when the variable is unset or empty.
func WithPrivateKey(envKey string, path string) Option {
	return func(opt *clientOptions) error {
		envErr := WithPrivateKeyFromEnvKey(envKey)(opt)
		if envErr == nil {
			return nil
		}

		fileErr := WithPrivateKeyFromFile(path)(opt)
		if fileErr == nil {
			return nil
		}

		return fmt.Errorf("no ssh private key found: %v; %v", envErr, fileErr)
	}
}

// WithPrivateKeyFromEnvKey loads the private key from the environment
// variable envKey.
func WithPrivateKeyFromEnvKey(envKey string) Option {
	return func(o *clientOptions) error {
		if o.privateKey != "" {
			return errors.New("private key already set")
		}

		value := os.Getenv(envKey)
		if value == "" {
			return fmt.Errorf("env var '%s' is not set", envKey)
		}

		o.privateKey = value

		return nil
	}
}

// WithPrivateKeyFromFile loads the private key from filepath. A leading ~ is
// expanded to the user's home directory.
func WithPrivateKeyFromFile(filepath string) Option {
	return func(o *clientOptions) error {
		if o.privateKey != "" {
			return errors.New("private key already set")
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

// Client dials the remote host and returns a Runner. The caller owns the
// connection and should Close it when done.
func Client(optsFns ...Option) (Runner, error) {
	opts := clientOptions{
		timeout: 30 * time.Second,
	}

	for _, optsFn := range optsFns {
		if err := optsFn(&opts); err != nil {
			return nil, err
		}
	}

	switch {
	case opts.addr == "":
		return nil, errors.New("ssh: address is required (use WithAddr)")
	case opts.user == "":
		return nil, errors.New("ssh: user is required (use WithUser)")
	case opts.privateKey == "":
		return nil, errors.New("ssh: private key is required (use WithPrivateKey)")
	}

	signer, err := ssh.ParsePrivateKey([]byte(opts.privateKey))
	if err != nil {
		return nil, fmt.Errorf("ssh: failed to parse private key: %w", err)
	}

	hostKeyCallback := opts.hostKeyCallback
	if hostKeyCallback == nil {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	clientConfig := &ssh.ClientConfig{
		User: opts.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         opts.timeout,
	}

	sshClient, err := ssh.Dial("tcp", opts.addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh: failed to dial %s: %w", opts.addr, err)
	}

	return &client{sshClient}, nil
}
