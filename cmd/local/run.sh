#!/bin/bash
# Helper script to run script-local with environment variables from .env file

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source .env if it exists
if [ -f "$SCRIPT_DIR/.env" ]; then
	echo "Loading configuration from .env..."
	set -a
	source "$SCRIPT_DIR/.env"
	set +a
else
	echo "Error: .env file not found"
	echo "Please copy .env.example to .env and fill in your values:"
	echo "  cp $SCRIPT_DIR/.env.example $SCRIPT_DIR/.env"
	echo "  vi $SCRIPT_DIR/.env"
	exit 1
fi

# Validate required variables
for var in WORKSPACE SCRIPTS DEPLOY_HOST DEPLOY_USER; do
	if [ -z "${!var}" ]; then
		echo "Error: $var is not set in .env"
		exit 1
	fi
done

# Build the command
CMD="go run $SCRIPT_DIR"
CMD="$CMD -workspace $WORKSPACE"
CMD="$CMD -paths $SCRIPTS"
CMD="$CMD -host $DEPLOY_HOST"
CMD="$CMD -user $DEPLOY_USER"

# Optional flags
if [ -n "$DEPLOY_PORT" ]; then
	CMD="$CMD -port $DEPLOY_PORT"
fi

if [ -n "$SSH_KEY_PATH" ]; then
	CMD="$CMD -key $SSH_KEY_PATH"
fi

if [ -n "$SSH_KEY_ENV" ]; then
	CMD="$CMD -key-env $SSH_KEY_ENV"
fi

if [ -n "$HOST_KEY" ]; then
	CMD="$CMD -host-key '$HOST_KEY'"
fi

if [ -n "$SSH_TIMEOUT" ]; then
	CMD="$CMD -timeout $SSH_TIMEOUT"
fi

echo "Running: $CMD"
echo ""

# Execute the command
eval "$CMD"
