#!/usr/bin/env bash
# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2023-present Datadog, Inc.

#set -x

#export DD_TRACE_DEBUG=true
fail=0
failures=""

testname="$1"

fail() {
    echo FAIL: $1
    fail=1
    failures+="FAIL: ${1}\n"
}

pass() {
    echo PASS: $1
    failures+="PASS: ${1}\n"
}

## Build Orchestrion
go build -o ./_integration-tests/orchestrion

cd ./_integration-tests

## We need to change the default behavior of getting the latest release
## and instead use the head of the main branch.
go get github.com/datadog/orchestrion@main

## Run Orchestrion on the integration test services
./orchestrion -w ./tests

## Build and run all the tests
go mod tidy
go build -o valid ./validator
for i in tests/*; do    
    ss -lntp
    tname=`basename $i`
    if [[ "$testname" != "" && "$testname" != "$tname" ]]; then
       continue
    fi
    echo '################################################################################'
    echo RUN ${tname}:
    curl "http://localhost:8126/test/session/start?test_session_token=${tname}"
    #
    go build ./${i} || { fail $tname; continue; }
    
    
    ./${tname} &
    testpid=$!

    ## Send a request to the "url" field in validation.json, if present.
    url=`cat ${i}/validation.json | jq -r '.url // empty'`
    if [[ "$url" != "" ]]; then 
	curl "$url" || { fail $tname; continue; }
    fi

    ## Run the 'curl' command in the "curl" field in validation.json, if present.
    curl_command=`cat ${i}/validation.json | jq -r '.curl // empty'`
    if [[ "$curl_command" != "" ]]; then 
	bash -c "$curl_command" || { fail $tname; continue; }
    fi

    ## Send SIGTERM to the test program.
    kill $testpid
    ## In the background, wait 30 seconds and then send a SIGKILL, in case the program fails to
    ## shut down as a result of the SIGTERM. This prevents test hangs if the program does not
    ## respond correctly to the SIGTERM.
    (sleep 30; kill -9 $testpid >/dev/null 2>&1) &

    ## Wait for the program to shut down
    wait $testpid
    
    ./valid -tname ${tname} -vfile ${i}/validation.json -surl "http://localhost:8126/test/session/traces?test_session_token=${tname}" && pass $tname || fail $tname
done

if [ "$fail" != "0" ]; then
    echo "The integration test suite Failed. See the failed tests below and see the logs above to diagnose failures."
else
    echo "The integration test suite Passed."
fi

echo -ne $failures
exit $fail
