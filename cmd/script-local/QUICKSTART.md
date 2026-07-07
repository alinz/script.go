# Quick Start: Local Testing

Get started testing your deployment scripts locally in 5 minutes.

## 1. Create a Test Deployment Script

Create a test script directory in your project:

```bash
mkdir -p scripts/deploy
cd scripts/deploy
go mod init github.com/myorg/myproject/scripts/deploy
go get github.com/alinz/script.go
cat > main.go << 'EOF'
package main

import (
	"fmt"
	"github.com/alinz/script.go"
)

var Runner = func(workspace string) error {
	runner, err := script.NewRunner(&script.Config{
		User: "deploy",
		Host: "example.com",
		// PrivateKeyEnv: "SSH_PRIVATE_KEY", // default
		// Port: 22,                          // default
	})
	if err != nil {
		return err
	}
	defer runner.Close()

	fmt.Println("Testing deployment script...")

	// Run a command on the remote host
	return runner.RunRemote(
		"echo 'Hello from deployment script'",
		"uname -a",
	)
}
EOF
```

## 2. Set Up Local Testing Configuration

```bash
# Copy the example env file
cp cmd/local/.env.example cmd/local/.env

# Edit it with your target host
vi cmd/local/.env
```

Example `.env`:
```bash
WORKSPACE=/path/to/project
SCRIPTS=scripts/deploy
DEPLOY_HOST=staging.example.com
DEPLOY_USER=deploy
SSH_KEY_PATH=~/.ssh/deploy_key
```

## 3. Test Your Script Locally

Using the helper script:
```bash
./cmd/local/run.sh
```

Or directly with flags:
```bash
go run ./cmd/local \
  -workspace . \
  -paths "scripts/deploy" \
  -host staging.example.com \
  -user deploy \
  -key ~/.ssh/deploy_key
```

## 4. Iterate and Improve

- Edit your script in `scripts/deploy/main.go`
- Test locally with `./cmd/local/run.sh`
- Once working, update your GitHub Action to reference the same script

## Next Steps

- Read the full [cmd/local/README.md](README.md) for advanced options
- Test with multiple scripts using comma-separated paths
- Pin the host key in production for better security
- See examples in [alinz/examples-script.go](https://github.com/alinz/examples-script.go)

## Troubleshooting

**SSH key not found:**
```bash
# Make sure your SSH key path is correct
ls -la ~/.ssh/deploy_key

# Or set the key via environment variable
export SSH_PRIVATE_KEY=$(cat ~/.ssh/deploy_key)
./cmd/local/run.sh
```

**Connection timeout:**
```bash
# Increase timeout for slow networks
go run ./cmd/local \
  -workspace . \
  -paths "scripts/deploy" \
  -host example.com \
  -user deploy \
  -timeout 2m
```

**Permission denied:**
```bash
# Verify the SSH key permissions
chmod 600 ~/.ssh/deploy_key

# Test SSH connection manually
ssh -i ~/.ssh/deploy_key deploy@example.com whoami
```

## Common Patterns

### Test with Staging vs Production

```bash
# Test with staging
export DEPLOY_HOST=staging.example.com
./cmd/local/run.sh

# Test with production (verify host key!)
export DEPLOY_HOST=prod.example.com
export HOST_KEY=$(ssh-keyscan prod.example.com | grep ssh-ed25519)
./cmd/local/run.sh
```

### Run Multiple Scripts

```bash
# In .env, set:
SCRIPTS=build,test,deploy

# Or inline:
go run ./cmd/local \
  -workspace . \
  -paths "build,test,deploy" \
  -host example.com \
  -user deploy
```

### Test in Docker (CI-like Environment)

```bash
# Build in a container matching your CI environment
docker run --rm \
  -v ~/.ssh:/home/builder/.ssh:ro \
  -v $(pwd):/project \
  golang:1.23 \
  bash -c 'cd /project && ./cmd/local/run.sh'
```

### Save and Load SSH Key from Secret

```bash
# Load from environment (e.g., from pass, 1password, AWS Secrets)
export SSH_PRIVATE_KEY=$(pass show deployment/ssh_key)
./cmd/local/run.sh
```
