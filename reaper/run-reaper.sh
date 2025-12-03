#!/bin/bash
# Bovine Pages Server - Cache Reaper Runner Script
#
# This script makes it easier to run the reaper with environment variables.
# Copy this file to run-reaper.sh and configure your settings below.

# Redis Configuration
export REDIS_HOST="${REDIS_HOST:-localhost}"
export REDIS_PORT="${REDIS_PORT:-6379}"
export REDIS_PASSWORD="${REDIS_PASSWORD:-}"

# Forgejo Configuration
export FORGEJO_HOST="${FORGEJO_HOST:-https://git.example.com}"
export FORGEJO_TOKEN="${FORGEJO_TOKEN:-}"

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Run the reaper
cd "$SCRIPT_DIR"
python3 reaper.py "$@"
