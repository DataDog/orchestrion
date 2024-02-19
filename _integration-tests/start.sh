#!/usr/bin/env bash

# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2023-present Datadog, Inc.

####################################################################################################
# Usage: /start.sh <test> [args...]
#
# This program starts the service handler specified as `<test>` from the `/.bin` directory. It
# starts a test agent session before starting the handler, and collects the traces from the session
# upon exit and puts them at `/output/traces.json`. The handlers SDTOUT and STDERR streams are
# redirected to `/output/stdout.log` and `/output/stderr.log` respectively.
####################################################################################################

set -euo pipefail

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ $# -lt 1 ]]; then
  echo "Usage: ${SCRIPTDIR}/start.sh <test> [args...]"
  echo "Error: missing argument value for <test>"
  exit 2
fi

TEST_NAME="${1}"
shift

token="${TEST_NAME}-$(date +'%s.%N')"

finish() {
  rv=$?
  local token=$1
  echo "Service exited with status ${rv}"
  echo "Reading traces from the agent..."
  for (( i=0; i<5; i++ )); do
    curl -f "http://${DD_AGENT_HOST:-localhost}:8126/test/session/traces?test_session_token=${token}" \
      -o "/output/traces.json"
    if [[ "$(jq '. | length' < /output/traces.json)" == "0" ]]; then
      echo "Agent returned no traces... retrying in 1 second..."
      sleep 1
    else
      break
    fi
  done
  exit $rv
}

term() {
  local pid=$1
  echo "Received SIGTERM, forwarding to ${pid}..."
  kill -TERM "${pid}"
}

echo "Copying instrumented source to output..."
mkdir -p "/output/src"
cp -r ${SCRIPTDIR}/tests/${TEST_NAME}/*.go "/output/src/"

echo -n "Starting test session with the agent..."
curl -f --retry 5 --retry-all-errors --retry-max-time 30 "http://${DD_AGENT_HOST:-localhost}:8126/test/session/start?test_session_token=${token}"

echo "" # The test agent response does not end with a new line...
trap "finish ${token}" EXIT

if [[ "${DEBUG:-}" == "" ]]; then
  echo "Starting service handler..."
  "${SCRIPTDIR}/.bin/${TEST_NAME}" "$@" >/output/stdout.log 2>/output/stderr.log &
else
  echo "Starting service handler in DEBUG mode..."
  dlv --listen=:2345 \
    --headless=true \
    --accept-multiclient \
    --api-version=2 \
    exec \
  "${SCRIPTDIR}/.bin/${TEST_NAME}" "$@" >/output/stdout.log 2>/output/stderr.log &
fi
pid=$!

# We trap TERM to circumvent bash's default behavior & ensure the EXIT trap does not run prematurely.
trap "term ${pid}" 'TERM'

wait "${pid}"
