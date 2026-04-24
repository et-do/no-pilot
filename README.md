# no-pilot

Zero-trust MCP server mirroring GitHub Copilot's built-in VS Code tools, with strict policy enforcement and no cloud dependencies.

[![Release](https://img.shields.io/github/v/release/et-do/no-pilot)](https://github.com/et-do/no-pilot/releases/latest)
[![CI](https://github.com/et-do/no-pilot/actions/workflows/ci.yml/badge.svg)](https://github.com/et-do/no-pilot/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Overview

**no-pilot** is a drop-in, zero-trust replacement for Copilot's built-in agent tools, running entirely on your infrastructure. It enforces project and user policies for every file read, search, and shell command—no exceptions.

- Mirrors Copilot's built-in VS Code tools (file read, directory list, search, terminal, etc.)
- Enforces deny/allow patterns from user and project config files
- No cloud, no telemetry, no sidecar—just a single binary
- Designed for teams and regulated environments

---

## Quick Start

<details>
<summary><strong>VS Code Dev Container (recommended)</strong></summary>

The devcontainer wires up `.vscode/mcp.json` so Copilot launches the server via `go run .` on demand — no pre-built binary required.

1. Open the repo in VS Code and click **Reopen in Container** when prompted (or run **Dev Containers: Reopen in Container** from the Command Palette).
2. Wait for the container to finish building.
3. Open the Output panel (`Ctrl+Shift+U`), select `MCP: no-pilot`, and confirm the server is running and tools are discovered.

> [!TIP]
> After making code changes, restart the MCP server to recompile: **Command Palette → MCP: Restart Server → no-pilot**. No `make install` needed — `go run .` uses the Go build cache and is always in sync with your source.

</details>

<details>
<summary><strong>Linux / macOS &mdash; from source (no container)</strong></summary>

Use this setup if you prefer to work directly in Linux or macOS without a Dev Container.

**Prerequisites:** Go 1.24+ installed and available on your `PATH`, and the repository cloned locally.

**1. Add no-pilot to VS Code MCP config**

Open (or create) `.vscode/mcp.json` in your project:

```json
{
  "servers": {
    "no-pilot": {
      "type": "stdio",
      "command": "/bin/sh",
      "args": ["-c", "cd /absolute/path/to/no-pilot && go run ."]
    }
  }
}
```

> [!IMPORTANT]
> Use a shell wrapper (`/bin/sh -c "cd ... && go run ."`) rather than pointing directly at a pre-built binary. VS Code starts the MCP server immediately on window attach — potentially before any build scripts finish — which causes `ENOENT` errors if the binary is not yet on disk. `go run .` compiles on demand using the Go build cache and is always ready.

**2. Open the Output panel**

Press `Ctrl+Shift+U`, select `MCP: no-pilot`, and confirm the server starts and tools are discovered.

> [!TIP]
> After making source changes, restart the MCP server to recompile: **Command Palette → MCP: Restart Server → no-pilot**.

</details>

<details>
<summary><strong>Linux / macOS &mdash; pre-built binary</strong></summary>

**1. Download and install the binary**

```sh
# Linux (amd64)
curl -L https://github.com/et-do/no-pilot/releases/latest/download/no-pilot-linux-amd64 \
  -o ~/.local/bin/no-pilot && chmod +x ~/.local/bin/no-pilot

# Linux (arm64)
curl -L https://github.com/et-do/no-pilot/releases/latest/download/no-pilot-linux-arm64 \
  -o ~/.local/bin/no-pilot && chmod +x ~/.local/bin/no-pilot

# macOS (Apple Silicon)
curl -L https://github.com/et-do/no-pilot/releases/latest/download/no-pilot-darwin-arm64 \
  -o ~/.local/bin/no-pilot && chmod +x ~/.local/bin/no-pilot

# macOS (Intel)
curl -L https://github.com/et-do/no-pilot/releases/latest/download/no-pilot-darwin-amd64 \
  -o ~/.local/bin/no-pilot && chmod +x ~/.local/bin/no-pilot
```

If `~/.local/bin` is not on your `$PATH`, add this to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```sh
export PATH="$HOME/.local/bin:$PATH"
```

**2. Add no-pilot to VS Code MCP config**

Open (or create) `.vscode/mcp.json` in your project, or open the user config via `MCP: Open User Configuration` in VS Code:

```json
{
  "servers": {
    "no-pilot": {
      "command": "/home/your-username/.local/bin/no-pilot",
      "args": []
    }
  }
}
```

> [!TIP]
> VS Code does not expand `~` in the command path — use the full absolute path.

**3. Restart VS Code**

Open the Output panel (`Ctrl+Shift+U`), select `MCP: no-pilot`, and confirm the server starts and tools are discovered.

</details>

<details>
<summary><strong>Windows</strong></summary>

**1. Download the binary**

Open PowerShell and run:

```powershell
# Windows (amd64)
$dest = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Invoke-WebRequest -Uri "https://github.com/et-do/no-pilot/releases/latest/download/no-pilot-windows-amd64" `
  -OutFile "$dest\no-pilot.exe"

# Windows (arm64)
$dest = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Invoke-WebRequest -Uri "https://github.com/et-do/no-pilot/releases/latest/download/no-pilot-windows-arm64" `
  -OutFile "$dest\no-pilot.exe"
```

Then add `%USERPROFILE%\bin` to your PATH:

```powershell
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:USERPROFILE\bin", "User")
```

> [!NOTE]
> Restart your terminal after updating PATH for the change to take effect.

**2. Add no-pilot to VS Code MCP config**

Open (or create) `.vscode/mcp.json` in your project, or open the user config via `MCP: Open User Configuration` in VS Code:

```json
{
  "servers": {
    "no-pilot": {
      "command": "C:\\Users\\your-username\\bin\\no-pilot.exe",
      "args": []
    }
  }
}
```

**3. Restart VS Code**

Open the Output panel (`Ctrl+Shift+U`), select `MCP: no-pilot`, and confirm the server starts and tools are discovered.

> [!WARNING]
> If you are using a **VS Code Dev Container**, do not use the Windows binary in your user-level `mcp.json`. The Windows binary cannot access container filesystem paths. Use the Dev Container setup above instead — the workspace `.vscode/mcp.json` runs the server inside the container where your files are.

</details>

---

## Policy Configuration

> [!NOTE]
> Policy configuration is optional. By default no-pilot runs with no restrictions.

Policies are configured via YAML files:

| Platform | User config path |
|---|---|
| Linux | `~/.config/no-pilot/config.yaml` |
| macOS | `~/Library/Application Support/no-pilot/config.yaml` |
| Windows | `%AppData%\no-pilot\config.yaml` |

Place a `.no-pilot.yaml` file in your repo root to set project-level policy. Project config is layered on top of user config — restrictions can only tighten, never loosen.

**Example:**

```yaml
tools:
  read_readFile:
    allowed: true
    deny_paths:
      - '**/secrets/**'
      - '**/*.key'

  read_listDirectory:
    allowed: true
    deny_paths:
      - '**/secrets/**'

  execute_runInTerminal:
    allowed: true
    allow_commands:
      - 'go build *'
      - 'go test *'
      - 'ls *'
    deny_commands:
      - 'rm *'
      - 'curl *'

  search_grepSearch:
    allowed: true
    deny_paths:
      - '**/secrets/**'
```

<details>
<summary><strong>Policy field reference</strong></summary>

| Field | Type | Description |
|---|---|---|
| `allowed` | bool | Set to `false` to disable the tool entirely. Defaults to `true`. |
| `deny_paths` | glob list | File path arguments matching any pattern are blocked. |
| `allow_commands` | glob list | Only commands matching a pattern are permitted (allowlist). |
| `deny_commands` | glob list | Commands matching any pattern are blocked, even if `allow_commands` permits them. |
| `deny_urls` | glob list | The **hostname** of URL arguments matching any pattern is blocked (web tools). |

</details>

<details>
<summary><strong>How user and project configs merge (zero-trust rules)</strong></summary>

> [!IMPORTANT]
> These rules are designed so that restrictions can only tighten, never loosen, as configs layer on top of each other.

- **`allowed: false` is sticky** — a tool disabled at the user level cannot be re-enabled by a project config.
- **Deny lists union** — every denied path, command, or URL from every config layer accumulates.
- **Allow lists intersect** — a command must satisfy every allowlist that has been configured; if only one layer restricts commands, that list applies.

</details>

---

MIT License | © et-do
