#Requires -Version 5.1
<#
.SYNOPSIS
  Install Windows + WSL autostart for the Cursor agent worker.

.DESCRIPTION
  - Copies launchers to %USERPROFILE%\.gumi\bin
  - Installs a systemd user unit inside WSL (survives console close)
  - Registers ONE Scheduled Task at logon (Startup folder entry removed to
    avoid dual-launch races)
  - Writes %USERPROFILE%\.wslconfig with vmIdleTimeout=-1
#>
[CmdletBinding()]
param(
  [string]$Distro = 'Ubuntu',
  [string]$WslUser = 'dev',
  [string]$WorkerDir = '',
  [string]$WorkerName = 'gumi-windows',
  [int]$LogonDelaySec = 15,
  [switch]$SkipPowerSettings,
  [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$TaskName = 'CursorAgentWorker'
$StartupName = 'CursorAgentWorker.cmd'

if (-not $WorkerDir) { $WorkerDir = "/home/$WslUser/Gumi" }

$NativeBin = Join-Path $env:USERPROFILE '.gumi\bin'
New-Item -ItemType Directory -Force -Path $NativeBin | Out-Null
$SourceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $SourceDir) { $SourceDir = (Get-Location).Path }
foreach ($name in @(
    'launch-worker.ps1',
    'disable-sleep.ps1',
    'enable-autologon.ps1',
    'install-worker-autostart.ps1'
  )) {
  $src = Join-Path $SourceDir $name
  if (Test-Path $src) { Copy-Item -Force $src (Join-Path $NativeBin $name) }
}
$LaunchPs1 = Join-Path $NativeBin 'launch-worker.ps1'
$StartupDir = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Startup'
$StartupCmd = Join-Path $StartupDir $StartupName

if ($Uninstall) {
  Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
  Remove-Item -Force -ErrorAction SilentlyContinue $StartupCmd
  & wsl.exe -d $Distro -u $WslUser --exec /bin/bash -c "systemctl --user disable --now cursor-agent-worker.service 2>/dev/null || true" | Out-Null
  Write-Host "Removed '$TaskName', Startup entry, and stopped systemd unit (if present)."
  exit 0
}

if (-not (Test-Path $LaunchPs1)) { throw "Missing launcher: $LaunchPs1" }

$linuxScript = "$WorkerDir/scripts/windows/start-cursor-worker.sh"
$exists = (wsl.exe -d $Distro -u $WslUser -- bash -c "test -f '$linuxScript' && echo yes || echo no").Trim()
if ($exists -ne 'yes') { throw "Worker script not found in WSL at $linuxScript" }

# Install systemd user unit now.
$unitSrc = "$WorkerDir/scripts/windows/cursor-agent-worker.service"
& wsl.exe -d $Distro -u $WslUser --exec /bin/bash -c @"
set -e
mkdir -p `$HOME/.config/systemd/user `$HOME/.gumi/logs
chmod +x '$linuxScript'
cp -f '$unitSrc' `$HOME/.config/systemd/user/cursor-agent-worker.service
systemctl --user daemon-reload
systemctl --user enable cursor-agent-worker.service
systemctl --user restart cursor-agent-worker.service
systemctl --user --no-pager --full status cursor-agent-worker.service | head -n 25
"@

if (-not $SkipPowerSettings) {
  $powerScript = Join-Path $NativeBin 'disable-sleep.ps1'
  if (Test-Path $powerScript) {
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $powerScript -LidDoNothing
  }
}

# Keep WSL from shutting down when the last interactive terminal closes.
$wslConfig = Join-Path $env:USERPROFILE '.wslconfig'
@"
[wsl2]
vmIdleTimeout=-1
"@ | Set-Content -Path $wslConfig -Encoding ASCII
Write-Host "Wrote $wslConfig (vmIdleTimeout=-1)"

# Remove Startup-folder duplicate (Task alone is enough).
Remove-Item -Force -ErrorAction SilentlyContinue $StartupCmd

$arg = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$LaunchPs1`" -Distro $Distro -WslUser $WslUser -WorkerDir $WorkerDir -WorkerName $WorkerName -InitialDelaySec $LogonDelaySec"
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $arg
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
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
  -Description 'Wake WSL + ensure Cursor agent systemd user service is running.' `
  -Force | Out-Null

Write-Host "Installed scheduled task '$TaskName' (AtLogOn + ${LogonDelaySec}s)."
Write-Host "  systemd unit: cursor-agent-worker.service (inside WSL)"
Write-Host "  Windows log:  %USERPROFILE%\.gumi\logs\cursor-worker-windows.log"
Write-Host "  WSL log:      ~/.gumi/logs/cursor-worker.log"
Write-Host ''
Write-Host 'Check now:'
Write-Host '  wsl -d Ubuntu -u dev -- systemctl --user status cursor-agent-worker.service'
