# no-pilot VS Code Bridge Extension

Optional companion extension for `no-pilot`.

This extension starts a localhost HTTP bridge inside the VS Code extension host so the no-pilot MCP server can route selected calls through live VS Code APIs.

## Endpoints

- `POST /terminal/run`
- `POST /terminal/get_output`
- `POST /terminal/send`
- `POST /terminal/kill`
- `POST /terminal/list`
- `POST /terminal/last_command`
- `POST /read/problems`

## Local development

1. `npm install`
2. `npm run compile`
3. Press `F5` in this extension folder to launch an Extension Development Host.

Configure no-pilot to target the bridge URL:

```json
{
  "servers": {
    "no-pilot": {
      "type": "stdio",
      "command": "/bin/sh",
      "args": ["-c", "cd /workspaces/no-pilot && NO_PILOT_VSCODE_BRIDGE_URL=http://127.0.0.1:7777 go run ."]
    }
  }
}
```

## Marketplace publishing

```bash
npx @vscode/vsce package
npx @vscode/vsce publish
```

Use `VSCE_PAT` in CI for automated publishing.
