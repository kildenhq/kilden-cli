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

For a system-wide install, install normally and move the binary with `sudo`
(don't pipe a remote script straight into `sudo` — that runs unreviewed code
as root):

```bash
sudo mv ~/.local/bin/kd /usr/local/bin/kd
```

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

## Getting started

```bash
kd init                        # create a project, print the snippet, wait for the first event
kd init --name "Fjord" --timezone America/Santiago
kd init --no-wait              # create it and get out of the way
```

`kd init` is deliberately thin. It creates the project, makes it active, prints
the browser snippet with your write key already in it, and then waits — up to
`--wait` (five minutes by default) — until Kilden receives its first real event.

It writes **nothing** into your repository and guesses nothing about your
stack. Framework detection and file writing are where 90% of the cost and all
of the maintenance of an `init` live, and they are exactly the part you can
hand to the coding agent you already have open. Paste the snippet, or paste it
into your agent.

A wait that times out exits 0. People install tomorrow, and a red exit code
would be lying about that.

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
kd key create --kind public --label web  # public write key (wk_)
kd key create --kind secret --label CI   # server key (sk_), full value shown once

kd identity-secret             # list the project's identity-verification secrets
kd identity-secret create --kid v1   # HS256 secret (is_), full value shown once
kd identity-secret disable 3   # revoke a secret by id (from `identity-secret ls`)

kd events                      # recent events, newest first
kd events --event signup       # filter by event name
kd tail                        # stream events live as they arrive (Ctrl+C to stop)
kd tail --event signup         # only stream a given event
kd tail --json | jq            # raw JSON per event, one per line
kd properties                  # event names + property keys observed

kd apply -f kilden.yaml         # create or update config from a spec (idempotent)
kd cohort                       # cohorts, flags, insights, in-app units,
kd flag                         # campaigns and experiments each get the same
kd insight                      # quartet: ls / get / rm / export
kd unit
kd campaign
kd experiment

kd cohort materialize power-users   # recompute membership now, not on the sweep
```

## Config as code

`kd apply` reads one YAML (or JSON) document with a key per collection, and is
**idempotent**: re-applying an unchanged spec leaves everything exactly as it
was — no duplicates, and no side effects like re-announcing a cohort's whole
membership.

```yaml
cohorts:
  - slug: power-users
    name: Power users
    definition:
      conditions:
        - type: behavior
          event: checkout_completed
          count_gte: 3
          days: 30
flags:
  - key: checkout-v2
    name: New checkout
    active: true
    rollout_percentage: 50
campaigns:
  - slug: welcome-series
    name: Welcome series
    status: active
    reentry: never
    cohort: power-users          # cohorts are named by slug, never by uuid
    nodes:
      - key: signup-trigger      # the key is the node's stable identity
        type: trigger
        config: {mode: event, event: signup}
      - key: welcome-email
        type: email
        config: {subject: Welcome, body_template: "<p>Hi</p>"}
    edges:
      - from: signup-trigger
        to: welcome-email
```

Collections are applied in **dependency order** (cohorts → flags → insights →
units → campaigns → experiments), so the order you write them in does not
matter.

Two identities, depending on the resource: `slug` for cohorts, insights, units
and campaigns — anything you created in the panel keeps a null slug and is never
listed or overwritten — and `key` for flags and experiments, where the panel and
your file address the same rows on purpose.

**A campaign node's `key` matters more than it looks.** The engine's node id is
derived from it, and people mid-flow are parked on that id: keeping a key keeps
them where they are, and renaming one is a genuinely different step, so whoever
was waiting there leaves the campaign.

Some things are deliberately not in the document, because a file describes state
rather than acts: broadcasting a campaign, sending a test, and concluding or
promoting an experiment all stay in the panel.

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
