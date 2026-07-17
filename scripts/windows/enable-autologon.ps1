#Requires -Version 5.1
#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Enable Windows auto logon for the local self-hosted agent user.

.DESCRIPTION
  Configures Winlogon so Windows signs in as the given local user after boot
  (and again after sign-out when -Force is set). Needed so the CursorAgentWorker
  Scheduled Task (At logon) can start without someone at the keyboard.

  WARNING: Stores DefaultPassword in the Winlogon registry in plain text.
  Prefer a dedicated local account with a blank or unique password on a
  physically controlled machine.

.PARAMETER UserName
  Local account to auto-logon. Default: dev

.PARAMETER Password
  Account password. Omit or pass empty string for blank-password accounts
  (Password required = No).

.PARAMETER Force
  Also set ForceAutoLogon=1 so Windows logs back in after sign-out.

.PARAMETER Disable
  Turn auto logon off and clear stored DefaultPassword.
#>
[CmdletBinding(DefaultParameterSetName = 'Enable')]
param(
  [Parameter(ParameterSetName = 'Enable')]
  [string]$UserName = 'dev',

  [Parameter(ParameterSetName = 'Enable')]
  [string]$Password = '',

  [Parameter(ParameterSetName = 'Enable')]
  [switch]$Force,

  [Parameter(ParameterSetName = 'Disable')]
  [switch]$Disable
)

$ErrorActionPreference = 'Stop'
$path = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon'

if ($Disable) {
  Set-ItemProperty -Path $path -Name 'AutoAdminLogon' -Value '0' -Type String
  Set-ItemProperty -Path $path -Name 'ForceAutoLogon' -Value '0' -Type String
  Remove-ItemProperty -Path $path -Name 'DefaultPassword' -ErrorAction SilentlyContinue
  Remove-ItemProperty -Path $path -Name 'AutoLogonCount' -ErrorAction SilentlyContinue
  Write-Host 'Auto logon disabled.'
  exit 0
}

$domain = $env:COMPUTERNAME
Set-ItemProperty -Path $path -Name 'AutoAdminLogon' -Value '1' -Type String
Set-ItemProperty -Path $path -Name 'DefaultUserName' -Value $UserName -Type String
Set-ItemProperty -Path $path -Name 'DefaultDomainName' -Value $domain -Type String
Set-ItemProperty -Path $path -Name 'DefaultPassword' -Value $Password -Type String
Set-ItemProperty -Path $path -Name 'ForceAutoLogon' -Value ($(if ($Force) { '1' } else { '0' })) -Type String
Remove-ItemProperty -Path $path -Name 'AutoLogonCount' -ErrorAction SilentlyContinue

# Fewer blockers on unattended reboot
New-Item -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Personalization' -Force | Out-Null
Set-ItemProperty -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Personalization' -Name 'NoLockScreen' -Value 1 -Type DWord
Set-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System' -Name 'DisableCAD' -Value 1 -Type DWord -ErrorAction SilentlyContinue

Write-Host "Auto logon enabled for $domain\$UserName"
Write-Host "  AutoAdminLogon  = $((Get-ItemProperty $path).AutoAdminLogon)"
Write-Host "  ForceAutoLogon  = $((Get-ItemProperty $path).ForceAutoLogon)"
Write-Host "  Password stored = $(-not [string]::IsNullOrEmpty($Password))"
Write-Host ''
Write-Host 'Reboot to verify: Windows should sign in without a password prompt,'
Write-Host 'then Scheduled Task CursorAgentWorker should start the agent worker.'
