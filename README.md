# Blazium CLI

Command-line tool for installing [Blazium](https://blazium.app) editors, managing a local project registry, self-updating, remote-controlling a running editor, and deploying to Steam / itch.io / Blazium Games.

Desktop companion: [Blazium Hub](https://blazium.app/dev-tools/download?tool=hub) (bundles this CLI on install).

**License:** [MIT](LICENSE) — Copyright Blazium Games

Repository: [github.com/blazium-games/blazium-cli](https://github.com/blazium-games/blazium-cli)

## Community

- Official website: [https://blazium.app/](https://blazium.app/)
- IndieDB blog: [https://www.indiedb.com/engines/blazium-engine](https://www.indiedb.com/engines/blazium-engine)
- Official community: [Blazium Discord](https://discord.gg/sZaf9KYzDp)
- Docs: [docs.blazium.app](https://docs.blazium.app)

## Ecosystem

| Product | Role | Release |
|---------|------|---------|
| [CLI](https://github.com/blazium-games/blazium-cli) | Install editors, projects, remote control, Steam/itch deploy | Linux and Windows, x86_64 and x86_32. Catalog: [cli.json](https://cdn.blazium.app/cli/cli.json) |
| [Hub](https://github.com/blazium-games/blazium-hub) | Desktop companion; installers bundle the CLI | Linux and Windows, x86_64 and x86_32. Engine builds track `blazium_4.8` |
| [Crash reporter](https://github.com/blazium-games/blazium_crash_reporter) | Sidecar UI for engine and Hub crash reports | Linux and Windows, x86_64 and x86_32. Catalog: [crash_reporter.json](https://cdn.blazium.app/crash_reporter/crash_reporter.json). Engine builds track `blazium_4.8` |
| [Toolchain](https://github.com/blazium-games/blazium-toolchain) | PS1, PS2, N64, and Interactive DVD | Linux and Windows, x86_64 and x86_32. Catalog: [toolchain.json](https://cdn.blazium.app/toolchain/toolchain.json) |
| [Skills](https://github.com/blazium-games/blazium-skills) | Agent skill packs for Claude, Cursor, Codex, and Grok | Own semver, separate from the 0.8.x API baseline. Catalog: [skills.json](https://cdn.blazium.app/skills/skills.json) |
| [Subagents](https://github.com/blazium-games/blazium-subagents) | Studio roster that loads those skills | Own semver. Catalog: [subagents.json](https://cdn.blazium.app/subagents/subagents.json) |

## Install

### 1. npm

Linux and Windows, x64 and ia32:

```text
npx @blazium-engine/cli
npm install -g @blazium-engine/cli
```

The command is `blazium-cli`. Install `@blazium-engine/cli`. License: MIT.

npm installs one of these optional binaries for the current platform:

| Package | Platform | CPU |
|---------|----------|-----|
| `@blazium-engine/cli-linux-x64` | linux | x64 |
| `@blazium-engine/cli-linux-ia32` | linux | ia32 |
| `@blazium-engine/cli-win32-x64` | win32 | x64 |
| `@blazium-engine/cli-win32-ia32` | win32 | ia32 |

When that optional package is installed, its binary is used and nothing is downloaded. The CDN download runs only if that package is absent.

### 2. GitHub Releases

Download a binary for your platform from:

https://github.com/blazium-games/blazium-cli/releases

| Platform | Arch | Asset |
|----------|------|--------|
| Windows | x86_64 | `blazium-cli-windows-x86_64.exe` |
| Windows | x86_32 | `blazium-cli-windows-x86_32.exe` |
| Linux | x86_64 | `blazium-cli-linux-x86_64` |
| Linux | x86_32 | `blazium-cli-linux-x86_32` |

Published binaries are Linux and Windows only. A CLI you build yourself on macOS can still install and open a macOS editor; that editor install is separate from these release assets.

Non-Windows assets may include a matching `.sig` (GPG detach-sign). Put the binary on your `PATH` and rename to `blazium-cli` / `blazium-cli.exe` if you prefer.

### 3. CDN manifest

Published builds, URLs, and checksums:

`https://cdn.blazium.app/cli/cli.json`

Paths look like `https://cdn.blazium.app/cli/{os}/{arch}/{version}/blazium-cli[.exe]`.

### 4. Self-update

If you already have a build:

```text
blazium-cli upgrade
blazium-cli update apply --product cli
```

On Windows, when CLI lives under Program Files (Hub install), update may prompt for elevation (UAC).

### 5. Build from source

Requires [Go](https://go.dev/) 1.25+.

```text
git clone https://github.com/blazium-games/blazium-cli.git
cd blazium-cli
go test ./...
go build -o blazium-cli .
```

Embedded version strings live in `data/cliBuild.txt` and `data/defaultEngineBuild.txt` (placeholders are committed for local builds; CI overwrites them for releases).

## Hub commands

Registry file: `%APPDATA%\blazium\hub.json` (Windows) or `~/.config/blazium/hub.json` (Linux/macOS).

```text
blazium-cli install 0.6.714
blazium-cli install 0.6.751 --channel nightly
blazium-cli install nightly --templates
blazium-cli uninstall 0.6.714
blazium-cli uninstall 0.6.751 --channel nightly
blazium-cli editors
blazium-cli editors add C:\path\to\blazium.exe --version 0.6.714
blazium-cli editors default
blazium-cli editors default --channel release --version latest
blazium-cli editors default 0.6.714
blazium-cli editors path 0.6.714
blazium-cli install-path
blazium-cli install-path D:\Blazium\Editors
blazium-cli open ./MyProject
blazium-cli load ./MyProject
blazium-cli handle-uri "blazium://hub"
blazium-cli projects
blazium-cli projects add ./MyProject
blazium-cli projects remove MyProject
blazium-cli upgrade --dry-run
blazium-cli upgrade
blazium-cli --quiet open ./MyProject
blazium-cli --help
```

Editors install under `{install-path}/{channel}/{version}` (`release`, `prerelease`, or `nightly`).

### Deep links (`handle-uri`)

| URI | Action |
|-----|--------|
| `blazium://hub` | Launch/focus Hub via authenticated remote_control |
| `blazium://open?path=<path>` | Open project (focus if already running) |
| `blazium://load?path=<path>` | Load project with full profile |
| `blazium://project/<encoded-path>` | Shorthand for open |
| `blazium://install?version=<ver>&channel=` | Download/install editor |
| `blazium://register?path=<path>&version=&channel=` | Register a local editor binary |

OS installers register `blazium://` to **this CLI** (`handle-uri`). For Hub, CLI loads `hub_remote.json` from the **user** path then the **machine** path (`%ProgramData%\blazium\` / `/etc/blazium/`), launches Hub if needed, waits for `/v1/health`, then `exec` `show_hub` / `focus_window`. Editors use the same launch → wait → talk pattern with per-instance tokens in `cli.json`.

```text
blazium-cli hub-remote ensure
blazium-cli hub-remote ensure --path "C:\ProgramData\blazium\hub_remote.json"
```

`hub-remote ensure` never rotates a valid token; `--path` writes only that file (installer/machine use).

### Default editor policy

Unset settings mean **latest release** among installed editors.

- `default_editor_channel`: `release` | `prerelease` | `nightly` (default `release`)
- `default_editor_version`: `latest` or a concrete version (default `latest`)
- Optional hard pin `default_editor` wins when that version is installed

### Open / load

`open` and `load` launch the editor with a unique remote_control port + token by default (`remote.enable_on_open`, default true). After the editor's `/v1/health` is up, the CLI assigns a short **6-character instance id** via `POST /v1/instance`.

`load` also prints a project profile (JustAMCP / remote_control / editor_version / features).

Disable remote on launch: `blazium-cli remote config set enable-on-open false`.

## Remote control

Talk to a running Blazium editor that has the `remote_control` module enabled.

```text
blazium-cli remote status --format json
blazium-cli remote instances
blazium-cli remote list
blazium-cli remote exec ping
blazium-cli remote eval "2 + 2"
blazium-cli remote eval-gdscript "2 + 2"
blazium-cli remote eval-lua "1 + 1"
blazium-cli remote logs --since 0 --limit 200
blazium-cli remote errors --limit 100
blazium-cli remote debugger info
blazium-cli remote debugger clear
blazium-cli remote failed-run
blazium-cli remote autowork run --wait
blazium-cli remote autowork status
blazium-cli remote autowork results
blazium-cli remote --instance AB3K7M status
blazium-cli remote --project ./MyProject status
blazium-cli remote config get enable-on-open
blazium-cli remote config set enable-mcp-on-load true
blazium-cli remote enable --path ./MyProject
blazium-cli remote doctor
```

Instance selection:

- **1** live instance → used automatically
- **2+** → newest is default; a warning is printed (suppress with `--quiet`)
- `--instance <id>` / `--project <path>` select explicitly

Env: `BLAZIUM_REMOTE_HOST`, `BLAZIUM_REMOTE_PORT`, `BLAZIUM_REMOTE_TOKEN`, `BLAZIUM_REMOTE_EVAL_DEFAULT`.

CLI prefs: `%APPDATA%\blazium\cli.json` / `~/.config/blazium/cli.json`.

## Template metadata sources

`install --templates` uses the CDN `.tpz` bundle. Individual template file helpers resolve metadata in this order:

1. `https://cdn.blazium.app/{channel}/{version}/template_files.json` (per-file CDN manifest)
2. `.../templates.json` (legacy per-file array or `{base,mono}` bundle)
3. `.../details.json` (export template manager bundle)
4. Blazium templates API `GET /api/v1/templates/{deploy_type}/{version}` on `https://blazium.app` (override base URL with `BLAZIUM_CEREBRO_URL`)

## Deploy (Steam / itch.io)

Store deploys live in this CLI. `blazium-toolchain steam setup` only pin-fetches steamcmd. itch.io push and steam-sync use [itchio/butler](https://github.com/itchio/butler) as a Go module (no broth download). Steam Guard is in-process (no steamguard-cli / node-steam-totp).

Optional `blazium-deploy.yml` next to the game (see `blazium-deploy.example.yml`). Strings may use `${NAME}` or `${NAME:-default}`. Secrets come from flags, YAML after expansion, then `BLAZIUM_*` env.

```text
blazium-cli deploy tools status
blazium-cli deploy tools ensure
blazium-cli deploy steam guard totp
blazium-cli deploy steam guard setup          # interactive; write down the revocation code
blazium-cli deploy steam upload --dry-run
blazium-cli deploy steam upload
blazium-cli deploy steam set-live --build-id ID --beta-key beta
blazium-cli deploy itch login
blazium-cli deploy itch push ./build --target user/game:windows
blazium-cli deploy itch steam-sync --dry-run
blazium-cli deploy itch steam-logout
blazium-cli games build --asset build.yml
```

Example CI: [docs/deploy.example.yml](docs/deploy.example.yml).

Security:

- steam-sync refresh tokens are full Steam account access; `deploy itch steam-logout` deletes local creds.
- Create a publisher Web API key just for the CLI; it is only used to list/authorize apps.
- CI uses `BLAZIUM_STEAM_*` / `BLAZIUM_BUTLER_API_KEY`. Do not run `guard setup` on a runner.
- Write down the Steam revocation code before Finalize. Losing it and the maFile can lock the builder account.
- After steam-sync assemble, the CLI surfaces butler’s Steamworks SDK warning when `steam_api` files are present.
