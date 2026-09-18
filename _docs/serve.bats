#!/usr/bin/env bats
# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2026-present Datadog, Inc.

setup() {
  bats_require_minimum_version 1.5.0
  export DOCS="${BATS_TEST_TMPDIR}/project/_docs"
  export CALLS="${BATS_TEST_TMPDIR}/calls" HUGO_ARGS="${BATS_TEST_TMPDIR}/args"
  mkdir -p "${DOCS}"
  cp "${BATS_TEST_DIRNAME}/serve.sh" "${DOCS}/serve.sh"
  cd "${BATS_TEST_TMPDIR}" || return
  export -f go git
}

go() {
  [[ "${PWD}" == "${DOCS}" ]] || return 99
  printf 'go:%s\n' "$*" >>"${CALLS}"
  case "$2" in
    ./generator) return "${TEST_GENERATOR_STATUS:-0}" ;;
    github.com/gohugoio/hugo)
      printf '%s\n' "$@" >"${HUGO_ARGS}"
      return "${TEST_HUGO_STATUS:-0}"
      ;;
    *) return 99 ;;
  esac
}

git() {
  [[ "${PWD}" == "${DOCS}" ]] || return 99
  [[ "$*" == 'submodule update --init --recursive' ]] || return 99
  printf 'git:%s\n' "$*" >>"${CALLS}"
  return "${TEST_SUBMODULE_STATUS:-0}"
}

@test "docs generation and submodule setup precede serving with preserved arguments" {
  run -0 bash "${DOCS}/serve.sh" --title 'Local docs' --port 1314
  run -0 head -n 2 "${CALLS}"
  [[ "${output}" == $'go:run ./generator\ngit:submodule update --init --recursive' ]]
  run -0 cat "${HUGO_ARGS}"
  [[ "${output}" == $'run\ngithub.com/gohugoio/hugo\nserve\n--title\nLocal docs\n--port\n1314' ]]
}

@test "generator failure stops before submodule setup or serving" {
  run -17 env TEST_GENERATOR_STATUS=17 bash "${DOCS}/serve.sh"
  [[ ! -e "${HUGO_ARGS}" ]]
  run -0 cat "${CALLS}"
  [[ "${output}" == 'go:run ./generator' ]]
}

@test "submodule failure stops before serving" {
  run -23 env TEST_SUBMODULE_STATUS=23 bash "${DOCS}/serve.sh"
  [[ ! -e "${HUGO_ARGS}" ]]
}

@test "server exit status is preserved" {
  run -37 env TEST_HUGO_STATUS=37 bash "${DOCS}/serve.sh"
}
