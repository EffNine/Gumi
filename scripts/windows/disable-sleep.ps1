#Requires -Version 5.1
<#
.SYNOPSIS
  Keep this Windows host awake for a self-hosted Cursor agent worker.

.DESCRIPTION
  Sets the active power plan so the PC does not sleep or hibernate on idle
  (AC and battery). Optionally disables hibernate entirely (needs elevation).

.PARAMETER DisableHibernate
  Run `powercfg /hibernate off` (requires Administrator). Removes hibernate
  from the power menu and frees the hibernation file.

.PARAMETER LidDoNothing
  Set lid close action to Do Nothing on AC and battery (laptops).
#>
[CmdletBinding()]
param(
  [switch]$DisableHibernate,
  [switch]$LidDoNothing
)

$ErrorActionPreference = 'Stop'

function Set-NeverSleep {
  Write-Host 'Setting sleep/hibernate idle timeouts to Never (AC + DC)...'
  powercfg /change standby-timeout-ac 0 | Out-Null
  powercfg /change standby-timeout-dc 0 | Out-Null
  powercfg /change hibernate-timeout-ac 0 | Out-Null
  powercfg /change hibernate-timeout-dc 0 | Out-Null
  powercfg /change monitor-timeout-ac 0 | Out-Null
  powercfg /change monitor-timeout-dc 0 | Out-Null

  powercfg /SETACVALUEINDEX SCHEME_CURRENT SUB_SLEEP HYBRIDSLEEP 0 | Out-Null
  powercfg /SETDCVALUEINDEX SCHEME_CURRENT SUB_SLEEP HYBRIDSLEEP 0 | Out-Null

  if ($LidDoNothing) {
    Write-Host 'Setting lid close action to Do Nothing...'
    powercfg /SETACVALUEINDEX SCHEME_CURRENT SUB_BUTTONS LIDACTION 0 2>$null
    powercfg /SETDCVALUEINDEX SCHEME_CURRENT SUB_BUTTONS LIDACTION 0 2>$null
  }

  powercfg /SETACTIVE SCHEME_CURRENT | Out-Null
  Write-Host 'Active power scheme updated.'
}

function Disable-HibernateFile {
  $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).
    IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
  if (-not $isAdmin) {
    Write-Warning 'DisableHibernate requires an elevated PowerShell. Skipping.'
    return
  }
  Write-Host 'Disabling hibernate...'
  powercfg /hibernate off
}

Set-NeverSleep
if ($DisableHibernate) {
  Disable-HibernateFile
}

Write-Host ''
Write-Host 'Current sleep timeouts:'
powercfg /query SCHEME_CURRENT SUB_SLEEP STANDBYIDLE | Select-String 'Current (AC|DC)'
Write-Host ''
Write-Host 'Available sleep states:'
powercfg /a
