#!/usr/bin/env bash

# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2023-present Datadog, Inc.

####################################################################################################
# These integration tests use Docker to isolate the tested behaviors from any local environment
# service that may affect test outcomes. For example, a local Datadog agent running on port 8123 may
# interfere with the test agent's ability to return complete traces.
####################################################################################################

set -euo pipefail

ROOT_DIR=$(cd $(dirname "${BASH_SOURCE[0]}") && pwd)

fail=0
messages=""

testname="${1:-}"

fail() {
    local message="\033[0;31mFAIL: ${1}\033[0m\n"
    echo -e "${message}"
    fail=1
    messages+="${message}"
}

pass() {
    local message="\033[0;32mPASS: ${1}\033[0m\n"
    echo -e "${message}"
    messages+="${message}"
}

cid=$(mktemp "${TMPDIR}/orchestrion-integration-tests-CID-XXXXXXXXXX")
trap "rm -f ${cid}" EXIT

network="host"
## If we're not running in a github action, set up the fake agent locally.
if [[ "${GITHUB_ACTIONS:-}" == "" ]]; then
    rm -f "${cid}" # Docker run refuses to proceed if it already exists...
    docker run --rm -id --cidfile="${cid}" -eLOG_LEVEL=DEBUG -eTRACE_LANGUAGE=golang -eENABLED_CHECKS=trace_stall,trace_count_header,trace_peer_service,trace_dd_service ghcr.io/datadog/dd-apm-test-agent/ddapm-test-agent:latest
    agent_cid=$(cat "${cid}")
    trap "echo -n 'Stopping agent container: '; docker container rm -f ${agent_cid}" EXIT

    network="container:${agent_cid}"
fi

## Prepare output directory
OUT_DIR="${ROOT_DIR}/_integration-tests/outputs"
rm -rf "${OUT_DIR}" # Ensure the directory is empty before we start
mkdir -p "${OUT_DIR}"
echo "*" > ${OUT_DIR}/.gitignore # Make sure it's always ignored by git once it exists

