# script.go

Write the rest of your deployment pipeline in Go instead of bash.

This GitHub Action compiles your Go code as a [Go plugin](https://pkg.go.dev/plugin) and executes it inside the workflow. Your script gets a small, batteries-included API for running local commands, running commands on a remote server over SSH, uploading files, and writing env files — no `sshpass`, `scp` flags, or heredoc quoting.

## Usage

```yml
name: Deploy
on:
  push:
    branches:
      - "main"

jobs:
  deploy:
    name: Deploy
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: "stable" # needs Go 1.25+

      - name: Run deployment scripts
        uses: alinz/script.go@v2
        with:
          workspace: ${{ github.workspace }} # <- this is important
          paths: .github/scripts/deploy # comma-separated if you have more than one
        env:
          SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
```

Each script lives in its own folder with its own `go.mod`, and must export a `Runner` symbol with the signature `func(workspace string) error` (either `func Runner(workspace string) error` or `var Runner = func(workspace string) error` works):

```go
// .github/scripts/deploy/main.go
package main

import (
	"github.com/alinz/script.go/v2"
)

var Runner = func(workspace string) error {
	runner, err := script.NewRunner(&script.Config{
		User: "deploy",
		Host: "example.com",
		// Port: 22,                        // default
		// PrivateKeyEnv: "SSH_PRIVATE_KEY", // default
	})
	if err != nil {
		return err
	}
	defer runner.Close()

	// build locally — full shell semantics, pipes and quoting included
	err = runner.RunLocal(workspace,
		"go build -o ${workspace}/bin/api ./cmd/api",
	)
	if err != nil {
		return err
	}

	// upload artifacts
	err = runner.CopyFiles("0755", "/opt/api", workspace, "bin/api")
	if err != nil {
		return err
	}

	// write an env file on the server (values may reference local env vars)
	err = runner.CreateEnvFile("/opt/api/.env", map[string]string{
		"DATABASE_URL": "${DATABASE_URL}",
		"PORT":         "8080",
	})
	if err != nil {
		return err
	}

	// restart the service
	return runner.RunRemote(
		"sudo systemctl restart api",
	)
}
```

> Because scripts are compiled with `-buildmode=plugin`, the action requires a Linux (or macOS) runner. Windows runners are not supported.

For a full example repository, see [alinz/examples-script.go](https://github.com/alinz/examples-script.go).

## API

Add the library to your script's module:

```
go get github.com/alinz/script.go/v2
```

### `script.NewRunner(config *script.Config) (script.Runner, error)`

Validates the config, connects to the remote host, and returns a `Runner`. Call `Close()` when done.

```go
type Config struct {
	User             string        // required
	Host             string        // required
	Port             int           // default: 22
	PrivateKeyEnv    string        // default: "SSH_PRIVATE_KEY"
	DefaultLocalPath string        // default: "~/.ssh/id_rsa" (fallback when the env var is unset)
	HostPublicKey    string        // optional: pin the host key (one line of `ssh-keyscan host` output)
	Timeout          time.Duration // default: 30s connection timeout
}
```

The private key is read from the environment variable named by `PrivateKeyEnv`; if that is unset, the file at `DefaultLocalPath` is used. When `HostPublicKey` is empty, host key verification is disabled — set it in production so a DNS or network hijack can't intercept your deployment.

### `Runner`

```go
type Runner interface {
	RunLocal(workspace string, cmds ...string) error
	RunRemote(cmds ...string) error
	CopyFiles(permissions, remotePath, workspace string, filepaths ...string) error
	CreateEnvFile(path string, env map[string]string) error
	Close() error
}
```

- **`RunLocal`** runs each command through `sh -c`, so quoting, pipes, and redirection all work. `${workspace}` and any `${ENV_VAR}` references are expanded first; unknown references are left for the shell.
- **`RunRemote`** runs commands in a single SSH session, streaming stdout/stderr into the workflow log.
- **`CopyFiles`** uploads files (paths relative to `workspace`) into `remotePath`, creating the directory if needed. `permissions` is an octal mode like `"0644"` (`""` defaults to `0644`).
- **`CreateEnvFile`** renders the map as sorted `KEY=value` lines and uploads it with `0600` permissions. Values may reference local env vars with `${NAME}`. The file is transferred over SCP, so values containing quotes, `$`, or backticks arrive intact.
- **`Close`** releases the SSH connection.

The lower-level SSH client is also available as `github.com/alinz/script.go/v2/pkg/ssh` with a functional-options constructor (`ssh.Client(ssh.WithAddr(...), ssh.WithUser(...), ssh.WithPrivateKey(...), ssh.WithHostKey(...), ssh.WithTimeout(...))`).

## Upgrading from v1 to v2

v2 is a cleanup and robustness release. The plugin contract (`var Runner = func(workspace string) error`) and the action inputs (`workspace`, `paths`) are **unchanged** — existing workflows keep working after switching the action ref to `@v2`. The library API has a few breaking changes:

**1. New import path.** The module moved to `/v2`:

```diff
-import "github.com/alinz/script.go"
+import "github.com/alinz/script.go/v2"
```

Then in your script's folder run `go get github.com/alinz/script.go/v2` and `go mod tidy`.

**2. `Runner` gained a `Close()` method.** Add `defer runner.Close()` after `NewRunner`. (If you implemented the interface yourself, you must add the method.)

**3. `RunLocal` now runs commands through `sh -c`.** Previously commands were naively split on spaces, so quoting and pipes didn't work. Now they do. Plain commands like `go build -o bin/api ./cmd/api` behave exactly as before; commands that relied on literal `"`/`|`/`>` characters being passed as arguments must now be shell-quoted.

**4. `Config` has defaults and validation.** `Port: 0` now means 22, `PrivateKeyEnv: ""` means `SSH_PRIVATE_KEY`, and `DefaultLocalPath: ""` means `~/.ssh/id_rsa`. `NewRunner` returns an error immediately when `Host` or `User` is missing, or when the private key can't be found or parsed (v1 deferred these to a confusing dial failure).

**5. `CreateEnvFile` uploads over SCP instead of `echo "..." > file`.** Values containing quotes, `$`, or backticks are no longer mangled by the remote shell, and the file is written with `0600` instead of the default umask. If you relied on the remote shell expanding variables inside values, reference local env vars with `${NAME}` instead — they are expanded before upload.

**6. `pkg/ssh` options are now the exported `ssh.Option` type** (previously an unexported type), and `ssh.Client` validates that address, user, and key are set.

Bug fixes you get for free:

- Running **multiple scripts** in one action invocation now works — v1 compiled every plugin to the same temp file, so only the last script actually ran (N times).
- Environment values containing `=` (base64 secrets, connection strings) are no longer truncated during `${VAR}` expansion.
- Files in `CopyFiles` upload in the order given instead of random map order, and the input slice is no longer mutated.
- Removed the arbitrary `time.Sleep` calls around remote execution; remote stdout is now streamed to the log too (v1 only streamed stderr).
- The `Runner` symbol may now be declared as either `func Runner(...)` or `var Runner = func(...)` — v1 only accepted the former and failed with a confusing type error on the latter.

## Development

```
go build ./...
go test ./...
```

## License

[MIT](LICENSE)
