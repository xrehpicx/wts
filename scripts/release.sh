#!/usr/bin/env bash
set -euo pipefail

bump=${1:-patch}
case "$bump" in
  patch|minor) ;;
  *) echo "Version bump must be patch or minor." >&2; exit 1 ;;
esac

branch=$(git branch --show-current)
if [ "$branch" != "main" ]; then
  echo "Releases must be created from main (current branch: ${branch:-detached})." >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "Working tree must be clean before releasing." >&2
  exit 1
fi

command -v gh >/dev/null || { echo "Install and authenticate the GitHub CLI (gh) before releasing." >&2; exit 1; }
git fetch --tags origin main:refs/remotes/origin/main
revision=$(git rev-parse HEAD)
if [ "$revision" != "$(git rev-parse refs/remotes/origin/main)" ]; then
  echo "Local main must match origin/main. Push or update main before releasing." >&2
  exit 1
fi

# The workflow rechecks this revision before tagging, so a concurrent push cannot
# silently change the code that was requested for release.
gh workflow run release.yml --ref main -f "bump=$bump" -f "revision=$revision"
echo "Release workflow requested for $revision ($bump)."
echo "Follow publication with: gh run list --workflow release.yml"
