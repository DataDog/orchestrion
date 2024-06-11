#!/usr/bin/env bash

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
git submodule update --init --recursive
go run github.com/gohugoio/hugo serve "$@"
