# Kilden CLI (`kd`)

The command-line client for [Kilden](https://kilden.io) — sign in to your
account and manage your teams, projects, write keys and event data from the
terminal.

Authentication uses the OAuth 2.0 **Device Authorization Grant** (RFC 8628):
no passwords, no secrets stored beyond the tokens the browser hands back.

## Install

### macOS / Linux — one line

```bash
curl -fsSL https://raw.githubusercontent.com/kildenhq/kilden-cli/main/install.sh | bash
```

Detects your OS and CPU, downloads the latest release, verifies its checksum,
and installs `kd` into `~/.local/bin`. Override with env vars:

```bash
# pin a version and/or pick a different (writable) install dir
curl -fsSL https://raw.githubusercontent.com/kildenhq/kilden-cli/main/install.sh \
  | KILDEN_VERSION=v0.1.0 KILDEN_INSTALL_DIR="$HOME/bin" bash
```

For a system-wide install into a root-owned dir, run it under `sudo`:
`curl -fsSL … | sudo KILDEN_INSTALL_DIR=/usr/local/bin bash`.

If `~/.local/bin` isn't on your `PATH`, the installer tells you how to add it.

### Windows (x64)

Download `kd_windows_amd64.zip` from the
[latest release](https://github.com/kildenhq/kilden-cli/releases/latest),
unzip it, and put `kd.exe` somewhere on your `PATH` (e.g. a folder you add under
*System → Environment Variables*). In PowerShell:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\kilden"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Invoke-WebRequest -Uri "https://github.com/kildenhq/kilden-cli/releases/latest/download/kd_windows_amd64.zip" -OutFile "$env:TEMP\kd.zip"
Expand-Archive -Force "$env:TEMP\kd.zip" $dir
# add $dir to your PATH, then:
kd --version
```

### Manual download

Pre-built binaries for every release are on the
[releases page](https://github.com/kildenhq/kilden-cli/releases/latest):

| Platform | Asset |
|---|---|
| macOS (Apple Silicon) | `kd_darwin_arm64.tar.gz` |
| macOS (Intel) | `kd_darwin_amd64.tar.gz` |
| Linux (x86-64) | `kd_linux_amd64.tar.gz` |
| Linux (ARM64) | `kd_linux_arm64.tar.gz` |
| Windows (x64) | `kd_windows_amd64.zip` |

### Windows (x64)

Download `kd_windows_amd64.zip` from the
[latest release](https://github.com/kildenhq/kilden-cli/releases/latest),
unzip it, and put `kd.exe` somewhere on your `PATH` (e.g. a folder you add under
*System → Environment Variables*). In PowerShell:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\kilden"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Invoke-WebRequest -Uri "https://github.com/kildenhq/kilden-cli/releases/latest/download/kd_windows_amd64.zip" -OutFile "$env:TEMP\kd.zip"
Expand-Archive -Force "$env:TEMP\kd.zip" $dir
# add $dir to your PATH, then:
kd --version
```

### With Go

```bash
go install github.com/kildenhq/kilden-cli@latest
```

The binary is `kd`. Prefer typing `kilden`? Add an alias:

```bash
alias kilden=kd
```

## Sign in

```bash
kd login
```

`kd` prints a short code and opens your browser. Confirm the code matches,
approve, and you're in. The session is stored under your OS config dir
(`~/.config/kilden/credentials.json`, mode `0600`) and refreshes itself.

Self-hosting the panel? Point the CLI at it:

```bash
kd login --host https://panel.example.com
# or: export KILDEN_HOST=https://panel.example.com
```

## Usage

```bash
kd whoami                      # who am I, and which team/project is active

kd team                        # list your teams (the active one is marked *)
kd team use                    # pick the active team (interactive)
kd team use 42                 # …or by id

kd project                     # list the active team's projects
kd project use <project-id>    # switch the active project
kd project create --name "Web" --timezone America/Santiago

kd key                         # list the active project's write keys
kd key create --kind secret --label CI   # full value shown once

kd events                      # recent events, newest first
kd events --event signup       # filter by event name
kd properties                  # event names + property keys observed
```

Most data commands act on your **active project**; override per command with
`--project <id>`.

> Kilden properties are schema-less by design — there is nothing to "create".
> `kd properties` lists what your events actually carry.

## How auth works

1. `kd login` asks the panel for a device code (`POST /oauth/device/code`).
2. You approve in the browser at the verification URL.
3. `kd` polls `POST /oauth/token` (honoring `authorization_pending` /
   `slow_down`) until it receives an access + refresh token.
4. Subsequent commands call the panel's management API at `/api/v1`, sending
   the bearer token and refreshing it transparently when it expires.

The CLI ships a **public** OAuth client id (no secret — a CLI can't keep one).
Authorization for everything you do is enforced server-side by the panel
against your team membership and role.

## Development

```bash
go build -o kd .
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
