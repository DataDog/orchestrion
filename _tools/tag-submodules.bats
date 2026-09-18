#!/usr/bin/env bats
# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2026-present Datadog, Inc.

# Exercise release tagging against disposable local repositories, never GitHub.
setup() {
  bats_require_minimum_version 1.5.0
  export SCRIPT="${BATS_TEST_DIRNAME}/tag-submodules.sh"
  export TAG='v1.13.1'
  export TEST_REMOTE="${BATS_TEST_TMPDIR}/remote.git"
  export TEST_API_LOG="${BATS_TEST_TMPDIR}/api.log"
  export TEST_READ_COUNT="${BATS_TEST_TMPDIR}/reads"
  export TEST_REAL_GIT
  TEST_REAL_GIT=$(type -P git)
  export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
  export GIT_AUTHOR_NAME='Release Test' GIT_AUTHOR_EMAIL='test@example.com'
  export GIT_COMMITTER_NAME='Release Test' GIT_COMMITTER_EMAIL='test@example.com'
  export GH_TOKEN='local-test-only'
  unset TEST_REF_OUTCOME TEST_RELEASE_STATE TEST_FAIL_FETCH TEST_FAIL_PREFLIGHT

  mkdir "${BATS_TEST_TMPDIR}/repo"
  cd "${BATS_TEST_TMPDIR}/repo" || return
  "${TEST_REAL_GIT}" init --bare --template= "${TEST_REMOTE}"
  "${TEST_REAL_GIT}" init --template= -b main
  add_module go.mod github.com/DataDog/orchestrion
  add_module instrument/go.mod github.com/DataDog/orchestrion/instrument
  add_module extra/go.mod '"github.com/DataDog/orchestrion/extra"'
  add_module samples/go.mod github.com/DataDog/orchestrion/_samples
  add_module _tools/go.mod github.com/DataDog/orchestrion/_tools
  add_module internal/testdata/fixture/go.mod github.com/DataDog/orchestrion/internal/testdata/fixture
  add_module thirdparty/go.mod example.com/thirdparty
  "${TEST_REAL_GIT}" add .
  "${TEST_REAL_GIT}" -c commit.gpgsign=false commit -m 'release fixture'
  export SHA
  SHA=$("${TEST_REAL_GIT}" rev-parse HEAD)
  "${TEST_REAL_GIT}" tag "${TAG}"
  "${TEST_REAL_GIT}" remote add origin "${TEST_REMOTE}"
  "${TEST_REAL_GIT}" push origin main "refs/tags/${TAG}"

  # Exported Bash functions also intercept calls in the helper's child Bash process.
  # gh never falls through to a real executable; git uses only local repositories.
  export -f gh git
}

add_module() {
  mkdir -p "$(dirname "$1")"
  printf 'module %s\n\ngo 1.25.0\n' "$2" >"$1"
}

git() {
  if [[ "$1" == fetch && -n "${TEST_FAIL_FETCH:-}" ]]; then
    printf 'fetch failed\n' >&2
    return 1
  fi
  if [[ "$1" == ls-remote && -n "${TEST_FAIL_PREFLIGHT:-}" ]]; then
    local count=0
    if [[ -f "${TEST_READ_COUNT}" ]]; then
      read -r count <"${TEST_READ_COUNT}"
    fi
    count=$((count + 1))
    printf '%s\n' "${count}" >"${TEST_READ_COUNT}"
    if ((count > 1)); then
      printf 'remote read failed\n' >&2
      return 1
    fi
  fi
  "${TEST_REAL_GIT}" "$@"
}

gh() {
  if [[ "$1" == release && "$2" == view ]]; then
    case "${TEST_RELEASE_STATE:-published}" in
      missing) return 1 ;;
      draft) return 0 ;;
      *)
        printf '%s\n' "$3"
        return 0
        ;;
    esac
  fi
  if [[ "$1" != api || "$2" != --method || "$3" != POST ]]; then
    printf 'Unexpected gh operation: %s\n' "$*" >&2
    return 1
  fi
  printf '%s\n' "$*" >>"${TEST_API_LOG}"
  local endpoint="$4" tag='' message='' object='' ref='' sha=''
  shift 4
  while (($# > 0)); do
    case "$1" in
      -f)
        case "$2" in
          tag=*) tag="${2#*=}" ;;
          message=*) message="${2#*=}" ;;
          object=*) object="${2#*=}" ;;
          ref=*) ref="${2#*=}" ;;
          sha=*) sha="${2#*=}" ;;
          type=commit) ;;
          *) return 1 ;;
        esac
        shift 2
        ;;
      --jq)
        [[ "$2" == .sha ]] || return 1
        shift 2
        ;;
      --silent) shift ;;
      *) return 1 ;;
    esac
  done
  case "${endpoint}" in
    repos/DataDog/orchestrion/git/tags)
      "${TEST_REAL_GIT}" --git-dir "${TEST_REMOTE}" mktag <<EOF
