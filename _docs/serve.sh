#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
go run ./generator
git submodule update --init --recursive
go run github.com/gohugoio/hugo serve "$@"