## Pre-build orchestrion binary
echo "Building orchestrion binary"
BIN_DIR="${OUT_DIR}/-orchestrion-"
mkdir -p "${BIN_DIR}"
eval $(go env) # Makes all Go environment variables available in this environment.
docker run --rm -t                                                              \
    -v"${ROOT_DIR}:${ROOT_DIR}" -w"${ROOT_DIR}"                                 \
    -v"${GOCACHE}:${GOCACHE}" -eGOCACHE="${GOCACHE}"                            \
    -v"${GOMODCACHE}:${GOMODCACHE}" -eGOMODCACHE="${GOMODCACHE}"                \
    -v"${BIN_DIR}:/output"                                                      \
    -eGOPROXY="${GOPROXY}"                                                      \
    golang:${GOVERSION#go}                                                      \
    go build -o /output/ .

echo "Building base image: "
cat <<EOF > "${BIN_DIR}/Dockerfile"
FROM golang:${GOVERSION#go}
RUN apt update -y && apt install -y curl jq
COPY orchestrion /usr/local/bin/orchestrion
EOF
iid=$(mktemp "${TMPDIR}/orchestrion-integration-tests-IID-XXXXXXXXXX")
trap "rm -f ${iid}" EXIT
docker build --iidfile="${iid}" "${BIN_DIR}"
image=$(cat "${iid}")

## Run all the tests
cd "${ROOT_DIR}/_integration-tests"
for tdir in ./tests/*; do
    tname=$(basename ${tdir})
    if [[ "${testname}" != "" && "${testname}" != "${tname}" ]]; then
       continue
    fi

    echo -e "\033[0;36m################################################################################\033[0m"
    echo -e "RUN \033[0;35m${tname}\033[0m:"

    mkdir -p "${OUT_DIR}/${tname}/tmp" # Make sure the output directory exists

    # Build the service binary
    rm -f "${cid}" # Docker run refuses to proceed if it already exists...
    echo "Building the service entry point:"
    docker run --rm -t --net="${network}" --cidfile="${cid}" --quiet            \
        -v"${ROOT_DIR}:${ROOT_DIR}" -w"${ROOT_DIR}/_integration-tests"          \
        -v"${GOCACHE}:${GOCACHE}" -eGOCACHE="${GOCACHE}"                        \
        -v"${GOMODCACHE}:${GOMODCACHE}" -eGOMODCACHE="${GOMODCACHE}"            \
        -v"${BIN_DIR}:/var/opt/orchestrion/bin"                                 \
        -v"${OUT_DIR}/${tname}:/output"                                         \
        -eGOPROXY="${GOPROXY}"                                                  \
        -eGOTMPDIR="/output/tmp"                                                \
        "${image}"                                                              \
        orchestrion go build -work -o "/output/${tname}" "./tests/${tname}"     \
        || { fail "${tname}"; continue; }

    # Start the service in a Docker container
    rm -f "${cid}" # Docker run refuses to proceed if it already exists...
    echo "Starting service container:"
    docker run -dt --net="${network}" --cidfile="${cid}" --quiet                \
        -v"${OUT_DIR}/${tname}:/output" -w/output                               \
        -v"${ROOT_DIR}:${ROOT_DIR}"                                             \
        -v"${GOCACHE}:${GOCACHE}" -eGOCACHE="${GOCACHE}"                        \
        -v"${GOMODCACHE}:${GOMODCACHE}" -eGOMODCACHE="${GOMODCACHE}"            \
        -eGOPROXY="${GOPROXY}"                                                  \
        "${image}"                                                              \
        "${ROOT_DIR}/_integration-tests/start.sh" "${tname}"                    \
        || { fail "${tname}"; continue; }
    container=$(cat "${cid}")
    trap "echo -n 'Stopping ${tname} service container: '; docker container rm --force ${container}" EXIT

    ## Send a request to the "url" field in validation.json, if present.
    url=`cat "${tdir}/validation.json" | jq -r ".url // empty"`
    if [[ "${url}" != "" ]]; then
        echo "Hitting configured url '${url}':"
        # We use Docker here so we can reach the test's own network...
        docker run --rm -t --network="${network}" curlimages/curl -f "${url}" || { fail ${tname}; continue; }
        echo ""
    fi

    ## Run the 'curl' command in the "curl" field in validation.json, if present.
    curl_command=`cat "${tdir}/validation.json" | jq -r ".curl // empty"`
    if [[ "${curl_command}" != "" ]]; then
        echo "Executing configured command against service:"
        # We use Docker here so we can reach the test's own network...
        docker run --rm -t --network="${network}" --entrypoint="/bin/sh" curlimages/curl -c "${curl_command}" || { fail ${tname}; continue; }
        echo ""
    fi

    ## Send SIGTERM to the test program.
    echo -n "Sending SIGTERM to the service container: "
    docker container kill --signal 'TERM' "${container}" || true # Ignore failures

    ## Wait for the program to shut down (note we have an EXIT trap that'll forcefully remove the containers)
    echo -n "Waiting for the container to terminate (143 is SIGTERM, which is expected): "
    timeout 30 docker container wait "${container}" || {
        status="$?"
        case "${status}" in
        "124")
            echo "Timed out waiting for the container to terminate!"
            ;;
        *)
            echo "Failed with status ${status}!"
            ;;
        esac
        fail "${tname}"
        continue
    }

    echo "Container logs follow:"
    echo -en "\033[0;33m"
    docker logs "${container}"
    echo -en "\033[0m"

    echo "Validating traces..."
    go run ./validator                                                          \
        -tname ${tname}                                                         \
        -vfile ${tdir}/validation.json                                          \
        -surl "file://${PWD}/outputs/${tname}/traces.json"                      \
    && pass $tname || fail $tname
done

echo -e "\033[0;36m################################################################################\033[0m"
if [ "$fail" != "0" ]; then
    echo "The integration test suite Failed. See the failed tests below and see the logs above to diagnose failures."
else
    echo "The integration test suite Passed."
fi

echo -e $messages
exit $fail
