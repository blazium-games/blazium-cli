# Blazium CLI

Tool that can handle downloading of the Blazium Engine, Blazium Templates, and manage lobby server games/scripts.

## Usage

```sh
blazium-cli <subcommand> [flags] [args]
```

## Subcommands

### Download Management

- `download`            Download templates or editors
    - `--template`      Download the export template
    - `--editor`        Download the editor
    - `--get-version`   Specify the version to use (defaults to value in data/defaultEngineBuild.txt if not set)
    - `--silent`        Suppress all logging output
    - `--mono`          Download the mono version
    - `--platform`      Specify the platform (e.g., linux, windows, macos) (required for --editor)
    - `--arch`          Specify the architecture (e.g., x86_64, arm64, arm32, 32bit, 64bit) (required for --editor)
    - `<destination>`   Destination path (use `.` for current directory)

### Lobby/Game Management

- `lobby create-game --lobby-control <lua|angelscript|relay> --send-rate <send-rate> [--tick-rate <tick-rate>] <folder>`         Create a new game, add it to games.ini, and associate it with a local folder
    - `<folder>` is always required, even for relay games, and must be the last argument.
- `lobby start-game <folder>`          Start the game associated with the folder
- `lobby stop-game <folder>`           Stop the game associated with the folder
- `lobby restart-game <folder>`        Restart the game associated with the folder
- `lobby delete-game <folder>`                Delete the game associated with the folder and remove it from games.ini
- `lobby dev`                          Start development mode: watches all folders in games.ini, lints scripts, and uploads on change

## Help Commands

Show general help:
```sh
blazium-cli help
```

Show help for download or lobby subcommands:
```sh
blazium-cli download --help
blazium-cli lobby --help
```

## How it works

- The CLI manages a `games.ini` file in the working directory, which caches all game IDs and their settings.
- Each game section in `games.ini` contains its settings and the local folder for scripts.
- If `lobby_control=relay`, the `tickrate` field is not required in `games.ini`, but `folder` is always required.
- The `dev` command watches all folders in `games.ini` for changes. On script or settings change:
    - Lints scripts with Luau
    - If lint passes, uploads scripts and settings to the server automatically

## Example `games.ini`

```ini
[25a98d5f-ca46-4ec5-8686-b2bf51471ffc]
lobby_control=lua
sendrate=100
tickrate=1000
folder=hangman

[533207a3-0477-4361-a1f3-f357000aa8dd]
lobby_control=relay
sendrate=100
folder=quiz
```

## Examples

Create a new Lua game:
```sh
blazium-cli lobby create-game --lobby-control lua --send-rate 100 --tick-rate 1000 ./hangman
```

Create a new relay game (folder required, tickrate not needed):
```sh
blazium-cli lobby create-game --lobby-control relay --send-rate 100 ./quiz
```

Start a game:
```sh
blazium-cli lobby start-game ./hangman
```

Stop a game:
```sh
blazium-cli lobby stop-game ./hangman
```

Restart a game:
```sh
blazium-cli lobby restart-game ./hangman
```

Delete a game:
```sh
blazium-cli lobby delete-game ./hangman
```

Start development mode (watches all games and uploads on change):
```sh
blazium-cli lobby dev
```

You must always specify a destination path for downloads (use `.` for current directory).

## Installation & Running Locally

1. **Install Go** (if not already):
   - https://go.dev/dl/

2. **Install dependencies:**
```sh
go mod tidy
```

3. **Run the CLI locally:**
```sh
go run . <subcommand> [flags] [args]
# Example:
go run . download template .
```

## Running Tests

To run all tests:
```sh
go test -v
```

To run a specific test file:
```sh
go test -v download_test.go
```
