#Requires -Version 5.1
<#
.SYNOPSIS
  Windows-side launcher that waits for WSL, then starts the Cursor agent worker.

.DESCRIPTION
  Used from the Startup folder and/or Scheduled Task after auto logon.
  Logs to %USERPROFILE%\.gumi\logs\cursor-worker-windows.log
#>
[CmdletBinding()]
param(
  [string]$Distro = 'Ubuntu',
  [string]$WslUser = 'dev',
  [string]$WorkerDir = '/home/dev/Gumi',
  [string]$WorkerName = 'gumi-windows',
  [int]$InitialDelaySec = 20,
  [int]$WslReadyTimeoutSec = 180
)

$ErrorActionPreference = 'Continue'
$logDir = Join-Path $env:USERPROFILE '.gumi\logs'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$logFile = Join-Path $logDir 'cursor-worker-windows.log'

function Write-Log([string]$Message) {
  $line = '{0} {1}' -f (Get-Date -Format 'o'), $Message
  Add-Content -Path $logFile -Value $line
  Write-Host $line
}

Write-Log "launcher start distro=$Distro user=$WslUser delay=${InitialDelaySec}s"
if ($InitialDelaySec -gt 0) {
  Start-Sleep -Seconds $InitialDelaySec
}

# Wait until WSL can run a trivial command (boot can lag after auto logon).
$deadline = (Get-Date).AddSeconds($WslReadyTimeoutSec)
$ready = $false
while ((Get-Date) -lt $deadline) {
  & wsl.exe -d $Distro -u $WslUser --exec /bin/true 2>$null
  if ($LASTEXITCODE -eq 0) {
    $ready = $true
    break
  }
  Write-Log "waiting for WSL ready (last exit=$LASTEXITCODE)"
  Start-Sleep -Seconds 3
}
if (-not $ready) {
  Write-Log "ERROR: WSL not ready within ${WslReadyTimeoutSec}s"
  exit 1
}
Write-Log 'WSL ready'

$scriptPath = "$WorkerDir/scripts/windows/start-cursor-worker.sh"
$bash = @"
export PATH=`$HOME/.local/bin:`$PATH
export CURSOR_WORKER_DIR='$WorkerDir'
export CURSOR_WORKER_NAME='$WorkerName'
chmod +x '$scriptPath' 2>/dev/null || true
exec '$scriptPath'
"@

Write-Log "starting worker watchdog name=$WorkerName dir=$WorkerDir"
# Keep this PowerShell process alive with the WSL session so Task Scheduler
# treats the job as still running and can restart it on failure.
& wsl.exe -d $Distro -u $WslUser -- bash -lc $bash
$code = $LASTEXITCODE
Write-Log "worker/wsl exited code=$code"
exit $code