object ${object}
type commit
tag ${tag}
tagger Release Test <test@example.com> 1700000000 +0000

${message}
EOF
      ;;
    repos/DataDog/orchestrion/git/refs)
      if [[ "${TEST_REF_OUTCOME:-}" == reject ]]; then
        printf 'GH013: Cannot create ref due to creations being restricted\n' >&2
        return 1
      fi
      "${TEST_REAL_GIT}" --git-dir "${TEST_REMOTE}" update-ref "${ref}" "${sha}" \
        0000000000000000000000000000000000000000 || return
      if [[ "${TEST_REF_OUTCOME:-}" == lost-response ]]; then
        printf 'connection lost after ref creation\n' >&2
        return 1
      fi
      ;;
    *)
      printf 'Unexpected GitHub endpoint: %s\n' "${endpoint}" >&2
      return 1
      ;;
  esac
}

remote_ref() {
  "${TEST_REAL_GIT}" --git-dir "${TEST_REMOTE}" rev-parse --verify "refs/tags/$1"
}

publish_existing() {
  local tag="$1"
  shift
  git -c tag.gpgsign=false tag "$@" "${tag}"
  git push origin "refs/tags/${tag}"
}

resolve_release() {
  # Match the workflow's command substitution, where errexit is not inherited.
  bash -c 'source "$1"; sha=$(resolve_release "$2"); printf "%s\n" "$sha"' test "${SCRIPT}" "${TAG}"
}

