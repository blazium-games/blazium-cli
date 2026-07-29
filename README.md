# Blazium CLI

Command-line tool for installing [Blazium](https://blazium.app) editors, managing a local project registry, self-updating, and remote-controlling a running editor.

**License:** [MIT](LICENSE) — Copyright Blazium Games

Repository: [github.com/blazium-games/blazium-cli](https://github.com/blazium-games/blazium-cli)

## Install

Published builds and checksums are listed at:

`https://cdn.blazium.app/cli/cli.json`

Download the binary for your platform from that manifest (or use an existing install and run `blazium-cli upgrade`).

## Build from source

Requires [Go](https://go.dev/) 1.23.2+.

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
blazium-cli install nightly --templates
blazium-cli uninstall 0.6.714
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
blazium-cli projects
blazium-cli projects add ./MyProject
blazium-cli projects remove MyProject
blazium-cli upgrade --dry-run
blazium-cli upgrade
blazium-cli --quiet open ./MyProject
blazium-cli --help
```

### Default editor policy

Unset settings mean **latest release** among installed editors.

- `default_editor_channel`: `release` | `prerelease` | `nightly` (default `release`)
- `default_editor_version`: `latest` or a concrete version (default `latest`)
- Optional hard pin `default_editor` wins when that version is installed

### Open / load

`open` and `load` launch the editor with a unique remote_control port + token by default (`remote.enable_on_open`, default true). After the editor’s `/v1/health` is up, the CLI assigns a short **6-character instance id** via `POST /v1/instance`.

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
