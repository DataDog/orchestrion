#!/usr/bin/env pwsh

$integ = Join-Path (Get-Location) "_integration-tests"
$tests = Get-ChildItem -Path (Join-Path $integ "tests") -Name

if ($Args.Length -gt 0)
{
  $filtered = @()
  foreach ($a in $Args)
  {
    $found = $false
    foreach ($t in $tests)
    {
      if ($t -eq $a)
      {
        $filtered += $t
        $found = $true
        break
      }
    }
    if (!$found)
    {
      Write-Host "Test case '$($a)' does not exit" -ForegroundColor "Red"
    }
  }
  $tests = $filtered
}
if ($tests.Length -eq 0)
{
  Write-Host "No test cases selected, exiting immediately!" -ForegroundColor "Red"
  exit 1
}

$BinExt = ""
if ($IsWindows) {
  $BinExt = ".exe"
}

$Failed = @{}
$Skipped = @{}
$outputs = Join-Path (Get-Location) "_integration-tests" "outputs"
if (Test-Path $outputs)
{
  Remove-Item -Path $outputs -Recurse -Force
}
$null = New-Item -ItemType Directory -Path $outputs
"*" >(Join-Path $outputs ".gitignore") # So git never considers that content.
"module github.com/datadog/orchestrion/_integration-tests/outputs" >(Join-Path $outputs "go.mod")
"go 1.12" >>(Join-Path $outputs "go.mod")

# Build orchestrion
Write-Progress -Activity "Preparation" -Status "Building orchestrion" -PercentComplete 0
$orchestrion = Join-Path $outputs "orchestrion$($BinExt)"
go build -cover -covermode=atomic -coverpkg="github.com/datadog/orchestrion/..." -o $orchestrion .
if ($LastExitCode -ne 0)
{
  throw "Failed to build orchestrion"
}

$Env:GOCOVERDIR = Join-Path $outputs "coverage"
$null = New-Item -ItemType Directory -Path $Env:GOCOVERDIR -Force

# Warm up orchestrion
Write-Progress -Activity "Preparation" -Status "Warming up" -PercentComplete 50
try
{
  $env:ORCHESTRION_LOG_FILE = Join-Path $outputs "warmup" "orchestrion" '$PID.log'
  $env:ORCHESTRION_LOG_LEVEL = "TRACE"
  $env:GOTMPDIR = Join-Path $outputs "warmup" "tmp"
  $null = New-Item -ItemType Directory -Path $env:GOTMPDIR # The directory must exist...
  & $orchestrion warmup -work 2>&1 1>(Join-Path $outputs "warmup" "output.log")
  if ($LastExitCode -ne 0)
  {
    throw "Failed to warm up orchestrion"
  }
}
finally
{
  $env:GOTMPDIR = $null
  $env:ORCHESTRION_LOG_LEVEL = $null
  $env:ORCHESTRION_LOG_FILE = $null
}

Write-Progress -Activity "Preparation" -Status "Install test agent" -PercentComplete 75
$venv = $(Join-Path $outputs "venv")
python -m venv $venv 2>&1 1> (Join-Path $outputs "python-venv.log")
if ($LastExitCode -ne 0)
{
  throw "Failed to create python virtual environment"
}
Write-Progress -Activity "Preparation" -Status "Install test agent" -PercentComplete 85

$scripts = "bin"
if ($IsWindows)
{
  # On Windows, the venv binaries directory is called "Scripts" for some reason.
  $scripts = "Scripts"
}
. (Join-Path $venv $scripts "Activate.ps1")
pip install "ddapm-test-agent" 2>&1 1> (Join-Path $outputs "pip.log")
if ($LastExitCode -ne 0)
{
  throw "Failed to pip install ddapm-test-agent"
}

Write-Progress -Activity "Preparation" -Completed

