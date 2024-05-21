#!/usr/bin/env pwsh

function New-TemporaryDirectory {
  $parent = [System.IO.Path]::GetTempPath()
  $name = [System.IO.Path]::GetRandomFileName()
  New-Item -ItemType Directory -Path (Join-Path $parent $name)
}


$Failed = @()
$TmpDir = New-TemporaryDirectory
try
{
  $outputs = Join-Path (Get-Location) "_integration-tests" "outputs"

  # Build orchestrion
  Write-Progress -Activity "Preparation" -Status "Building orchestrion" -PercentComplete 0
  $orchestrion = Join-Path $outputs "orchestrion.exe"
  go build -o $orchestrion .
  if ($LastExitCode -ne 0)
  {
    throw "Failed to build orchestrion"
  }

  # Warm up orchestrion
  Write-Progress -Activity "Preparation" -Status "Warming up" -PercentComplete 50
  & $orchestrion warmup
  if ($LastExitCode -ne 0)
  {
    throw "Failed to warm up orchestrion"
  }

  Write-Progress -Activity "Preparation" -Status "Install test agent" -PercentComplete 75
  $venv = $(Join-Path $TmpDir "venv")
  python -m venv $venv 2>&1 1> (Join-Path $outputs "python-venv.log")
  if ($LastExitCode -ne 0)
  {
    throw "Failed to create python virtual environment"
  }
  . (Join-Path $venv "bin" "activate.ps1")
  pip install "ddapm-test-agent" 2>&1 1> (Join-Path $outputs "pip.log")
  if ($LastExitCode -ne 0)
  {
    throw "Failed to pip install ddapm-test-agent"
  }

  Write-Progress -Activity "Preparation" -Completed

  # Running test cases
  Write-Progress -Activity "Testing" -Status "Initialization" -PercentComplete 0
  $integ = Join-Path (Get-Location) "_integration-tests"
  $tests = Get-ChildItem -Path (Join-Path $integ "tests") -Name
  for ($i = 0 ; $i -lt $tests.Length ; $i++)
  {
    $name = $tests[$i]
    Write-Progress -Activity "Testing" -Status "$($name) ($($i+1) of $($tests.Length))" -PercentComplete (100 * $i / $tests.Length)
    try {
      # Parse validator instructions
      $vfile = Join-Path $integ "tests" $name "validation.json"
      $json = Get-Content -Raw $vfile | ConvertFrom-Json

      # Build test case
      $outDir = Join-Path $outputs $name
      $bin = Join-Path $outDir "$($name)"
      if ($IsWindows)
      {
        # Correctly have the extension...
        $bin += ".exe"
      }
      try
      {
        $env:ORCHESTRION_LOG_FILE = Join-Path $outDir "orchestrion-log" "\$PID.log"
        $env:ORCHESTRION_LOG_LEVEL = "TRACE"
        & $orchestrion go -C $integ build -o $bin "./tests/$($name)" 2>&1 1>(Join-Path $outDir "build.log")
        if ($LastExitCode -ne 0)
        {
          throw "Failed to build test case"
        }
      }
      finally
      {
        $env:ORCHESTRION_LOG_LEVEL = ""
        $env:ORCHESTRION_LOG_FILE = ""
      }

      # Run test case
      $job = (& $bin 2>&1 1>(Join-Path $outDir "output.log")) &

      $env:TRACE_LANGUAGE = 'golang'
      $env:LOG_LEVEL = 'DEBUG'
      $env:ENABLED_CHECKS = 'trace_stall,trace_count_header,trace_peer_service,trace_dd_service'
      $agent = (& (Join-Path $venv "bin" "ddapm-test-agent") 2>&1 1>(Join-Path $outDir "agent.log")) &
      try {
        if ($job.State -ne "Running")
        {
          throw "Failed to run test case (state is $($job.State))"
        }

        $token = New-Guid
        $attemptsLeft = 10
        for (;;)
        {
          try
          {
            $null = Invoke-WebRequest -Uri "http://localhost:8126/test/session/start?test_session_token=$($token)" -MaximumRetryCount 10 -RetryIntervalSec 1
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
        if ($null -ne $json.url)
        {
          Write-Output "[$($name)]: Validating using: GET $($json.url)"
          $attemptsLeft = 5
          for (;;)
          {
            try
            {
              $null = Invoke-WebRequest -Uri $json.url -MaximumRetryCount 5 -RetryIntervalSec 1
              break # Invoke-WebRequest returns IIF the response had a successful status code
            }
            catch [System.Net.Http.HttpRequestException]
            {
              if ($null -ne $_.Exception.Response.StatusCode)
              {
                throw "GET $($json.url) => HTTP $($_.Exception.Response.StatusCode) - $($_.Exception.Response.StatusDescription)"
              }
              elseif ($attemptsLeft -le 0)
              {
                throw "GET $($json.url) => Failed and all attempts are exhaused. Last error: $($_)"
              }
              else
              {
                $attemptsLeft--
                Start-Sleep -Milliseconds 150
              }
            }
          }
        }
        elseif ($null -ne $json.curl)
        {
          Write-Output "[$($name)]: Validating using: $($json.curl)"
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

        Write-Output "[$($name)]: Validation was successful"
        try
        {
          $null = Invoke-WebRequest -Uri $json.quit -MaximumRetryCount 5 -RetryIntervalSec 1
        }
        catch
        {
          $null = $_ # Swallow the exception
        }

        $null = Wait-Job -Job $job -Timeout 15
        for (;;)
        {
          $resp = Invoke-WebRequest -Uri "http://localhost:8126/test/session/traces?test_session_token=$($token)" -MaximumRetryCount 5 -RetryIntervalSec 1
          $data = $resp.Content | ConvertFrom-Json
          if ($data.Length -ne 0)
          {
            Write-Output "[$($name)]: Collected $($data.Length) traces"
            $tracesFile = Join-Path $outDir "traces.json"
            $resp.Content > $($tracesFile)

            go -C $integ run ./validator -tname $name -vfile $vfile -surl "file://$($tracesFile)" 2>&1 | Write-Host
            if ($LastExitCode -ne 0)
            {
              throw "Validation of traces failed"
            }

            Write-Host "[$($name)]: Success!" -ForegroundColor "Green"
            break
          }
        }
      }
      finally
      {
        Remove-Job -Job $job -Force
        Remove-Job -Job $agent -Force
      }
    }
    catch
    {
      Write-Host "[$($name)]: Failed: $($_)" -ForegroundColor "Red"
      $Failed += $name
    }
  }
  Write-Progress -Activity "Testing" -Completed
}
finally
{
  # Clean up the temporary directory
  Remove-Item -Recurse -Force $TmpDir
}

if ($Failed.Length -gt 0)
{
  Write-Host "###########################" -ForegroundColor "Red"
  Write-Host "Some tests failed:" -ForegroundColor "Red"
  foreach ($name in $Failed)
  {
    Write-Host "- $($name)" -ForegroundColor "Red"
  }
  exit 1
}

Write-Host "###########################" -ForegroundColor "Green"
Write-Host "All tests were successful!" -ForegroundColor "Green"
