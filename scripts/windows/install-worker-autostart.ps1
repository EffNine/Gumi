#Requires -Version 5.1
<#
.SYNOPSIS
  Install Windows autostart for the Cursor agent worker in WSL.

.DESCRIPTION
  Installs two redundant start mechanisms (both are single-instance safe):
    1. Startup-folder shortcut → launch-worker.ps1 (most reliable after interactive logon)
    2. Scheduled Task "CursorAgentWorker" with a post-logon delay

.PARAMETER Distro
  WSL distro name. Default: Ubuntu

.PARAMETER WslUser
  Linux username inside the distro. Default: dev

.PARAMETER WorkerDir
  Absolute Linux path to the repo. Default: /home/<WslUser>/Gumi

.PARAMETER WorkerName
  Friendly My Machines worker name.

.PARAMETER LogonDelaySec
  Seconds to wait after logon before starting WSL/worker. Default: 25

.PARAMETER SkipPowerSettings
  Do not change Windows sleep settings.

.PARAMETER Uninstall
  Remove Startup entry and Scheduled Task.
#>
[CmdletBinding()]
param(
  [string]$Distro = 'Ubuntu',
  [string]$WslUser = 'dev',
  [string]$WorkerDir = '',
  [string]$WorkerName = 'gumi-windows',
  [int]$LogonDelaySec = 25,
  [switch]$SkipPowerSettings,
  [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$TaskName = 'CursorAgentWorker'
$StartupName = 'CursorAgentWorker.cmd'

if (-not $WorkerDir) {
  $WorkerDir = "/home/$WslUser/Gumi"
}

# Always install the Windows launcher onto a native NTFS path. Startup/Task
# cannot depend on \\wsl.localhost\... because that share is not available
# until WSL has already started (chicken-and-egg after auto logon).
$NativeBin = Join-Path $env:USERPROFILE '.gumi\bin'
New-Item -ItemType Directory -Force -Path $NativeBin | Out-Null
$SourceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $SourceDir) { $SourceDir = (Get-Location).Path }
foreach ($name in @('launch-worker.ps1', 'disable-sleep.ps1', 'enable-autologon.ps1', 'install-worker-autostart.ps1')) {
  $src = Join-Path $SourceDir $name
  if (Test-Path $src) {
    Copy-Item -Force $src (Join-Path $NativeBin $name)
  }
}
$LaunchPs1 = Join-Path $NativeBin 'launch-worker.ps1'
$StartupDir = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Startup'
$StartupCmd = Join-Path $StartupDir $StartupName

if ($Uninstall) {
  Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
  Remove-Item -Force -ErrorAction SilentlyContinue $StartupCmd
  Write-Host "Removed '$TaskName' task and Startup entry (if present)."
  exit 0
}

if (-not (Test-Path $LaunchPs1)) {
  throw "Missing launcher: $LaunchPs1"
}

# Ensure Linux scripts exist / are executable.
$linuxScript = "$WorkerDir/scripts/windows/start-cursor-worker.sh"
$exists = (wsl.exe -d $Distro -u $WslUser -- bash -lc "test -f '$linuxScript' && echo yes || echo no").Trim()
if ($exists -ne 'yes') {
  throw "Worker script not found in WSL at $linuxScript"
}
wsl.exe -d $Distro -u $WslUser -- bash -lc "chmod +x '$linuxScript' '$WorkerDir/scripts/windows/'*.sh 2>/dev/null || true" | Out-Null

if (-not $SkipPowerSettings) {
  $powerScript = Join-Path $ScriptDir 'disable-sleep.ps1'
  if (Test-Path $powerScript) {
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $powerScript -LidDoNothing
  } else {
    powercfg /change standby-timeout-ac 0 | Out-Null
    powercfg /change standby-timeout-dc 0 | Out-Null
    powercfg /change hibernate-timeout-ac 0 | Out-Null
    powercfg /change hibernate-timeout-dc 0 | Out-Null
    powercfg /SETACTIVE SCHEME_CURRENT | Out-Null
  }
}

# --- Startup folder (runs in the interactive user session after logon) ---
New-Item -ItemType Directory -Force -Path $StartupDir | Out-Null
$cmd = @"
@echo off
rem Autostart Cursor self-hosted agent worker via WSL
powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "$LaunchPs1" -Distro $Distro -WslUser $WslUser -WorkerDir $WorkerDir -WorkerName $WorkerName -InitialDelaySec $LogonDelaySec
"@
Set-Content -Path $StartupCmd -Value $cmd -Encoding ASCII
Write-Host "Installed Startup entry: $StartupCmd"

# --- Scheduled Task backup (At logon + delay) ---
$arg = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$LaunchPs1`" -Distro $Distro -WslUser $WslUser -WorkerDir $WorkerDir -WorkerName $WorkerName -InitialDelaySec $LogonDelaySec"
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $arg
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
# Delay so the desktop / WSL networking are up after auto logon.
try { $trigger.Delay = 'PT{0}S' -f $LogonDelaySec } catch { }

$settings = New-ScheduledTaskSettingsSet `
  -AllowStartIfOnBatteries `
  -DontStopIfGoingOnBatteries `
  -StartWhenAvailable `
  -ExecutionTimeLimit ([TimeSpan]::Zero) `
  -RestartCount 999 `
  -RestartInterval (New-TimeSpan -Minutes 1) `
  -MultipleInstances IgnoreNew
$settings.DisallowStartIfOnBatteries = $false
$settings.StopIfGoingOnBatteries = $false

$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask `
  -TaskName $TaskName `
  -Action $action `
  -Trigger $trigger `
  -Settings $settings `
  -Principal $principal `
  -Description 'Cursor self-hosted agent worker (WSL). Startup-folder + delayed AtLogOn.' `
  -Force | Out-Null

Write-Host "Installed scheduled task '$TaskName' (AtLogOn + ${LogonDelaySec}s delay)."
Write-Host "  Distro:      $Distro"
Write-Host "  WSL user:    $WslUser"
Write-Host "  Worker dir:  $WorkerDir"
Write-Host "  Worker name: $WorkerName"
Write-Host ''
Write-Host 'Windows log:  %USERPROFILE%\.gumi\logs\cursor-worker-windows.log'
Write-Host 'WSL log:      ~/.gumi/logs/cursor-worker.log'
Write-Host ''
Write-Host 'Test now (without reboot):'
Write-Host "  Start-ScheduledTask -TaskName '$TaskName'"
Write-Host '  OR open a new logon / reboot (with auto logon configured).'
Write-Host ''
Write-Host 'Remove with:'
Write-Host '  powershell -File install-worker-autostart.ps1 -Uninstall'