# Running test cases
Write-Progress -Activity "Testing" -Status "Initialization" -PercentComplete 0
try
{
  if ((docker context inspect --format '{{ .Name }}') -eq "colima")
  {
    $env:DOCKER_HOST = docker context inspect --format '{{ .Endpoints.docker.Host }}'
    $env:TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE = '/var/run/docker.sock'
  }
  elseif ($IsWindows)
  {
    $osType = docker info --format '{{ .OSType }}'
    if ("linux" -ne $osType)
    {
      throw "Unable to run Linux containers (OS Type of Docker is $($osType))"
    }
  }
}
catch
{
  Write-Host "Docker is not available ($($_.Exception)), skipping tests that require it" -ForegroundColor "Yellow"
  $env:DOCKER_NOT_AVAILABLE = 'true'
}
for ($i = 0 ; $i -lt $tests.Length ; $i++)
{
  $name = $tests[$i]
  Write-Progress -Activity "Testing" -Status "$($name) ($($i+1) of $($tests.Length))" -PercentComplete (100 * $i / $tests.Length)
  try {
    # Parse validator instructions
    $vfile = Join-Path $integ "tests" $name "validation.json"
    $json = Get-Content -Raw $vfile | ConvertFrom-Json

    # Create complete list of variants (the basic one being blank)
    $variants = @{"" = @()}
    if ($null -ne $json.variants)
    {
      foreach ($v in $json.variants.PSObject.Properties)
      {
        $variants[$v.Name] = $v.Value.flags
      }
    }

    foreach ($var in $variants.GetEnumerator())
    {
      # Build test case
      $outDir = Join-Path $outputs "tests" $name
      $logName = $name
      if ($var.Name -ne "")
      {
        $outDir = "$($outDir)@$($var.Name)"
        $logName = "$($logName)@$($var.Name)"
      }
      $extraArgs = $var.Value

      $null = New-Item -ItemType Directory -Path $outDir # Ensure the directory exists

      $bin = Join-Path $outDir "$($name)$($BinExt)"
      try
      {
        $env:ORCHESTRION_LOG_FILE = Join-Path $outDir "orchestrion-log" '$PID.log'
        $env:ORCHESTRION_LOG_LEVEL = "TRACE"
        $env:GOTMPDIR = Join-Path $outDir "tmp"
        $oldGoFlags = $env:GOFLAGS

        $null = New-Item -ItemType Directory -Path $env:GOTMPDIR # The directory must exist...
        switch ($env:TESTCASE_BUILD_MODE)
        {
          "TOOLEXEC"
          {
            Write-Output "[$($logName)]: Building with manual -toolexec command"
            go -C $integ build @extraArgs `
              -toolexec "$($orchestrion) toolexec" `
              -work -o $bin "./tests/$($name)" 2>&1 1>(Join-Path $outDir "build.log")
          }
          "GOFLAGS"
          {
            Write-Output "[$($logName)]: Building with GOFLAGS command"
            $env:GOFLAGS = "$($oldGoFlags) '-toolexec=$($orchestrion) toolexec'"
            go -C $integ build @extraArgs -work -o $bin "./tests/$($name)" 2>&1 1>(Join-Path $outDir "build.log")
          }
          default
          {
            Write-Output "[$($logName)]: Building with orchestrion driver command (TESTCASE_BUILD_MODE=$($env:TESTCASE_BUILD_MODE))"
            & $orchestrion go -C $integ build @extraArgs `
              -work -o $bin "./tests/$($name)" 2>&1 1>(Join-Path $outDir "build.log")
          }
        }
        if ($LastExitCode -ne 0)
        {
          Write-Output "[$($logName)] Build failed; output follows:"
          Get-Content -Path (Join-Path $outDir "build.log") | Write-Output
          throw "Failed to build test case"
        }
      }
      finally
      {
        $env:GOFLAGS = $oldGoFlags
        $env:GOTMPDIR = $null
        $env:ORCHESTRION_LOG_LEVEL = $null
        $env:ORCHESTRION_LOG_FILE = $null
      }

      # Run test case
      $env:TRACE_LANGUAGE = 'golang'
      $env:LOG_LEVEL = 'DEBUG'
      $env:ENABLED_CHECKS = 'trace_stall,trace_count_header,trace_peer_service,trace_dd_service'
      $agent = (& (Join-Path $venv $scripts "ddapm-test-agent") 2>&1 1>(Join-Path $outDir "agent.log")) &

      $server = Start-Process -FilePath $bin -RedirectStandardOutput (Join-Path $outDir "stdout.log") -RedirectStandardError (Join-Path $outDir "stderr.log") -PassThru
      try
      {
        $token = New-Guid
        $attemptsLeft = 10
        for (;;)
        {
          try
          {
            if ($agent.State -ne "Running")
            {
              throw "Agent is no longer running (state: $($agent.State))"
            }
            $null = Invoke-WebRequest -Uri "http://localhost:8126/test/session/start?test_session_token=$($token)" -MaximumRetryCount 15 -RetryIntervalSec 1
            break # Invoke-WebRequest returns IIF the response had a successful status code
          }
          catch [System.Net.Http.HttpRequestException]
          {
            if ($null -ne $_.Exception.Response.StatusCode)
            {
              throw "Failed to start test session: HTTP $($_.Exception.Response.StatusCode) - $($_.Exception.Response.StatusDescription)"
            }
            elseif ($attemptsLeft -le 0)
            {
              throw "Failed to start test session: Failed and all attempts are exhaused. Last error: $($_)"
            }
            else
            {
              $attemptsLeft--
              Start-Sleep -Milliseconds 150
            }
          }
        }

        # Perform validations
        $skip = false
        if ($null -ne $json.url)
        {
          Write-Output "[$($logName)]: Validating using: GET $($json.url)"
          $attemptsLeft = 600 # 60 seconds with poll interval of 100ms
          for (;;)
          {
            try
            {
              $null = Invoke-WebRequest -Uri $json.url
              break # Invoke-WebRequest returns IIF the response had a successful status code
            }
            catch [System.Net.Http.HttpRequestException]
            {
              if ($null -ne $_.Exception.Response.StatusCode)
              {
                throw "GET $($json.url) => HTTP $([int]$_.Exception.Response.StatusCode) ($($_.Exception.Response.StatusCode))"
              }
              elseif ($attemptsLeft -le 0)
              {
                throw "GET $($json.url) => Failed and all attempts are exhaused. Last error: $($_)"
              }
              elseif ($server.HasExited)
              {
                if ($server.ExitCode -eq 42)
                {
                  $skip = $true
                  break
                }
                throw "GET $($json.url) => Failed and server is no longer running. Last error: $($_)"
              }
              else
              {
                $attemptsLeft--
                Start-Sleep -Milliseconds 100
              }
            }
          }
        }
        elseif ($null -ne $json.curl)
        {
          Write-Output "[$($logName)]: Validating using: $($json.curl)"
          Invoke-Expression "$($json.curl) --retry 5 --retry-all-errors --retry-max-time 30 2>&1 1>$(Join-Path $outDir "curl.log")"
          if ($LastExitCode -ne 0)
          {
            throw "Validation failed: $($json.curl)"
          }
        }
        else
        {
          throw "No validation instructions found!"
        }

        if ($skip)
        {
          Write-Host "[$($logName)]: Unsupported on this platform" -ForegroundColor "Yellow"
          $Skipped.$name = $true
        }
        else
        {
          Write-Output "[$($logName)]: Validation was successful"
          try
          {
            $null = Invoke-WebRequest -Uri $json.quit -MaximumRetryCount 5 -RetryIntervalSec 1
          }
          catch
          {
            $null = $_ # Swallow the exception
          }

          $server.WaitForExit()
          for (;;)
          {
            $resp = Invoke-WebRequest -Uri "http://localhost:8126/test/session/traces?test_session_token=$($token)" -MaximumRetryCount 5 -RetryIntervalSec 1
            $data = $resp.Content | ConvertFrom-Json
            if ($data.Length -ne 0)
            {
              Write-Output "[$($logName)]: Collected $($data.Length) traces"
              $tracesFile = Join-Path $outDir "traces.json"
              $resp.Content > $($tracesFile)

              go -C $integ run ./validator -name $name -variant $var.Name -validation $vfile -traces $tracesFile 2>&1 | Write-Host
              if ($LastExitCode -ne 0)
              {
                throw "Validation of traces failed"
              }

              Write-Host "[$($logName)]: Success!" -ForegroundColor "Green"
              break
            }
          }
        }
      }
      finally
      {
        if (!$server.HasExited)
        {
          $server.Kill($true)
        }
        Remove-Job -Job $agent -Force
      }
    }
  }
  catch
  {
    Write-Host "[$($name)]: Failed: $($_)" -ForegroundColor "Red"
    $Failed.$name = $_
  }
}
Write-Progress -Activity "Testing" -Completed

Write-Host ""
Write-Host "###########################" -ForegroundColor "Blue"
Write-Host "Summary:" -ForegroundColor "Blue"
foreach ($t in $tests)
{
  $color = "Green"
  $icon = "✅"
  $status = "Success"
  if ($null -ne $Failed.$t)
  {
    $color = "Red"
    $icon = "💥"
    $status = $Failed.$t
  }
  elseif ($null -ne $Skipped.$t)
  {
    $color = "Yellow"
    $icon = "⚠️"
    $status = "Skipped (unsupported on this platform)"
  }
  Write-Host "- $($icon) $($t): $($status)" -ForegroundColor $color
}

if ($Failed.Count -gt 0)
{
  exit 1
}
