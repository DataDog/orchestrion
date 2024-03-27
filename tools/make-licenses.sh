#!/usr/bin/env bash
set -euo pipefail

TMPDIR=$(mktemp -d)
trap "rm -rf ${TMPDIR}" EXIT

go run github.com/google/go-licenses save --save_path ${TMPDIR}/licenses ./... 2> errors
go run github.com/google/go-licenses report ./... --template ./tools/licenses.tpl > LICENSE-3rdparty.csv 2>> errors

go run ./tools/copyrights/add_copyrights.go

rm -rf /tmp/licenses
