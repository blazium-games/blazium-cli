# @blazium-engine/cli

Command-line tool for the Blazium engine. It installs editors, manages projects, remote-controls a running editor, and deploys to Steam, itch.io, and Blazium Games.

```bash
npx @blazium-engine/cli
npm install -g @blazium-engine/cli
blazium-cli
```

Supported platforms are Linux and Windows, x64 and ia32. When npm installs the matching optional package (`@blazium-engine/cli-linux-x64`, `cli-linux-ia32`, `cli-win32-x64`, or `cli-win32-ia32`), that binary is used and nothing is downloaded. The CDN download runs only if that optional package is absent.

- Repository: https://github.com/blazium-games/blazium-cli
- Docs: https://docs.blazium.app
- Discord: https://discord.gg/sZaf9KYzDp
- License: MIT
