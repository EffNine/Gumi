# Windows development setup (RTX / WSL2)

Gumi development on Windows is easiest with **WSL2 + Ubuntu**. Build tools and
the Go/Node toolchain run in Linux; **LM Studio or Ollama run on Windows** so
they can use your NVIDIA GPU (e.g. RTX 5070).

## One-command bootstrap

From **PowerShell** on the Windows host (Administrator only needed the first
time if WSL is not installed yet):

```powershell
# If you do not have the repo yet:
wsl --install -d Ubuntu   # reboot if prompted, open Ubuntu once
wsl -d Ubuntu -- bash -lc "sudo apt-get update -y && sudo apt-get install -y git && git clone https://github.com/EffNine/Gumi.git ~/Gumi"

# Then run the Windows bootstrap (installs Go/Node, builds Gumi):
powershell -ExecutionPolicy Bypass -File \\wsl$\Ubuntu\home\$env:USERNAME\Gumi\scripts\windows\setup-dev-env.ps1
```

If your WSL username is not the same as `$env:USERNAME` (e.g. `dev`), pass the
Linux path explicitly:

```powershell
powershell -ExecutionPolicy Bypass -File \\wsl$\Ubuntu\home\dev\Gumi\scripts\windows\setup-dev-env.ps1 -RepoDir /home/dev/Gumi
```

### Or from inside WSL only

```bash
curl -fsSL https://raw.githubusercontent.com/EffNine/Gumi/main/scripts/windows/setup-dev-env.sh | bash
# or, with a checkout already present:
bash scripts/windows/setup-dev-env.sh
```

What the script installs:

| Tool | Version |
|------|---------|
| Go | 1.25+ |
| Node.js / npm | 22+ |
| make, git, build-essential | apt |
| Gumi binary | `make build` → `~/Gumi/gumi` |

## Day-to-day workflow

```bash
cd ~/Gumi
make build && ./gumi start          # API :8787, dashboard :8788

# Hot-reload dashboard (separate terminal; runtime must be up):
cd ~/Gumi/dashboard && npm run dev

# Tests
cd ~/Gumi/runtime && go test ./... && go vet ./...
```

Open the tree in Cursor with the **WSL** remote (`File → Open Folder` from the
WSL side, or `cursor --folder-uri vscode-remote://wsl+Ubuntu/home/<user>/Gumi`).

## Wire up the GPU (Windows host)

1. Install [LM Studio](https://lmstudio.ai) (recommended) or [Ollama](https://ollama.com) on Windows.
2. Load a model and start the local server (LM Studio defaults to port `1234`).
3. From WSL:

```bash
export GUMI_PROVIDER_DEFAULT=lmstudio
export GUMI_LMSTUDIO_URL=http://127.0.0.1:1234/v1
./gumi start
```

Recent WSL2 builds forward `localhost` to Windows services. If that fails, use
the Windows host IP from `grep nameserver /etc/resolv.conf`.

Auth for local API calls: `Authorization: Bearer gumi-local`.

## Optional: Cursor self-hosted worker on this PC

So Cloud Agents can run **on this machine** (instead of a remote VM), use the
existing helper after the repo is cloned:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\setup-dev-env.ps1 -InstallWorkerAutostart
# or directly:
powershell -ExecutionPolicy Bypass -File .\scripts\windows\install-worker-autostart.ps1
```

See also `disable-sleep.ps1` and `enable-autologon.ps1` in the same folder.

## Native Windows (no WSL)

Supported for building, but you drive the steps yourself (no Makefile
convenience):

1. Install [Go 1.25+](https://go.dev/dl/) and [Node.js 22+](https://nodejs.org/).
2. `cd dashboard && npm ci && npm run build`
3. `cd runtime && go build -o ..\gumi.exe .\cmd\gumi`
4. `.\gumi.exe start`

Prefer WSL for day-to-day development on this repo.
