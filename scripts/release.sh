#!/usr/bin/env bash
set -euo pipefail

# Get latest tag
latest=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
commits_since=$(git rev-list "${latest}..HEAD" --count 2>/dev/null || echo "0")

if [ "$commits_since" = "0" ]; then
  echo "No changes since ${latest} — nothing to release."
  exit 0
fi

# Parse semver and bump patch
IFS='.' read -r major minor patch <<< "${latest#v}"
patch=$((patch + 1))
next="v${major}.${minor}.${patch}"

# Update version in main.go
sed -i '' "s/version = \".*\"/version = \"${major}.${minor}.${patch}\"/" main.go

# Commit, tag, push
git add main.go
git commit -m "release: ${next}"
git tag "${next}"
git push && git push origin "${next}"

echo ""
echo "Released ${next} (${commits_since} commits since ${latest})"
echo "Install:  go install github.com/xrehpicx/wts@latest"
