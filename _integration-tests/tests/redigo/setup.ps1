$oldErrorActionPreference = $ErrorActionPreference
try
{
  # Necessary because the exception thrown by Get-Command when the command is not found is a
  # non-terminating error, which does not trigger the catch block. Setting this preference allows
  # the catch to be executed.
  $ErrorActionPreference = "stop"
  $null = Get-Command "redis-server"
}
catch
{


  if ($IsLinux)
  {
    sudo apt update && \
      sudo apt install -y lsb-release curl gpg && \
      curl -fsSL https://packages.redis.io/gpg | sudo gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg && \
      Write-Output "deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/redis.list && \
      sudo apt update && \
      sudo apt install redis
  }
  elseif ($IsMacOS)
  {
    brew install redis
  }
  elseif ($IsWindows)
  {
    choco install redis
  }
  else {
    throw "Unsupported platform, please install redis-server on your own!"
  }

  if ($LastExitCode -ne 0)
  {
    throw "Failed to install redis-server (exit code $($LastExitCode))"
  }
}
finally
{
  $ErrorActionPreference = $oldErrorActionPreference
}

redis-server --bind 127.0.0.1 --dir $Args[0] --dbfilename "redis-dump.rdb" 2>&1 1>(Join-Path $Args[0] "redis-server.log") &
