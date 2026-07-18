#Requires -Version 5.1
<#
.SYNOPSIS
  Bootstrap Gumi development on a Windows PC (WSL2 + Ubuntu).

.DESCRIPTION
  1. Ensures WSL2 + Ubuntu are installed (may reboot / require elevation).
  2. Clones or updates the Gumi repo inside WSL at ~/Gumi (override with -RepoDir).
  3. Runs scripts/windows/setup-dev-env.sh to install Go 1.25+, Node 22+, and build.

  GPU inference (LM Studio / Ollama) stays on Windows so the RTX GPU is used.
  This script does not install LM Studio; it prints those steps at the end.

.PARAMETER Distro
  WSL distro name. Default: Ubuntu

.PARAMETER RepoUrl
  Git remote to clone. Default: https://github.com/EffNine/Gumi.git

.PARAMETER RepoDir
  Absolute Linux path for the checkout. Default: /home/<wsl-user>/Gumi

.PARAMETER SkipBuild
  Pass SKIP_BUILD=1 to the WSL setup script.

.PARAMETER InstallWorkerAutostart
  Also register the CursorAgentWorker scheduled task after setup.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\scripts\windows\setup-dev-env.ps1
#>
[CmdletBinding()]
param(
  [string]$Distro = 'Ubuntu',
  [string]$RepoUrl = 'https://github.com/EffNine/Gumi.git',
  [string]$RepoDir = '',
  [switch]$SkipBuild,
  [switch]$InstallWorkerAutostart
)

$ErrorActionPreference = 'Stop'

function Test-IsAdmin {
  $id = [Security.Principal.WindowsIdentity]::GetCurrent()
  $p = New-Object Security.Principal.WindowsPrincipal($id)
  return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Ensure-Wsl {
  if (Get-Command wsl.exe -ErrorAction SilentlyContinue) {
    $list = & wsl.exe -l -q 2>$null
    if ($LASTEXITCODE -eq 0 -and $list) {
      $names = @($list | ForEach-Object { $_.ToString().Trim() } | Where-Object { $_ })
      if ($names -contains $Distro -or ($names | Where-Object { $_ -like "$Distro*" })) {
        Write-Host "WSL distro '$Distro' is available."
        return
      }
    }
  }

  Write-Host "Installing WSL + $Distro (Administrator required)..."
  if (-not (Test-IsAdmin)) {
    throw @"
WSL/$Distro is not ready and this shell is not elevated.
Re-run from an Administrator PowerShell:
  wsl --install -d $Distro
Then reboot if prompted, open Ubuntu once to finish user setup, and re-run this script.
"@
  }
  & wsl.exe --install -d $Distro
  Write-Host @"

If Windows asked you to reboot, do that now, open '$Distro' once to create your Linux user,
then re-run this script.
"@
  exit 0
}

function Get-WslUser {
  $u = (& wsl.exe -d $Distro -- bash -lc 'whoami').Trim()
  if (-not $u) { throw "Could not detect WSL user for distro '$Distro'." }
  return $u
}

Ensure-Wsl

$wslUser = Get-WslUser
if (-not $RepoDir) {
  $RepoDir = "/home/$wslUser/Gumi"
}

Write-Host "WSL user:  $wslUser"
Write-Host "Repo dir:  $RepoDir"
Write-Host "Repo URL:  $RepoUrl"

# Ensure git exists, then clone or pull.
$cloneBash = @"
set -euo pipefail
if ! command -v git >/dev/null 2>&1; then
  sudo apt-get update -y
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y git
fi
if [[ -d '$RepoDir/.git' ]]; then
  echo 'Repo exists; fetching...'
  git -C '$RepoDir' fetch --all --prune
else
  mkdir -p "\$(dirname '$RepoDir')"
  git clone '$RepoUrl' '$RepoDir'
fi
chmod +x '$RepoDir/scripts/windows/setup-dev-env.sh'
"@

& wsl.exe -d $Distro -u $wslUser -- bash -lc $cloneBash
if ($LASTEXITCODE -ne 0) { throw "Failed to clone/update repo in WSL." }

$envExports = "export GUMI_REPO_DIR='$RepoDir' GUMI_REPO_URL='$RepoUrl' SKIP_CLONE=1"
if ($SkipBuild) { $envExports += ' SKIP_BUILD=1' }

$setupBash = "$envExports; bash '$RepoDir/scripts/windows/setup-dev-env.sh'"
& wsl.exe -d $Distro -u $wslUser -- bash -lc $setupBash
if ($LASTEXITCODE -ne 0) { throw "setup-dev-env.sh failed inside WSL." }

if ($InstallWorkerAutostart) {
  $here = Split-Path -Parent $MyInvocation.MyCommand.Path
  $workerScript = Join-Path $here 'install-worker-autostart.ps1'
  if (-not (Test-Path $workerScript)) {
    # Prefer the copy inside the WSL checkout (UNC path).
    $unc = "\\wsl$\$Distro$RepoDir\scripts\windows\install-worker-autostart.ps1"
    if (Test-Path $unc) { $workerScript = $unc }
  }
  if (Test-Path $workerScript) {
    Write-Host "Installing Cursor worker autostart..."
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $workerScript `
      -Distro $Distro -WslUser $wslUser -WorkerDir $RepoDir
  } else {
    Write-Warning "install-worker-autostart.ps1 not found; skipping."
  }
}

Write-Host @"

────────────────────────────────────────────────────────
Windows side next steps (GPU):

1. Install LM Studio (https://lmstudio.ai) or Ollama for Windows.
2. Load a model and start the local server (LM Studio default :1234).
3. In WSL:
     cd $RepoDir
     export GUMI_PROVIDER_DEFAULT=lmstudio
     export GUMI_LMSTUDIO_URL=http://127.0.0.1:1234/v1
     ./gumi start

Dashboard: http://127.0.0.1:8788
API:       http://127.0.0.1:8787/v1  (Bearer gumi-local)

Open the repo in Cursor via WSL:
  cursor --folder-uri vscode-remote://wsl+$Distro$RepoDir
────────────────────────────────────────────────────────
"@
