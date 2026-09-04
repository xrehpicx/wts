#!/usr/bin/env bash
# Called only after verification by the serialized release workflow.
set -euo pipefail

bump=${1:-patch}
revision=${2:-}
case "$bump" in
  patch|minor) ;;
  *) echo "Version bump must be patch or minor." >&2; exit 1 ;;
esac
if [ "${GITHUB_REF:-}" != "refs/heads/main" ]; then
  echo "The release workflow must run from main." >&2
  exit 1
fi
if [ -z "$revision" ] || [ "$revision" != "$(git rev-parse HEAD)" ]; then
  echo "Requested revision does not match the verified checkout." >&2
  exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "Working tree must be clean before tagging." >&2
  exit 1
fi
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT must be set}"

git fetch --tags origin main:refs/remotes/origin/main
if [ "$revision" != "$(git rev-parse refs/remotes/origin/main)" ]; then
  echo "Main changed after this release was requested. Run the release again." >&2
  exit 1
fi

# Ignore demo, prerelease and malformed tags. Version order is intentional: tag
# dates and proximity to HEAD do not reliably identify the latest release.
versions=$(git tag --sort=-version:refname | awk '/^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/')
latest=$(printf '%s\n' "$versions" | sed -n '1p')
previous=$(printf '%s\n' "$versions" | sed -n '2p')
new_tag=true
if [ -n "$latest" ]; then
  if ! git merge-base --is-ancestor "$latest" "$revision"; then
    echo "Latest release $latest is not an ancestor of the requested revision." >&2
    exit 1
  fi
  if [ "$(git rev-parse "$latest^{commit}")" = "$revision" ]; then
    new_tag=false
    tag=$latest
  fi
fi

if [ "$new_tag" = true ]; then
  previous=$latest
  IFS='.' read -r major minor patch <<< "${latest:-v0.0.0}"
  major=${major#v}
  case "$bump" in
    minor) minor=$((minor + 1)); patch=0 ;;
    patch) patch=$((patch + 1)) ;;
  esac
  tag="v${major}.${minor}.${patch}"
fi

# Listing releases distinguishes a missing release from an API/auth failure.
# A published release is left intact; a missing or draft release can be retried.
release_state=$(gh api --paginate 'repos/{owner}/{repo}/releases' --jq ".[] | select(.tag_name == \"$tag\") | .draft")
if [ "$release_state" = false ]; then
  if [ "$new_tag" = true ]; then
    echo "Release $tag exists without its expected tag; refusing to retag it." >&2
    exit 1
  fi
  echo "Release $tag is already published."
  released=false
elif [ -z "$release_state" ] || [ "$release_state" = true ]; then
  if [ "$new_tag" = true ]; then
    git tag -a "$tag" "$revision" -m "Release $tag"
    git push origin "refs/tags/$tag"
  fi
  echo "Publishing $tag at $revision."
  released=true
else
  echo "Unexpected release state for $tag: $release_state" >&2
  exit 1
fi

{
  echo "released=$released"
  echo "tag=$tag"
  echo "previous_tag=$previous"
  echo "revision=$revision"
} >> "$GITHUB_OUTPUT"
