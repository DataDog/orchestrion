#!/usr/bin/env pwsh

function New-TemporaryDirectory {
  $parent = [System.IO.Path]::GetTempPath()
  $name = [System.IO.Path]::GetRandomFileName()
  New-Item -ItemType Directory -Path (Join-Path $parent $name)
}

$TmpDir = New-TemporaryDirectory
try {
  go -C "_integration-tests" build -o $TmpDir ./tests/...
} finally {
  # Clean up the temporary directory
  Remove-Item -Recurse -Force $TmpDir
}
