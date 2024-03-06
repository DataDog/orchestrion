#!/bin/bash

set -x
set -e
testDir=$(realpath $(dirname $0))
cd $testDir
go build -a -o proxy ./proxy

for pkg in pkg_*
do
    # Inject $pkg into the base package
    go build -a -o main -toolexec "$testDir/proxy/proxy $testDir/$pkg/cfg.yaml" ./base
    out=$(./main)
    # Make sure running the program yields the instrumented output
    [ "$out" = "$pkg" ]
    rm -f main
done

rm -f proxy/proxy
