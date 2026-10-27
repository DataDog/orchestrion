#!/usr/bin/env bats
# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2026-present Datadog, Inc.

setup() {
  bats_require_minimum_version 1.5.0
  export FIXTURE="${BATS_TEST_TMPDIR}/repo"
  export TMPDIR="${BATS_TEST_TMPDIR}/tmp"
  export CALLS="${BATS_TEST_TMPDIR}/calls" MERGE_ARGS="${BATS_TEST_TMPDIR}/merge-args"
  export MOCK_LICENSES="${BATS_TEST_TMPDIR}/go-licenses"
  mkdir -p "${FIXTURE}/_tools" "${TMPDIR}"
  cp "${BATS_TEST_DIRNAME}/make-licenses.sh" "${BATS_TEST_DIRNAME}/verify-licenses.sh" "${FIXTURE}/_tools/"
  cd "${FIXTURE}" || return
  cat >"${MOCK_LICENSES}" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
command="$1"
shift
printf '%s:%s/%s\n' "${command}" "${GOOS}" "${GOARCH}" >>"${CALLS}"
if [[ "${TEST_LICENSE_FAILURE:-}" == "${command}" ]]; then
  printf 'mock %s failure\n' "${command}" >&2
  exit 19
fi
case "${command}" in
  save)
    while [[ "$1" != --save_path ]]; do shift; done
    mkdir -p "$2"
    printf 'license text\n' >"$2/${GOOS}-${GOARCH}.txt"
    ;;
  report) printf 'license,%s,%s\n' "${GOOS}" "${GOARCH}" ;;
  *) exit 99 ;;
esac
MOCK
  export -f go git
}

go() {
  case "$1" in
    -C)
      [[ "$3" == build && "$4" == -o ]] || return 99
      [[ "${TEST_BUILD_STATUS:-0}" == 0 ]] || return "${TEST_BUILD_STATUS}"
      mkdir -p "$(dirname "$5")"
      cp "${MOCK_LICENSES}" "$5"
      chmod +x "$5"
      ;;
    run)
      [[ "$2" == ./_tools/copyrights/merge.go && "$3" == -licenses && "$5" == -output ]] || return 99
      [[ "${TEST_MERGE_STATUS:-0}" == 0 ]] || return "${TEST_MERGE_STATUS}"
      printf '%s\n' "$@" >"${MERGE_ARGS}"
      local sources="$4" output="$6" csv
      shift 6
      [[ "$#" == 6 ]] || return 99
      for csv in "$@"; do
        [[ -s "${csv}" ]] || return 99
      done
      [[ "$(find "${sources}" -type f | wc -l | tr -d ' ')" == 6 ]] || return 99
      printf 'merged license report\n' >"${output}"
      ;;
    *) return 99 ;;
  esac
}

git() {
  printf 'git:%s\n' "$*" >>"${CALLS}"
  case "$*" in
    '--no-pager diff LICENSE-3rdparty.csv')
      [[ "${TEST_DIFF_STATUS:-0}" == 0 ]] || return "${TEST_DIFF_STATUS}"
      printf '%s' "${TEST_CSV_DIFF:-}"
      ;;
    'ls-files docs/ --exclude-standard --others') printf '%s' "${TEST_UNTRACKED_DOCS:-}" ;;
    *) return 99 ;;
  esac
}

assert_cleaned_up() {
  run -0 find "${TMPDIR}" -mindepth 1 -print
  [[ -z "${output}" ]]
}

prepare_verifier() {
  # Test verification independently of generation; neither touches the real repository.
  cat >_tools/make-licenses.sh <<'MOCK'
#!/usr/bin/env bash
printf 'regenerate\n' >>"${CALLS}"
exit "${TEST_REGENERATE_STATUS:-0}"
MOCK
  chmod +x _tools/make-licenses.sh
}

@test "licenses are collected for all six platforms regardless of host environment" {
  run -0 env GOOS=freebsd GOARCH=386 bash _tools/make-licenses.sh
  [[ -s "${MERGE_ARGS}" && -s LICENSE-3rdparty.csv ]]
  run -0 cat "${CALLS}"
  [[ "${output}" == $'save:linux/amd64\nreport:linux/amd64\nsave:linux/arm64\nreport:linux/arm64\nsave:darwin/amd64\nreport:darwin/amd64\nsave:darwin/arm64\nreport:darwin/arm64\nsave:windows/amd64\nreport:windows/amd64\nsave:windows/arm64\nreport:windows/arm64' ]]
  assert_cleaned_up
}

@test "license tool build failure is propagated and temporary files are removed" {
  run -17 env TEST_BUILD_STATUS=17 bash _tools/make-licenses.sh
  [[ ! -e "${MERGE_ARGS}" ]]
  assert_cleaned_up
}

@test "license source collection failure prevents merging and reports its error" {
  run ! env TEST_LICENSE_FAILURE=save bash _tools/make-licenses.sh
  [[ "${output}" == *'mock save failure'* && ! -e "${MERGE_ARGS}" ]]
  assert_cleaned_up
}

@test "license report failure prevents merging and reports its error" {
  run ! env TEST_LICENSE_FAILURE=report bash _tools/make-licenses.sh
  [[ "${output}" == *'mock report failure'* && ! -e "${MERGE_ARGS}" ]]
  assert_cleaned_up
}

@test "license merge failure is propagated and temporary files are removed" {
  run -23 env TEST_MERGE_STATUS=23 bash _tools/make-licenses.sh
  assert_cleaned_up
}

@test "license verification regenerates before checking tracked and untracked files" {
  prepare_verifier
  run -0 bash _tools/verify-licenses.sh
  run -0 cat "${CALLS}"
  [[ "${output}" == $'regenerate\ngit:--no-pager diff LICENSE-3rdparty.csv\ngit:ls-files docs/ --exclude-standard --others' ]]
}

@test "license verification rejects an outdated CSV with exit status two" {
  prepare_verifier
  run -2 env TEST_CSV_DIFF='changed license' bash _tools/verify-licenses.sh
  [[ "${output}" == *'License outdated:'* && "${output}" == *'changed license'* ]]
}

@test "license verification rejects untracked license documentation" {
  prepare_verifier
  run -2 env TEST_UNTRACKED_DOCS='docs/thirdparty/new-LICENSE' bash _tools/verify-licenses.sh
  [[ "${output}" == *'License removed:'* && "${output}" == *'docs/thirdparty/new-LICENSE'* ]]
}

@test "license regeneration failure stops verification before Git checks" {
  prepare_verifier
  run -29 env TEST_REGENERATE_STATUS=29 bash _tools/verify-licenses.sh
  run -0 cat "${CALLS}"
  [[ "${output}" == regenerate ]]
}

@test "Git diff failure is propagated instead of accepting a clean report" {
  prepare_verifier
  run -31 env TEST_DIFF_STATUS=31 bash _tools/verify-licenses.sh
}
