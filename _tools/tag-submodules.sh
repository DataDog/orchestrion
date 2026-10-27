#!/usr/bin/env bash
# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2026-present Datadog, Inc.

set -o errexit
set -o nounset
set -o pipefail

readonly REPOSITORY='DataDog/orchestrion'
readonly MODULE_PREFIX='github.com/DataDog/orchestrion/'
readonly VERSION_TAG_REGEX='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*))*))?(\+([0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*))?$'

fail() {
  printf '%s\n' "$*" >&2
  exit 1
}

validate_tag() {
  [[ "$1" =~ ${VERSION_TAG_REGEX} ]] || fail "Invalid release tag: $1"
}

# Called with the read-only workflow token, before obtaining the STS write token.
resolve_release() {
  local tag="$1" published
  validate_tag "${tag}"
  published=$(gh release view "${tag}" --repo "${REPOSITORY}" --json tagName,isDraft \
    --jq 'select(.isDraft == false) | .tagName') || return
  [[ "${published}" == "${tag}" ]] || fail "Release ${tag} is not published."
  # Explicitly propagate failures: command substitution does not inherit errexit.
  # target_commitish may be a moving branch; the published root tag is authoritative.
  git fetch --no-tags origin "refs/tags/${tag}" >&2 || return
  git rev-parse --verify 'FETCH_HEAD^{commit}'
}

# Peel annotated tags, but also accept existing lightweight tags at the same commit.
remote_commit() {
  local tag="$1" refs direct peeled
  refs=$(git ls-remote --tags origin "refs/tags/${tag}" "refs/tags/${tag}^{}") || return
  direct=$(awk -v ref="refs/tags/${tag}" '$2 == ref { print $1 }' <<<"${refs}")
  peeled=$(awk -v ref="refs/tags/${tag}^{}" '$2 == ref { print $1 }' <<<"${refs}")
  printf '%s\n' "${peeled:-${direct}}"
}

check_existing_tag() {
  local tag="$1" sha="$2" existing
  existing=$(remote_commit "${tag}") || return
  if [[ -n "${existing}" && "${existing}" != "${sha}" ]]; then
    fail "Tag ${tag} points to a different commit: ${existing}; expected ${sha}. Refusing to move it."
  fi
}

create_tag() {
  local tag="$1" sha="$2" existing annotation
  existing=$(remote_commit "${tag}") || return
  if [[ -n "${existing}" ]]; then
    [[ "${existing}" == "${sha}" ]] || fail "Tag ${tag} points to a different commit: ${existing}."
    printf 'Already correct: %s -> %s\n' "${tag}" "${sha}"
    return
  fi

  # Use GH_TOKEN explicitly through gh, never checkout's persisted GITHUB_TOKEN.
  annotation=$(gh api --method POST "repos/${REPOSITORY}/git/tags" \
    -f tag="${tag}" -f message="${tag}" -f object="${sha}" -f type=commit --jq '.sha')
  [[ "${annotation}" =~ ^[0-9a-f]{40}$ ]] || fail "Invalid tag object returned for ${tag}."
  if ! gh api --method POST "repos/${REPOSITORY}/git/refs" \
    -f ref="refs/tags/${tag}" -f sha="${annotation}" --silent; then
    printf 'Tag creation failed for %s; checking whether the ref was created.\n' "${tag}" >&2
  fi
  # Reconcile concurrent creation and responses lost after a successful write.
  existing=$(remote_commit "${tag}") || return
  [[ "${existing}" == "${sha}" ]] || fail "Failed to create ${tag} at ${sha}; remote commit is ${existing:-missing}."
  printf 'Verified: %s -> %s\n' "${tag}" "${sha}"
}

main() {
  if [[ "${1:-}" == '-h' || "${1:-}" == '--help' ]]; then
    printf 'Usage: %s [--] <published-release-tag> <release-commit-sha>\n' "$0"
    return
  fi
  if [[ "${1:-}" == '--' ]]; then
    shift
  fi
  [[ "$#" -eq 2 ]] || fail 'Expected a published release tag and its commit SHA.'
  local tag="$1" sha="$2" dependency root files gomod dir module suffix
  local -a tags=()
  for dependency in git gh awk; do
    command -v "${dependency}" >/dev/null || fail "Required tool not found: ${dependency}"
  done
  [[ -n "${GH_TOKEN:-}" ]] || fail 'GH_TOKEN is required for tag publication.'
  validate_tag "${tag}"
  [[ "${sha}" =~ ^[0-9a-f]{40}$ ]] || fail "Invalid release commit: ${sha}"
  root=$(remote_commit "${tag}") || return
  [[ "${root}" == "${sha}" ]] || fail "Root tag ${tag} does not resolve to ${sha}."
  git cat-file -e "${sha}^{commit}"

  # Inspect only tracked metadata at the release commit. Do not execute release code,
  # run Go, or inspect a newer checkout while holding a privileged token.
  files=$(git ls-tree -r --name-only "${sha}")
  while IFS= read -r gomod; do
    case "${gomod}" in
      _* | */_* | */testdata/*) continue ;;
      */go.mod) ;;
      *) continue ;;
    esac
    dir="${gomod%/go.mod}"
    module=$(git show "${sha}:${gomod}" | awk '$1 == "module" { gsub(/^"|"$/, "", $2); print $2 }')
    [[ "${module}" == "${MODULE_PREFIX}"* ]] || continue
    suffix="${module#"${MODULE_PREFIX}"}"
    [[ "${suffix}" == "${dir}" ]] || continue
    tags+=("${suffix}/${tag}")
  done <<<"${files}"

  # Detect all existing conflicts before creating any missing refs.
  for dependency in "${tags[@]}"; do
    check_existing_tag "${dependency}" "${sha}"
  done
  for dependency in "${tags[@]}"; do
    create_tag "${dependency}" "${sha}"
  done
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
