#!/usr/bin/env bash
set -euo pipefail

name="${1:-demo-script}"
interval="${2:-3}"

cleanup() {
  echo "[$name] received stop signal at $(date '+%Y-%m-%d %H:%M:%S')"
  exit 0
}

trap cleanup INT TERM

count=0
echo "[$name] started in $(pwd) with interval=${interval}s"
while true; do
  count=$((count + 1))
  echo "[$name] tick=${count} time=$(date '+%H:%M:%S')"
  sleep "$interval"
done
