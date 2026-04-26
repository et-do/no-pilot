# no-pilot — tool implementation tracker

Each tool mirrors a GitHub Copilot built-in agent tool and enforces the
`deny_patterns` / `allowed` policy from the merged user+project config before
executing. Tools are grouped by the same toolsets Copilot uses.

---


## `#read` — read files and editor state

- [x] `read/readFile` — read the content of a file (mirrors `read_file`)
- [x] `read/listDirectory` — list the contents of a directory (mirrors `list_dir`)
- [x] `read/terminalLastCommand` — get the last terminal command and its output (mirrors `terminal_last_command`)
- [ ] `read/terminalSelection` — get the current terminal selection (mirrors `terminal_selection`; not on the standalone-server roadmap because it requires VS Code terminal UI integration)
- [ ] VS Code terminal bridge mode for execute/read terminal tools: add terminal target abstraction, extension-host bridge transport, and managed fallback for standalone use
- [x] `read/problems` — get workspace errors and warnings from diagnostics (mirrors `get_errors`)
- [x] `read/getNotebookSummary` — list notebook cells and their metadata (mirrors `copilot_getNotebookSummary`)
- [x] `read/readNotebookCellOutput` — read the output of a notebook cell execution (mirrors `read_notebook_cell_output`)


## `#search` — search the workspace

- [x] `search/fileSearch` — find files by glob pattern (mirrors `file_search`)
- [x] `search/textSearch` — full-text / regex search across files (mirrors `grep_search`)
- [x] `search/codebase` — relevance-ranked lexical workspace search (closest standalone equivalent to `semantic_search`)
- [x] `search/changes` — list current source control changes (mirrors `get_changed_files`)
- [x] `search/usages` — find textual symbol usages across files (closest standalone equivalent to `vscode_listCodeUsages`)


## `#edit` — write files and workspace structure

- [x] `edit/createFile` — create a new file with given content (mirrors `create_file`)
- [x] `edit/createDirectory` — create a new directory (mirrors `create_directory`)
- [x] `edit/editFiles` — apply targeted string-replacement edits to one or more files (mirrors `replace_string_in_file` / `multi_replace_string_in_file`)
- [x] `edit/renameSymbol` — lexical symbol rename across the workspace (closest standalone equivalent to `vscode_renameSymbol`)
- [x] `edit/editNotebook` — insert, delete, or modify notebook cells in persisted `.ipynb` JSON (closest standalone equivalent to `edit_notebook_file`)
- [x] `edit/createNotebook` — create a new Jupyter notebook file (closest standalone equivalent to `create_new_jupyter_notebook`)


## `#execute` — run code and commands

- [x] `execute/runInTerminal` — run a shell command in a terminal (mirrors `run_in_terminal`)
- [x] `execute/listTerminals` — list tracked terminal sessions (no direct Copilot equivalent)
- [x] `execute/getTerminalOutput` — get output from a running async terminal session, including optional byte-range reads (mirrors `get_terminal_output`)
- [x] `execute/sendToTerminal` — send input to a persistent terminal session (mirrors `send_to_terminal`)
- [x] `execute/killTerminal` — terminate a terminal session (mirrors `kill_terminal`)
- [x] `execute/runNotebookCell` — execute notebook code cells up to a target cell and persist outputs (closest standalone equivalent to `run_notebook_cell`)
- [x] `execute/createAndRunTask` — create/update `.vscode/tasks.json` and run shell task commands (closest standalone equivalent to `create_and_run_task`)
- [x] `execute/runTests` — run tests across Go (`go test`), Python (`pytest`), and Node (`npm test`) with subset targeting, language selection/inference, and coverage mode support
- [x] `execute/testFailure` — return failure details from the most recent `execute_runTests` run (standalone equivalent to `test_failure`)

## `#browser` — headless browser (go-rod / chromedp)

All browser tools drive a real Chrome/Chromium instance via the DevTools Protocol.
No VS Code dependency — works as a standalone MCP server.

- [ ] `browser/navigate` — navigate to a URL
- [ ] `browser/readContent` — extract the readable text content of the current page
- [ ] `browser/screenshot` — take a screenshot of the current page or a specific element
- [ ] `browser/click` — click on a page element by selector or coordinate
- [ ] `browser/type` — type text into a focused input or element
- [ ] `browser/hover` — hover over an element
- [ ] `browser/drag` — drag from one coordinate to another
- [ ] `browser/handleDialog` — accept or dismiss browser dialogs (alert/confirm/prompt)
- [ ] `browser/scroll` — scroll the page or a specific element

## `#web` — lightweight web fetch (no browser required)

- [x] `web/fetch` — fetch and extract text content from a public URL (closest standalone equivalent to `fetch_webpage`)

## `#vscode` — VS Code API and workspace metadata

- [ ] `vscode/runCommand` — execute a VS Code command by ID (mirrors `run_vscode_command`)
- [ ] `vscode/getVSCodeAPI` — query VS Code API documentation (mirrors `get_vscode_api`)
- [ ] `vscode/askQuestions` — present clarifying questions to the user via the UI (mirrors `vscode_askQuestions`)
- [ ] `vscode/installExtension` — install a VS Code extension by ID (mirrors `install_extension`)
- [ ] `vscode/searchExtensions` — search the extension marketplace (mirrors `vscode_searchExtensions_internal`)
- [ ] `vscode/getProjectSetupInfo` — get scaffolding instructions for a project type (mirrors `get_project_setup_info`)

## `#agent` — subagent delegation

- [ ] `agent/runSubagent` — delegate a task to an isolated subagent context (mirrors `runSubagent`)

## `#workspace` — workspace management

- [ ] `workspace/createWorkspace` — scaffold a new VS Code workspace (mirrors `create_new_workspace`)

## `#image` — image context

- [ ] `image/viewImage` — read and describe an image file (mirrors `view_image`)

---

## Infrastructure (not tools, but required for all of the above)

- [x] Config loading (`internal/config`) — user + project YAML, merge, policy check
- [x] Server bootstrap (`internal/server`) — `mcp-go` MCPServer wired to stdio
- [x] Policy middleware — shared `enforce(cfg, toolName, path)` helper used by every tool handler
- [x] Deny pattern matching — glob matching for `deny_patterns` (e.g. `path/filepath.Match` or `doublestar`)
- [ ] Integration test harness — in-process client helpers shared across tool test files
