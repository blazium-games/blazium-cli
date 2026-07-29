# Blazium CLI

Command-line tool for installing Blazium editors, managing a local project registry, self-updating, and remote-controlling a running editor.

## Hub commands

Registry file: `%APPDATA%\blazium\hub.json` (Windows) or `~/.config/blazium/hub.json` (Linux/macOS).

```text
blazium-cli install 0.6.714
blazium-cli install nightly --templates
blazium-cli uninstall 0.6.714
blazium-cli editors
blazium-cli editors add C:\path\to\blazium.exe --version 0.6.714
blazium-cli editors default 0.6.714
blazium-cli editors path 0.6.714
blazium-cli install-path
blazium-cli install-path D:\Blazium\Editors
blazium-cli open ./MyProject
blazium-cli projects
blazium-cli projects add ./MyProject
blazium-cli projects remove MyProject
blazium-cli upgrade --dry-run
blazium-cli upgrade
blazium-cli --help
```

## Remote control (editor)

Talk to a running Blazium editor with the `remote_control` module enabled (`--enable-remote-control` or ProjectSettings `blazium/remote_control/server_enabled`):

```text
blazium-cli remote status --format json
blazium-cli remote list
blazium-cli remote exec ping
blazium-cli remote eval "2 + 2"
blazium-cli remote eval-gdscript "2 + 2"
blazium-cli remote eval-lua "1 + 1"
blazium-cli remote enable --path ./MyProject
blazium-cli remote doctor
```

Env: `BLAZIUM_REMOTE_HOST`, `BLAZIUM_REMOTE_PORT`, `BLAZIUM_REMOTE_TOKEN`, `BLAZIUM_REMOTE_EVAL_DEFAULT`.

See workspace `REMOTE_CONTROL_UNITY_CLI_REFERENCE.md` for the Unity CLI → Blazium mapping tracker.

## Template metadata sources

`install --templates` uses the CDN `.tpz` bundle. Individual template file helpers still resolve metadata in this order:

1. `https://cdn.blazium.app/{channel}/{version}/template_files.json` (per-file manifest from ci_cd)
2. `.../templates.json` (legacy per-file array, or Godot `{base,mono}` bundle)
3. `.../details.json` (Godot Export Template Manager bundle)
4. Cerebro `GET /api/v1/templates/{deploy_type}/{version}` (`BLAZIUM_CEREBRO_URL` overrides the host)
