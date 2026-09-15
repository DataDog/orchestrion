#!/usr/bin/env bash
set -euo pipefail

TMPDIR=$(mktemp -d "${TMPDIR}/make-licenses.XXXXXX")
trap "rm -rf ${TMPDIR}" EXIT ERR TERM

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

go -C $(dirname "${BASH_SOURCE[0]}") build -o "${TMPDIR}/bin/go-licenses" github.com/google/go-licenses/v2

# This package somehow breaks the license detection...
IGNORE_LIST="github.com/DataDog/sketches-go/ddsketch"

# We run for linux, darwin and windows, each on amd64 and arm64, to get all the licenses, including
# platform- and architecture-conditional ones. GOARCH is set explicitly (never inherited from the host)
# so the generated CSV is identical regardless of which architecture ran this script.
SOURCES="${TMPDIR}/sources"
mkdir -p "${SOURCES}"
declare -a LICENSE_FILES
for GOOS in linux darwin windows; do
  for GOARCH in amd64 arm64; do
    SOURCE_DIR="${TMPDIR}/sources-${GOOS}-${GOARCH}"
    echo "Aggregating source files in $(basename "${SOURCE_DIR}") so we can scrape copyright statements later..."
    GOOS="${GOOS}" GOARCH="${GOARCH}" "${TMPDIR}/bin/go-licenses" save --ignore "${IGNORE_LIST}" --save_path "${SOURCE_DIR}" ./... 2> "${TMPDIR}/errors" || (cat "${TMPDIR}/errors" >&2 && exit -1)
    chmod -R a+rw "${SOURCE_DIR}"
    cp -r "${SOURCE_DIR}"/* "${SOURCES}/"

    OUTFILE="${TMPDIR}/LICENSE-3rdparty.${GOOS}-${GOARCH}.csv"
    echo "Building $(basename "${OUTFILE}")"
    GOOS="${GOOS}" GOARCH="${GOARCH}" "${TMPDIR}/bin/go-licenses" report ./... --ignore "${IGNORE_LIST}" --template ./_tools/licenses.tpl > "${OUTFILE}" 2> "${TMPDIR}/errors" || (cat "${TMPDIR}/errors" >&2 && exit -1)
    LICENSE_FILES+=("${OUTFILE}")
  done
done

go run ./_tools/copyrights/merge.go -licenses "${SOURCES}" -output LICENSE-3rdparty.csv "${LICENSE_FILES[@]}"
