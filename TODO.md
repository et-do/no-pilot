# no-pilot — tool implementation tracker

Each tool mirrors a GitHub Copilot built-in agent tool and enforces the
`deny_patterns` / `allowed` policy from the merged user+project config before
executing. Tools are grouped by the same toolsets Copilot uses.

---


## `#read` — read files and editor state

- [x] `read/readFile` — read the content of a file (mirrors `read_file`)
- [x] `read/listDirectory` — list the contents of a directory (mirrors `list_dir`)
- [ ] `read/terminalLastCommand` — get the last terminal command and its output (mirrors `terminal_last_command`)
- [ ] `read/terminalSelection` — get the current terminal selection (mirrors `terminal_selection`)
- [x] `read/problems` — get workspace errors and warnings from diagnostics (mirrors `get_errors`)
- [ ] `read/getNotebookSummary` — list notebook cells and their metadata (mirrors `copilot_getNotebookSummary`)
- [ ] `read/readNotebookCellOutput` — read the output of a notebook cell execution (mirrors `read_notebook_cell_output`)


## `#search` — search the workspace

- [x] `search/fileSearch` — find files by glob pattern (mirrors `file_search`)
- [x] `search/textSearch` — full-text / regex search across files (mirrors `grep_search`)
- [ ] `search/codebase` — semantic/embedding-based workspace search (mirrors `semantic_search`)
- [ ] `search/changes` — list current source control changes (mirrors `get_changed_files`)
- [ ] `search/usages` — find all references, implementations, and definitions of a symbol (mirrors `vscode_listCodeUsages`)


## `#edit` — write files and workspace structure

- [ ] `edit/createFile` — create a new file with given content (mirrors `create_file`)
- [ ] `edit/createDirectory` — create a new directory (mirrors `create_directory`)
- [ ] `edit/editFiles` — apply targeted string-replacement edits to one or more files (mirrors `replace_string_in_file` / `multi_replace_string_in_file`)
- [ ] `edit/renameSymbol` — semantics-aware symbol rename across the workspace (mirrors `vscode_renameSymbol`)
- [ ] `edit/editNotebook` — insert, delete, or modify notebook cells (mirrors `edit_notebook_file`)
- [ ] `edit/createNotebook` — create a new Jupyter notebook (mirrors `create_new_jupyter_notebook`)


## `#execute` — run code and commands

- [x] `execute/runInTerminal` — run a shell command in a terminal (mirrors `run_in_terminal`)
- [ ] `execute/getTerminalOutput` — get output from a running async terminal session (mirrors `get_terminal_output`)
- [ ] `execute/sendToTerminal` — send input to a persistent terminal session (mirrors `send_to_terminal`)
- [ ] `execute/killTerminal` — terminate a terminal session (mirrors `kill_terminal`)
- [ ] `execute/runNotebookCell` — execute a notebook cell (mirrors `run_notebook_cell`)
- [ ] `execute/createAndRunTask` — create and run a workspace task (mirrors `create_and_run_task`)
- [ ] `execute/runTests` — run unit tests and return results (mirrors `runTests`)
- [ ] `execute/testFailure` — get detailed unit test failure information (mirrors `test_failure`)

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

- [ ] `web/fetch` — fetch and extract text content from a public URL (mirrors `fetch_webpage`)

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