@test "creates annotated tags only for matching modules" {
  run -0 bash "${SCRIPT}" "${TAG}" "${SHA}"
  local module tags
  for module in instrument extra; do
    [[ "$(remote_ref "${module}/${TAG}^{commit}")" == "${SHA}" ]]
    [[ "$(remote_ref "${module}/${TAG}")" != "${SHA}" ]]
  done
  tags=$(git ls-remote --tags origin)
  [[ "${tags}" != *samples/* && "${tags}" != *_tools/* ]]
  [[ "${tags}" != *testdata/* && "${tags}" != *thirdparty/* ]]
}

@test "retry preserves existing tag objects without API writes" {
  run -0 bash "${SCRIPT}" "${TAG}" "${SHA}"
  local before
  before=$(git ls-remote --tags origin)
  cp "${TEST_API_LOG}" "${BATS_TEST_TMPDIR}/before.log"
  run -0 bash "${SCRIPT}" "${TAG}" "${SHA}"
  [[ "$(git ls-remote --tags origin)" == "${before}" ]]
  cmp "${TEST_API_LOG}" "${BATS_TEST_TMPDIR}/before.log"
}

@test "partial release preserves an existing lightweight tag" {
  publish_existing "instrument/${TAG}"
  run -0 bash "${SCRIPT}" "${TAG}" "${SHA}"
  [[ "$(remote_ref "instrument/${TAG}")" == "${SHA}" ]]
  [[ "$(remote_ref "extra/${TAG}^{commit}")" == "${SHA}" ]]
}

@test "conflicts fail before any missing tags are created" {
  git -c commit.gpgsign=false commit --allow-empty -m 'different commit'
  publish_existing "instrument/${TAG}" -a -m 'existing release'
  run ! bash "${SCRIPT}" "${TAG}" "${SHA}"
  [[ "${output}" == *'different commit'* ]]
  run ! remote_ref "extra/${TAG}"
  [[ ! -e "${TEST_API_LOG}" ]]
}

@test "module discovery uses the release tree instead of the current checkout" {
  add_module later/go.mod github.com/DataDog/orchestrion/later
  git add .
  git -c commit.gpgsign=false commit -m 'unreleased module'
  run -0 bash "${SCRIPT}" "${TAG}" "${SHA}"
  run ! remote_ref "later/${TAG}"
}

@test "root tag must match the expected commit" {
  git -c commit.gpgsign=false commit --allow-empty -m later
  run ! bash "${SCRIPT}" "${TAG}" "$(git rev-parse HEAD)"
  [[ ! -e "${TEST_API_LOG}" ]]
}

@test "invalid or missing root tags cannot create tags" {
  local tag
  for tag in main instrument/v1.13.1 'v1.2.3;echo' v01.2.3 v9.9.9; do
    run ! bash "${SCRIPT}" "${tag}" "${SHA}"
  done
  [[ ! -e "${TEST_API_LOG}" ]]
}

@test "a lost success response is reconciled" {
  run -0 env TEST_REF_OUTCOME=lost-response bash "${SCRIPT}" "${TAG}" "${SHA}"
  [[ "$(remote_ref "instrument/${TAG}^{commit}")" == "${SHA}" ]]
}

@test "permission failures are not treated as success" {
  run ! env TEST_REF_OUTCOME=reject bash "${SCRIPT}" "${TAG}" "${SHA}"
  [[ "${output}" == *GH013* ]]
  run ! remote_ref "extra/${TAG}"
}

@test "published release resolves to its root tag" {
  run -0 --separate-stderr resolve_release
  [[ "${output}" == "${SHA}" ]]
}

@test "failed preflight reads cannot create tags" {
  export TEST_FAIL_PREFLIGHT=1
  run ! bash "${SCRIPT}" "${TAG}" "${SHA}"
  [[ ! -e "${TEST_API_LOG}" ]]
}

@test "failed fetch cannot reuse a stale FETCH_HEAD" {
  git fetch origin "refs/tags/${TAG}"
  export TEST_FAIL_FETCH=1
  run ! resolve_release
  [[ ! -e "${TEST_API_LOG}" ]]
}

@test "draft or missing releases cannot be repaired" {
  export TEST_RELEASE_STATE
  for TEST_RELEASE_STATE in draft missing; do
    run ! resolve_release
  done
  [[ ! -e "${TEST_API_LOG}" ]]
}

policy_matches() {
  local policy="$1" event="$2" ref="$3" repository="${4:-DataDog/orchestrion}" workflow="${5:-release.yml}"
  local path="${BATS_TEST_DIRNAME}/../.github/chainguard/self.github.release.${policy}.sts.yaml"
  local subject="repo:${repository}:ref:${ref}" pattern claim value permissions
  # Policies use only plain scalar values; yamlfmt checks YAML syntax separately.
  [[ "$(awk '$1 == "issuer:" { print $2 }' "${path}")" == https://token.actions.githubusercontent.com ]] || return 1
  permissions=$(awk '/^permissions:$/ { permissions=1; next } permissions && NF { print }' "${path}")
  [[ "${permissions}" == '  contents: write' ]] || return 1
  if grep -q '^  job_workflow_ref:' "${path}"; then
    return 1
  fi
  value=$(awk '$1 == "subject:" { print $2 }' "${path}")
  [[ -z "${value}" || "${value}" == "${subject}" ]] || return 1
  pattern=$(awk '$1 == "subject_pattern:" { print $2 }' "${path}")
  [[ -z "${pattern}" || "${subject}" =~ ${pattern} ]] || return 1
  for claim in event_name ref repository workflow_ref; do
    pattern=$(awk -v key="${claim}:" '$1 == key { print $2 }' "${path}")
    case "${claim}" in
      event_name) value="${event}" ;;
      ref) value="${ref}" ;;
      repository) value="${repository}" ;;
      workflow_ref) value="${repository}/.github/workflows/${workflow}@${ref}" ;;
    esac
    [[ -n "${pattern}" && "${value}" =~ ${pattern} ]] || return 1
  done
}

@test "published policy allows only the release workflow on version tags" {
  run -0 policy_matches published release refs/tags/v1.13.1
  run -0 policy_matches published release refs/tags/v1.14.0-rc.1
  run ! policy_matches published pull_request refs/pull/1/merge
  run ! policy_matches published workflow_dispatch refs/tags/v1.13.1
  run ! policy_matches published release refs/heads/main
  run ! policy_matches published release refs/tags/instrument/v1.13.1
  run ! policy_matches published release refs/tags/v1.13.1 fork/orchestrion
  run ! policy_matches published release refs/tags/v1.13.1 DataDog/orchestrion other.yml
}

@test "manual policy allows only dispatch of the release workflow on main" {
  run -0 policy_matches manual workflow_dispatch refs/heads/main
  run ! policy_matches manual release refs/heads/main
  run ! policy_matches manual pull_request refs/pull/1/merge
  run ! policy_matches manual workflow_dispatch refs/heads/feature
  run ! policy_matches manual workflow_dispatch refs/tags/v1.13.1
  run ! policy_matches manual workflow_dispatch refs/heads/main fork/orchestrion
  run ! policy_matches manual workflow_dispatch refs/heads/main DataDog/orchestrion other.yml
}
