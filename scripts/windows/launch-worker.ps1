#Requires -Version 5.1
<#
.SYNOPSIS
  Wake WSL and ensure the Cursor agent systemd user service is running.

.DESCRIPTION
  The worker runs under systemd inside WSL (cursor-agent-worker.service), so it
  survives Windows PowerShell / console teardown. This launcher only wakes WSL,
  starts the unit, and polls so the VM stays up.

  Logs to %USERPROFILE%\.gumi\logs\cursor-worker-windows.log
#>
[CmdletBinding()]
param(
  [string]$Distro = 'Ubuntu',
  [string]$WslUser = 'dev',
  [string]$WorkerDir = '/home/dev/Gumi',
  [string]$WorkerName = 'gumi-windows',
  [int]$InitialDelaySec = 15,
  [int]$WslReadyTimeoutSec = 180,
  [int]$PollSec = 30
)

$ErrorActionPreference = 'Continue'
$logDir = Join-Path $env:USERPROFILE '.gumi\logs'
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$logFile = Join-Path $logDir 'cursor-worker-windows.log'
$unit = 'cursor-agent-worker.service'
$unitSrc = "$WorkerDir/scripts/windows/cursor-agent-worker.service"
$unitDst = '/home/dev/.config/systemd/user/cursor-agent-worker.service'

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

function Ensure-WorkerService {
  # Single-line bash -c payload (no fragile multiline / PATH quoting).
  $cmd = "mkdir -p /home/dev/.gumi/logs /home/dev/.config/systemd/user && chmod +x '$WorkerDir/scripts/windows/start-cursor-worker.sh' && cp -f '$unitSrc' '$unitDst' && systemctl --user daemon-reload && systemctl --user enable '$unit' && systemctl --user restart '$unit' && systemctl --user is-active '$unit'"
  $out = & wsl.exe -d $Distro -u $WslUser --exec /bin/bash -c $cmd 2>&1
  $code = $LASTEXITCODE
  if ($out) { Write-Log ("systemctl: " + ($out | Out-String).Trim()) }
  return $code
}

function Get-ServiceState {
  $out = & wsl.exe -d $Distro -u $WslUser --exec /bin/bash -c "systemctl --user is-active $unit" 2>$null
  return ("$out").Trim()
}

Write-Log "launcher start distro=$Distro user=$WslUser delay=${InitialDelaySec}s (systemd-backed)"
if ($InitialDelaySec -gt 0) {
  Start-Sleep -Seconds $InitialDelaySec
}

while ($true) {
  if (-not (Wait-WslReady)) {
    Write-Log "ERROR: WSL not ready within ${WslReadyTimeoutSec}s; retrying"
    Start-Sleep -Seconds 5
    continue
  }

  $code = Ensure-WorkerService
  $state = Get-ServiceState
  Write-Log "ensure service exit=$code state=$state"

  while ($true) {
    Start-Sleep -Seconds $PollSec
    & wsl.exe -d $Distro -u $WslUser --exec /bin/true 2>$null
    if ($LASTEXITCODE -ne 0) {
      Write-Log 'WSL became unavailable; re-entering ready wait'
      break
    }
    $state = Get-ServiceState
    if ($state -ne 'active') {
      Write-Log "service state=$state; restarting"
      break
    }
  }
}
