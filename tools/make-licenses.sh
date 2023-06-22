#!/usr/bin/env bash
set -euo pipefail

./bin/go-licenses save --save_path /tmp/licenses . 2> errors
./bin/go-licenses report . --template ./tools/licenses.tpl > LICENSE-3rdparty.csv 2>> errors

go run ./tools/copyrights/add_copyrights.go 

rm -rf /tmp/licenses
