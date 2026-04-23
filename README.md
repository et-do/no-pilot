# no-pilot

Zero-trust MCP server mirroring GitHub Copilot’s built-in VS Code tools, with strict policy enforcement and no cloud dependencies.

---

## Overview

**no-pilot** is a drop-in, zero-trust replacement for Copilot’s built-in agent tools, running entirely on your infrastructure. It enforces project and user policies for every file read, search, and shell command—no exceptions.

**Features:**
- Mirrors Copilot’s built-in VS Code tools (file read, directory list, search, terminal, etc.)
- Enforces deny/allow patterns from user (~/.config/no-pilot/config.yaml) and project (.no-pilot.yaml) config
- No cloud, no telemetry, no sidecar—just a single binary
- Designed for teams and regulated environments


## Quick Start

1. **Download the no-pilot binary**
	 - Place it in `~/.local/bin/no-pilot` (Linux/macOS) or `%USERPROFILE%\bin\no-pilot.exe` (Windows), or any directory on your `$PATH`.
	 - Make it executable: `chmod +x ~/.local/bin/no-pilot`

2. **Add no-pilot to VS Code MCP config**
	 - Open (or create) `.vscode/mcp.json` in your project, or open the user config via `MCP: Open User Configuration` in VS Code.
	 - Add:

```json
{
	"servers": {
		"no-pilot": {
			"command": "/absolute/path/to/no-pilot",
			"args": []
		}
	}
}
```
> **Tip:** Use the full path to the binary if VS Code cannot find it. On Linux/macOS, this is often `~/.local/bin/no-pilot`.

3. **Configure policies**
	 - User config: `~/.config/no-pilot/config.yaml`
	 - Project config: `.no-pilot.yaml` in your repo root (overrides user config)

**Example policy:**

```yaml
deny_patterns:
  - '**/secrets/**'
  - '**/*.key'
allowed:
  - '**/*.go'
  - '**/README.md'
```

4. **Start the server manually (optional)**

```sh
no-pilot                # Start MCP server on stdio (default)
no-pilot --config ./no-pilot.yaml
```

5. **Restart VS Code**
	 - Open the Output panel (`Ctrl+Shift+U`) and select `MCP: no-pilot` to verify the server starts and tools are discovered.
	 - Use the “Configure Tools” button in the VS Code Chat input bar to verify no-pilot’s tools are listed.

---


## Example: Full Policy Configuration

Below is a sample config file (`~/.config/no-pilot/config.yaml` or `.no-pilot.yaml`) with rules for each tool category.

```yaml
# Deny reading secrets and private keys, allow only Go files and docs
tools:
  read/readFile:
    deny_paths:
      - '**/secrets/**'    # Block secret directories
      - '**/*.key'         # Block private keys
    allowed: true
    # Only allow reading Go files and README.md
    allowed_paths:
      - '**/*.go'
      - '**/README.md'

  read/listDirectory:
    deny_paths:
      - '**/secrets/**'
    allowed: true

  execute/runInTerminal:
    # Only allow safe commands, block dangerous ones
    allow_commands:
      - 'go build *'
      - 'go test *'
      - 'ls *'
    deny_commands:
      - 'rm *'             # Block file deletion
      - 'curl *'           # Block network exfiltration
      - 'cat /etc/*'       # Block reading system files
    allowed: true

  search/grepSearch:
    deny_paths:
      - '**/secrets/**'
    allowed: true

  # Add more tool-specific rules as needed
```

### Why these rules?

- **deny_paths**: Prevents accidental or malicious access to sensitive files (secrets, keys, etc).
- **allowed_paths**: Restricts access to only the files you want agents to see (e.g., source code, docs).
- **allow_commands**: Only allows a safe subset of shell commands to run (e.g., build, test, list files).
- **deny_commands**: Explicitly blocks dangerous commands (e.g., deleting files, network access, reading system files).
- **allowed**: Set to `true` to enable the tool, `false` to disable it entirely.

**Tip:** Project config (`.no-pilot.yaml`) always overrides user config for the same tool.

---



## Status

See TODO.md for tool parity and progress.

---

MIT License | © et-do
