# script-local

A CLI tool for locally testing deployment scripts before running them in GitHub Actions.

## Installation

The tool is automatically built when you run `go build ./cmd/local` or `go run ./cmd/local`.

## Usage

### Basic Usage

Test a deployment script locally with SSH configuration:

```bash
go run ./cmd/local \
  -workspace /path/to/workspace \
  -paths "deploy" \
  -host example.com \
  -user deploy \
  -key ~/.ssh/deploy_key
```

### Multiple Scripts

You can run multiple scripts in sequence using comma-separated paths:

```bash
go run ./cmd/local \
  -workspace /path/to/workspace \
  -paths "build,deploy" \
  -host example.com \
  -user deploy \
  -key ~/.ssh/deploy_key
```

### Using Environment Variables for SSH Key

Instead of providing a path to the key file, you can set the SSH private key via an environment variable:

```bash
export SSH_PRIVATE_KEY=$(cat ~/.ssh/deploy_key)
go run ./cmd/local \
  -workspace /path/to/workspace \
  -paths "deploy" \
  -host example.com \
  -user deploy
```

This is useful in CI/CD environments where you might have the key in a secret.

### Advanced Options

```bash
go run ./cmd/local \
  -workspace /path/to/workspace \
  -paths "deploy" \
  -host example.com \
  -user deploy \
  -key ~/.ssh/deploy_key \
  -port 2222 \
  -host-key "ssh-ed25519 AAAA..." \
  -timeout 60s
```

All flags are optional except `-workspace`, `-paths`, `-host`, and `-user`.

## Flags

- `-workspace` (required): Path to the workspace root
- `-paths` (required): Comma-separated paths to script folders (relative to workspace)
- `-host` (required): SSH host to deploy to
- `-user` (required): SSH user
- `-key`: Path to SSH private key (defaults to `$SSH_PRIVATE_KEY` env var, then `~/.ssh/id_rsa`)
- `-key-env`: Environment variable containing SSH private key (default: `SSH_PRIVATE_KEY`)
- `-port`: SSH port (default: 22)
- `-host-key`: Host public key for verification (optional, disables verification if empty)
- `-timeout`: Connection timeout (default: 30s)

## Script Requirements

Each script directory must:
1. Have its own `go.mod` file
2. Export a `Runner` symbol with signature `func(workspace string) error`

Example script:

```go
// deploy/main.go
package main

import "github.com/alinz/script.go"

var Runner = func(workspace string) error {
	runner, err := script.NewRunner(&script.Config{
		User: "deploy",
		Host: "example.com",
	})
	if err != nil {
		return err
	}
	defer runner.Close()

	return runner.RunRemote(
		"cd /opt/app && git pull && go build -o api ./cmd/api",
	)
}
```

The Runner function receives the workspace path and can use the `script.Runner` API to:
- **RunLocal**: Execute commands locally with full shell semantics
- **RunRemote**: Execute commands on the remote server
- **CopyFiles**: Upload files to the remote server
- **CreateEnvFile**: Create an environment file on the remote server

See the main [README](../../../README.md) for full API documentation.

## Examples

### Example: Test with Different SSH Keys

Test your deploy script with different SSH keys:

```bash
# Test with a specific key
go run ./cmd/local \
  -workspace . \
  -paths "deploy" \
  -host staging.example.com \
  -user deploy \
  -key ~/.ssh/staging_key

# Test with production credentials
go run ./cmd/local \
  -workspace . \
  -paths "deploy" \
  -host prod.example.com \
  -user deploy \
  -key ~/.ssh/prod_key \
  -host-key "ssh-ed25519 AAAA..." # Pin the host key in production
```

### Example: Test with Timeout

For slow networks or connections, increase the timeout:

```bash
go run ./cmd/local \
  -workspace . \
  -paths "deploy" \
  -host example.com \
  -user deploy \
  -timeout 2m
```

### Example: CI-like Testing

Simulate a CI environment by using environment variables:

```bash
export SSH_PRIVATE_KEY=$(cat /secure/deploy_key)
export DEPLOY_HOST=${DEPLOY_HOST:-example.com}
export DEPLOY_USER=${DEPLOY_USER:-deploy}

go run ./cmd/local \
  -workspace . \
  -paths "deploy" \
  -host "$DEPLOY_HOST" \
  -user "$DEPLOY_USER"
```

## Relationship to GitHub Actions

This tool builds and runs your scripts identically to the GitHub Action:
- Scripts are compiled as Go plugins
- The `Runner` symbol is executed with the workspace path
- SSH configuration is managed the same way

The main difference is that you can:
- **Test locally** without committing and pushing
- **Iterate faster** during development
- **Validate credentials** before running in CI
- **Debug scripts** with local logging and output

Your scripts will work identically in both environments.
