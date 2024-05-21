#!/usr/bin/env pwsh

function New-TemporaryDirectory {
  $parent = [System.IO.Path]::GetTempPath()
  $name = [System.IO.Path]::GetRandomFileName()
  New-Item -ItemType Directory -Path (Join-Path $parent $name)
}

$TmpDir = New-TemporaryDirectory
try {
  # Build orchestrion
  $orchestrion = Join-Path $TmpDir "orchestrion.exe"
  go build -o $orchestrion .

  # Warm up orchestrion
  $orchestrion warmup

  # Build the test cases
  $testBin = (Join-Path $TmpDir "tests")
  $orchestrion go -C "_integration-tests" build -o $testBin ./tests/...
} finally {
  # Clean up the temporary directory
  Remove-Item -Recurse -Force $TmpDir
}
