#!/usr/bin/env bash
# Computes the next semantic version from the commits since the latest final
# v* tag (release candidates like v1.2.0-rc.1 are ignored as a base).
#   - breaking change ("type!:" subject or "BREAKING CHANGE" in body) -> major
#   - "feat:" / "feat(scope):"                                         -> minor
#   - anything else                                                    -> patch
#
# Usage: next-version.sh [release|rc]
#   release (default)  vX.Y.Z
#   rc                 vX.Y.Z-rc.N, N counts up per base version
#
# Prints "version=...", "release=true|false" and "prerelease=true|false"
# (GITHUB_OUTPUT format).
set -euo pipefail

mode=${1:-release}
case "$mode" in release | rc) ;; *) echo "unknown mode: $mode" >&2; exit 2 ;; esac

emit() {
  local version=$1 release=$2
  if [ "$release" = true ] && [ "$mode" = rc ]; then
    local n
    n=$(git tag --list "${version}-rc.*" | sed 's/.*-rc\.//' | sort -n | tail -n1)
    version="${version}-rc.$(( ${n:-0} + 1 ))"
  fi
  echo "version=$version"
  echo "release=$release"
  echo "prerelease=$([ "$mode" = rc ] && echo true || echo false)"
}

last=$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' --exclude '*-*' 2>/dev/null || true)

if [ -z "$last" ]; then
  emit v0.1.0 true
  exit 0
fi

if [ -z "$(git rev-list "$last..HEAD" | head -n1)" ]; then
  emit "$last" false
  exit 0
fi

subjects=$(git log --format=%s "$last..HEAD")
bodies=$(git log --format=%b "$last..HEAD")

IFS=. read -r major minor patch <<<"${last#v}"

if grep -qE '^[a-zA-Z]+(\([^)]*\))?!:' <<<"$subjects" || grep -q '^BREAKING[ -]CHANGE' <<<"$bodies"; then
  major=$((major + 1)); minor=0; patch=0
elif grep -qE '^feat(\([^)]*\))?:' <<<"$subjects"; then
  minor=$((minor + 1)); patch=0
else
  patch=$((patch + 1))
fi

emit "v$major.$minor.$patch" true
