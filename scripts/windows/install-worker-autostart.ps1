#Requires -Version 5.1
<#
.SYNOPSIS
  Install a Windows Scheduled Task that keeps the Cursor agent worker running in WSL.

.DESCRIPTION
  Creates (or replaces) task "CursorAgentWorker" which, at user logon:
    1. Starts WSL Ubuntu
    2. Runs scripts/windows/start-cursor-worker.sh (restart loop)

  Also applies never-sleep power settings unless -SkipPowerSettings is set.

.PARAMETER Distro
  WSL distro name. Default: Ubuntu

.PARAMETER WslUser
  Linux username inside the distro. Default: current WSL user if detectable, else "dev"

.PARAMETER WorkerDir
  Absolute Linux path to the repo the worker should serve.
  Default: /home/<WslUser>/Gumi

.PARAMETER WorkerName
  Friendly My Machines worker name shown in cursor.com/agents.

.PARAMETER SkipPowerSettings
  Do not change Windows sleep settings.

.PARAMETER Uninstall
  Remove the Scheduled Task instead of installing it.
#>
[CmdletBinding()]
param(
  [string]$Distro = 'Ubuntu',
  [string]$WslUser = '',
  [string]$WorkerDir = '',
  [string]$WorkerName = 'gumi-windows',
  [switch]$SkipPowerSettings,
  [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$TaskName = 'CursorAgentWorker'

if ($Uninstall) {
  Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
  Write-Host "Removed scheduled task '$TaskName' (if it existed)."
  exit 0
}

if (-not $WslUser) {
  try {
    $WslUser = (wsl.exe -d $Distro -- bash -lc 'whoami').Trim()
  } catch {
    $WslUser = 'dev'
  }
}
if (-not $WorkerDir) {
  $WorkerDir = "/home/$WslUser/Gumi"
}

$ScriptPath = "$WorkerDir/scripts/windows/start-cursor-worker.sh"
$exists = (wsl.exe -d $Distro -u $WslUser -- bash -lc "test -f '$ScriptPath' && echo yes || echo no").Trim()
if ($exists -ne 'yes') {
  throw "Worker script not found in WSL at $ScriptPath. Clone/pull the repo first."
}

# Ensure executable bit inside WSL.
wsl.exe -d $Distro -u $WslUser -- bash -lc "chmod +x '$ScriptPath'" | Out-Null

if (-not $SkipPowerSettings) {
  $here = Split-Path -Parent $MyInvocation.MyCommand.Path
  $powerScript = Join-Path $here 'disable-sleep.ps1'
  if (Test-Path $powerScript) {
    Write-Host 'Applying never-sleep power settings...'
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $powerScript -LidDoNothing
  } else {
    # Running via UNC/WSL path — apply inline.
    powercfg /change standby-timeout-ac 0 | Out-Null
    powercfg /change standby-timeout-dc 0 | Out-Null
    powercfg /change hibernate-timeout-ac 0 | Out-Null
    powercfg /change hibernate-timeout-dc 0 | Out-Null
    powercfg /change monitor-timeout-ac 0 | Out-Null
    powercfg /change monitor-timeout-dc 0 | Out-Null
    powercfg /SETACTIVE SCHEME_CURRENT | Out-Null
  }
}

$bashCmd = "export CURSOR_WORKER_DIR='$WorkerDir' CURSOR_WORKER_NAME='$WorkerName'; exec '$ScriptPath'"
# -WindowStyle Hidden is not available on wsl.exe; task runs whether UI is shown.
$arg = "-d $Distro -u $WslUser -- bash -lc `"$bashCmd`""

$action = New-ScheduledTaskAction -Execute 'wsl.exe' -Argument $arg
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$settings = New-ScheduledTaskSettingsSet `
  -AllowStartIfOnBatteries `
  -DontStopIfGoingOnBatteries `
  -StartWhenAvailable `
  -ExecutionTimeLimit ([TimeSpan]::Zero) `
  -RestartCount 999 `
  -RestartInterval (New-TimeSpan -Minutes 1) `
  -MultipleInstances IgnoreNew

# Hidden / no time limit keeps the long-lived WSL worker alive.
$settings.DisallowStartIfOnBatteries = $false
$settings.StopIfGoingOnBatteries = $false

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask `
  -TaskName $TaskName `
  -Action $action `
  -Trigger $trigger `
  -Settings $settings `
  -Description "Cursor self-hosted agent worker (WSL $Distro). Restarts on failure." `
  -Force | Out-Null

Write-Host ""
Write-Host "Installed scheduled task '$TaskName'."
Write-Host "  Distro:      $Distro"
Write-Host "  WSL user:    $WslUser"
Write-Host "  Worker dir:  $WorkerDir"
Write-Host "  Worker name: $WorkerName"
Write-Host "  Trigger:     At logon for $env:USERDOMAIN\$env:USERNAME"
Write-Host ""
Write-Host "Start now with:"
Write-Host "  Start-ScheduledTask -TaskName '$TaskName'"
Write-Host "Logs (inside WSL):"
Write-Host "  ~/.gumi/logs/cursor-worker.log"
Write-Host "Remove later with:"
Write-Host "  powershell -File install-worker-autostart.ps1 -Uninstall"
