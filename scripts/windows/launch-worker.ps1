#Requires -Version 5.1
<#
.SYNOPSIS
  Windows-side launcher that waits for WSL, then starts the Cursor agent worker.

.DESCRIPTION
  Used from the Startup folder and/or Scheduled Task after auto logon.
  Logs to %USERPROFILE%\.gumi\logs\cursor-worker-windows.log

  Retries forever so a transient WSL boot failure cannot leave the machine
  without a worker.
#>
[CmdletBinding()]
param(
  [string]$Distro = 'Ubuntu',
  [string]$WslUser = 'dev',
  [string]$WorkerDir = '/home/dev/Gumi',
  [string]$WorkerName = 'gumi-windows',
  [int]$InitialDelaySec = 20,
  [int]$WslReadyTimeoutSec = 180,
  [int]$RetryDelaySec = 10
)

$ErrorActionPreference = 'Continue'
$logDir = Join-Path $env:USERPROFILE '.gumi\logs'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$logFile = Join-Path $logDir 'cursor-worker-windows.log'
$scriptPath = "$WorkerDir/scripts/windows/start-cursor-worker.sh"

function Write-Log([string]$Message) {
  $line = '{0} {1}' -f (Get-Date -Format 'o'), $Message
  Add-Content -Path $logFile -Value $line
  Write-Host $line
}

function Wait-WslReady {
  $deadline = (Get-Date).AddSeconds($WslReadyTimeoutSec)
  while ((Get-Date) -lt $deadline) {
    & wsl.exe -d $Distro -u $WslUser --exec /bin/true 2>$null
    if ($LASTEXITCODE -eq 0) { return $true }
    Write-Log "waiting for WSL ready (last exit=$LASTEXITCODE)"
    Start-Sleep -Seconds 3
  }
  return $false
}

function Start-WorkerOnce {
  # Pass env into WSL without fragile bash -lc quoting.
  $env:CURSOR_WORKER_DIR = $WorkerDir
  $env:CURSOR_WORKER_NAME = $WorkerName
  $env:WSLENV = 'CURSOR_WORKER_DIR/u:CURSOR_WORKER_NAME/u'

  # Ensure executable bit (idempotent).
  & wsl.exe -d $Distro -u $WslUser --exec /bin/chmod +x $scriptPath 2>$null

  Write-Log "starting worker watchdog name=$WorkerName dir=$WorkerDir script=$scriptPath"
  # --exec avoids login-shell PATH pollution from Windows (Program Files (x86)).
  & wsl.exe -d $Distro -u $WslUser --exec /bin/bash $scriptPath
  return $LASTEXITCODE
}

Write-Log "launcher start distro=$Distro user=$WslUser delay=${InitialDelaySec}s"
if ($InitialDelaySec -gt 0) {
  Start-Sleep -Seconds $InitialDelaySec
}

while ($true) {
  if (-not (Wait-WslReady)) {
    Write-Log "ERROR: WSL not ready within ${WslReadyTimeoutSec}s; retrying in ${RetryDelaySec}s"
    Start-Sleep -Seconds $RetryDelaySec
    continue
  }
  Write-Log 'WSL ready'

  $code = Start-WorkerOnce
  Write-Log "worker/wsl exited code=$code; retrying in ${RetryDelaySec}s"
  Start-Sleep -Seconds $RetryDelaySec
}
