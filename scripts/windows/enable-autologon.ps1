#Requires -Version 5.1
#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Enable Windows auto logon for the local self-hosted agent user.

.DESCRIPTION
  Configures Winlogon so Windows signs in as the given local user after boot
  (and again after sign-out when -Force is set). Needed so Startup / the
  CursorAgentWorker Scheduled Task can start without someone at the keyboard.

  WARNING: Stores DefaultPassword in the Winlogon registry in plain text.
  Prefer a dedicated local account on a physically controlled machine.

  Tip: Use DefaultDomainName "." for local accounts so the sign-in screen does
  not show both COMPUTER\user and .\user as two tiles with the same name.

.PARAMETER UserName
  Local account to auto-logon. Default: dev

.PARAMETER Password
  Account password (required for accounts that have a password set).

.PARAMETER Domain
  Logon domain. Use "." for a local account (recommended). Default: .

.PARAMETER Force
  Also set ForceAutoLogon=1 so Windows logs back in after sign-out.

.PARAMETER HideOtherUsers
  Hide other local users (afnan, WsiAccount, Administrator) from the sign-in list.

.PARAMETER Disable
  Turn auto logon off and clear stored DefaultPassword.
#>
[CmdletBinding(DefaultParameterSetName = 'Enable')]
param(
  [Parameter(ParameterSetName = 'Enable')]
  [string]$UserName = 'dev',

  [Parameter(ParameterSetName = 'Enable', Mandatory = $true)]
  [string]$Password,

  [Parameter(ParameterSetName = 'Enable')]
  [string]$Domain = '.',

  [Parameter(ParameterSetName = 'Enable')]
  [switch]$Force,

  [Parameter(ParameterSetName = 'Enable')]
  [switch]$HideOtherUsers,

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

Set-ItemProperty -Path $path -Name 'AutoAdminLogon' -Value '1' -Type String
Set-ItemProperty -Path $path -Name 'DefaultUserName' -Value $UserName -Type String
Set-ItemProperty -Path $path -Name 'DefaultDomainName' -Value $Domain -Type String
Set-ItemProperty -Path $path -Name 'DefaultPassword' -Value $Password -Type String
Set-ItemProperty -Path $path -Name 'ForceAutoLogon' -Value ($(if ($Force) { '1' } else { '0' })) -Type String
Remove-ItemProperty -Path $path -Name 'AutoLogonCount' -ErrorAction SilentlyContinue

New-Item -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Personalization' -Force | Out-Null
Set-ItemProperty -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Personalization' -Name 'NoLockScreen' -Value 1 -Type DWord
Set-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System' -Name 'DisableCAD' -Value 1 -Type DWord -ErrorAction SilentlyContinue

$logonUi = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\LogonUI'
Set-ItemProperty -Path $logonUi -Name 'LastLoggedOnUser' -Value ".\$UserName" -ErrorAction SilentlyContinue
Set-ItemProperty -Path $logonUi -Name 'LastLoggedOnSAMUser' -Value ".\$UserName" -ErrorAction SilentlyContinue

if ($HideOtherUsers) {
  $userList = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon\SpecialAccounts\UserList'
  New-Item -Path $userList -Force | Out-Null
  foreach ($hidden in @('afnan', 'WsiAccount', 'Administrator', 'Guest')) {
    New-ItemProperty -Path $userList -Name $hidden -Value 0 -PropertyType DWord -Force | Out-Null
  }
  New-ItemProperty -Path $userList -Name $UserName -Value 1 -PropertyType DWord -Force | Out-Null
}

Write-Host "Auto logon enabled for $Domain\$UserName"
Write-Host "  AutoAdminLogon  = $((Get-ItemProperty $path).AutoAdminLogon)"
Write-Host "  ForceAutoLogon  = $((Get-ItemProperty $path).ForceAutoLogon)"
Write-Host "  Password length = $($Password.Length)"
Write-Host ''
Write-Host 'Reboot to verify: Windows should sign in without a password prompt,'
Write-Host 'then Startup / CursorAgentWorker should start the agent worker.'
