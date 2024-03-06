#!/bin/bash

set -exuo pipefail

testDir=$(realpath $(dirname $0))
cacheDir=$(mktemp -d $TMPDIR/gocache-XXXXX)
cd $testDir
go build -a -o proxy ./proxy

for pkg in pkg_*
do
    # Inject $pkg into the base package
    GOCACHE=$cacheDir go build -a -o main -toolexec "$testDir/proxy/proxy $testDir/$pkg/cfg.yaml" ./base
    out=$(./main)
    # Make sure running the program yields the instrumented output
    diff  <(echo "$out") <(echo "$pkg")
    rm -f main
done

rm -f proxy/proxy
rm -r $cacheDir
