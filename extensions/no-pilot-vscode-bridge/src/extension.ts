import * as http from "http";
import { randomUUID } from "crypto";
import * as path from "path";
import * as vscode from "vscode";

type JSONValue = string | number | boolean | null | JSONValue[] | { [key: string]: JSONValue };

type ToolResponse = {
  text: string;
  isError?: boolean;
};

type TerminalSession = {
  id: string;
  terminal: vscode.Terminal;
  command: string;
  output: string;
  outputBytes: number;
  createdAt: number;
  updatedAt: number;
  running: boolean;
  hasExitCode: boolean;
  exitCode: number;
};

type RunPayload = {
  command?: string;
  mode?: string;
  timeout?: number;
  cwd?: string;
  env?: string;
};

type IDPayload = {
  id?: string;
};

type SendPayload = IDPayload & {
  command?: string;
};

type OutputPayload = IDPayload & {
  startOffset?: number;
  endOffset?: number;
};

type ProblemsPayload = {
  filePath?: string;
  path?: string;
  paths?: JSONValue;
};

let server: http.Server | undefined;
let serverAddress = "";
let outputListener: vscode.Disposable | undefined;
let closeListener: vscode.Disposable | undefined;
let shellExecutionListener: vscode.Disposable | undefined;

const sessions = new Map<string, TerminalSession>();
const terminalToSession = new Map<vscode.Terminal, string>();

let lastCommandText = "";
let lastCommandOutput = "";

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  context.subscriptions.push(
    vscode.commands.registerCommand("noPilotBridge.showStatus", () => {
      const msg = serverAddress
        ? `no-pilot bridge running at ${serverAddress}`
        : "no-pilot bridge is not running";
      void vscode.window.showInformationMessage(msg);
    }),
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("noPilotBridge.restartServer", async () => {
      await stopBridge();
      await startBridge();
      void vscode.window.showInformationMessage(`no-pilot bridge restarted at ${serverAddress}`);
    }),
  );

  await startBridge();

  context.subscriptions.push({
    dispose: () => {
      void stopBridge();
    },
  });
}

export function deactivate(): Thenable<void> | void {
  return stopBridge();
}

async function startBridge(): Promise<void> {
  const cfg = vscode.workspace.getConfiguration("noPilot");
  const host = String(cfg.get("bridgeHost", "127.0.0.1")).trim() || "127.0.0.1";
  const port = Number(cfg.get("bridgePort", 7777));

  attachTerminalListeners();

  server = http.createServer((req, res) => {
    void handleRequest(req, res);
  });

  await new Promise<void>((resolve, reject) => {
    server?.once("error", reject);
    server?.listen(port, host, () => {
      server?.off("error", reject);
      resolve();
    });
  });

  serverAddress = `http://${host}:${port}`;
  console.log(`[no-pilot-bridge] listening on ${serverAddress}`);
}

async function stopBridge(): Promise<void> {
  if (outputListener) {
    outputListener.dispose();
    outputListener = undefined;
  }
  if (closeListener) {
    closeListener.dispose();
    closeListener = undefined;
  }
  if (shellExecutionListener) {
    shellExecutionListener.dispose();
    shellExecutionListener = undefined;
  }

  for (const session of sessions.values()) {
    try {
      session.terminal.dispose();
    } catch {
      // no-op
    }
  }
  sessions.clear();
  terminalToSession.clear();

  if (!server) {
    serverAddress = "";
    return;
  }

  await new Promise<void>((resolve) => {
    server?.close(() => resolve());
  });
  server = undefined;
  serverAddress = "";
}

function attachTerminalListeners(): void {
  if (!outputListener) {
    outputListener = vscode.window.onDidWriteTerminalData((evt) => {
      const id = terminalToSession.get(evt.terminal);
      if (!id) {
        return;
      }
      const session = sessions.get(id);
      if (!session) {
        return;
      }
      session.output += evt.data;
      session.outputBytes = Buffer.byteLength(session.output);
      session.updatedAt = Date.now();
      lastCommandOutput = session.output;
    });
  }

  if (!closeListener) {
    closeListener = vscode.window.onDidCloseTerminal((terminal) => {
      const id = terminalToSession.get(terminal);
      if (!id) {
        return;
      }
      const session = sessions.get(id);
      if (!session) {
        terminalToSession.delete(terminal);
        return;
      }
      session.running = false;
      session.hasExitCode = true;
      session.updatedAt = Date.now();
      terminalToSession.delete(terminal);
    });
  }

  if (!shellExecutionListener) {
    shellExecutionListener = vscode.window.onDidEndTerminalShellExecution((evt) => {
      const id = terminalToSession.get(evt.terminal);
      if (!id) {
        return;
      }
      const session = sessions.get(id);
      if (!session) {
        return;
      }
      session.running = false;
      session.hasExitCode = true;
      session.exitCode = evt.exitCode ?? 0;
      session.updatedAt = Date.now();
    });
  }
}

async function handleRequest(req: http.IncomingMessage, res: http.ServerResponse): Promise<void> {
  setJSONHeaders(res);

  if (req.method !== "POST") {
    sendJSON(res, 405, { text: "method not allowed", isError: true });
    return;
  }

  const reqPath = String(req.url || "").split("?")[0];
  const payload = await readPayload(req);

  try {
    switch (reqPath) {
      case "/terminal/run":
        sendJSON(res, 200, handleTerminalRun(payload as RunPayload));
        return;
      case "/terminal/get_output":
        sendJSON(res, 200, handleTerminalGetOutput(payload as OutputPayload));
        return;
      case "/terminal/send":
        sendJSON(res, 200, handleTerminalSend(payload as SendPayload));
        return;
      case "/terminal/kill":
        sendJSON(res, 200, handleTerminalKill(payload as IDPayload));
        return;
      case "/terminal/list":
        sendJSON(res, 200, handleTerminalList());
        return;
      case "/terminal/last_command":
        sendJSON(res, 200, handleTerminalLastCommand());
        return;
      case "/read/problems":
        sendJSON(res, 200, handleReadProblems(payload as ProblemsPayload));
        return;
      default:
        sendJSON(res, 404, { text: `unknown route: ${reqPath}`, isError: true });
        return;
    }
  } catch (err) {
    sendJSON(res, 500, { text: errMessage(err), isError: true });
  }
}

function handleTerminalRun(payload: RunPayload): ToolResponse {
  const command = String(payload.command || "").trim();
  if (!command) {
    return { text: "command is required", isError: true };
  }

  const env = parseEnv(payload.env || "");
  const cwd = String(payload.cwd || "").trim();
  const mode = String(payload.mode || "sync").trim().toLowerCase();

  const options: vscode.TerminalOptions = {
    name: `no-pilot:${command.slice(0, 24)}`,
  };
  if (cwd) {
    options.cwd = cwd;
  }
  if (Object.keys(env).length > 0) {
    options.env = env;
  }

  const terminal = vscode.window.createTerminal(options);
  const id = randomUUID().replace(/-/g, "");
  const now = Date.now();
  const session: TerminalSession = {
    id,
    terminal,
    command,
    output: "",
    outputBytes: 0,
    createdAt: now,
    updatedAt: now,
    running: true,
    hasExitCode: false,
    exitCode: 0,
  };

  sessions.set(id, session);
  terminalToSession.set(terminal, id);

  lastCommandText = command;
  lastCommandOutput = "";

  terminal.show(false);
  terminal.sendText(command, true);

  if (mode === "async") {
    return { text: formatRunning(session) };
  }

  return {
    text:
      formatRunning(session) +
      "\n(note: VS Code bridge sync mode currently returns immediately; use execute_getTerminalOutput for streaming output)",
  };
}

function handleTerminalGetOutput(payload: OutputPayload): ToolResponse {
  const id = String(payload.id || "").trim();
  if (!id) {
    return { text: "id is required", isError: true };
  }

  const session = sessions.get(id);
  if (!session) {
    return { text: `terminal ${id} not found`, isError: true };
  }

  let output = session.output;
  let start = 0;
  let end = Buffer.byteLength(output);

  const hasStart = Number.isFinite(payload.startOffset);
  const hasEnd = Number.isFinite(payload.endOffset);

  if (hasStart || hasEnd) {
    start = Math.max(0, Number(payload.startOffset || 0));
    end = hasEnd ? Number(payload.endOffset) : end;
    if (end < 0) {
      end = Buffer.byteLength(output);
    }
    if (end < start) {
      return { text: `endOffset ${end} is before startOffset ${start}`, isError: true };
    }

    const buf = Buffer.from(output);
    output = buf.subarray(start, Math.min(end, buf.length)).toString();
  }

  const status = session.running ? "running" : `completed(exit=${session.exitCode})`;
  const meta = `terminal_id: ${id}\ncommand: ${session.command}\nstatus: ${status}\noutput_bytes: ${session.outputBytes}`;
  return { text: output ? `${meta}\n${output}` : meta };
}

function handleTerminalSend(payload: SendPayload): ToolResponse {
  const id = String(payload.id || "").trim();
  if (!id) {
    return { text: "id is required", isError: true };
  }

  const session = sessions.get(id);
  if (!session) {
    return { text: `terminal ${id} not found`, isError: true };
  }

  const input = typeof payload.command === "string" ? payload.command : "";
  session.terminal.sendText(input, true);
  session.updatedAt = Date.now();

  return { text: formatRunning(session) };
}

function handleTerminalKill(payload: IDPayload): ToolResponse {
  const id = String(payload.id || "").trim();
  if (!id) {
    return { text: "id is required", isError: true };
  }

  const session = sessions.get(id);
  if (!session) {
    return { text: `terminal ${id} not found`, isError: true };
  }

  session.terminal.dispose();
  session.running = false;
  session.updatedAt = Date.now();

  return { text: formatRunning(session) };
}

function handleTerminalList(): ToolResponse {
  if (sessions.size === 0) {
    return { text: "no terminal sessions" };
  }

  const lines: string[] = [];
  for (const s of sessions.values()) {
    const status = s.running ? "running" : `completed(exit=${s.exitCode})`;
    lines.push(`id=${s.id} status=${status} command=${JSON.stringify(s.command)} output_bytes=${s.outputBytes}`);
  }
  return { text: lines.join("\n") };
}

function handleTerminalLastCommand(): ToolResponse {
  if (!lastCommandText) {
    return { text: "" };
  }
  return { text: `command: ${lastCommandText}\n${lastCommandOutput}`.trim() };
}

function handleReadProblems(payload: ProblemsPayload): ToolResponse {
  const filters = collectProblemFilters(payload);
  const diagnostics = vscode.languages.getDiagnostics();

  const lines: string[] = [];
  for (const [uri, items] of diagnostics) {
    const fsPath = normalizePath(uri.fsPath);
    if (filters.length > 0 && !matchesAnyFilter(fsPath, filters)) {
      continue;
    }

    for (const d of items) {
      const sev = severityText(d.severity);
      const line = d.range.start.line + 1;
      const col = d.range.start.character + 1;
      const source = d.source ? `${d.source}: ` : "";
      const code = d.code ? ` [${typeof d.code === "string" ? d.code : d.code.value}]` : "";
      lines.push(`${fsPath}:${line}:${col}: ${sev}${code}: ${source}${d.message}`);
    }
  }

  if (lines.length === 0) {
    return { text: "no problems found" };
  }

  lines.sort();
  return { text: lines.join("\n") };
}

function collectProblemFilters(payload: ProblemsPayload): string[] {
  const out: string[] = [];
  const add = (v: string): void => {
    const t = normalizePath(v);
    if (t && !out.includes(t)) {
      out.push(t);
    }
  };

  if (payload.filePath) {
    add(resolveWorkspaceRelative(payload.filePath));
  }
  if (payload.path) {
    add(resolveWorkspaceRelative(payload.path));
  }

  for (const p of parsePathList(payload.paths)) {
    add(resolveWorkspaceRelative(p));
  }

  return out;
}

function parsePathList(v: JSONValue | undefined): string[] {
  if (typeof v === "string") {
    return v
      .replace(/[\n\r\t,]+/g, " ")
      .split(" ")
      .map((x) => x.trim())
      .filter(Boolean);
  }
  if (Array.isArray(v)) {
    return v.filter((x): x is string => typeof x === "string").map((x) => x.trim()).filter(Boolean);
  }
  return [];
}

function resolveWorkspaceRelative(p: string): string {
  if (path.isAbsolute(p)) {
    return normalizePath(p);
  }
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    return normalizePath(p);
  }
  return normalizePath(path.join(folder.uri.fsPath, p));
}

function matchesAnyFilter(filePath: string, filters: string[]): boolean {
  for (const f of filters) {
    if (filePath === f || filePath.startsWith(f + path.sep)) {
      return true;
    }
  }
  return false;
}

function severityText(s: vscode.DiagnosticSeverity): string {
  switch (s) {
    case vscode.DiagnosticSeverity.Error:
      return "error";
    case vscode.DiagnosticSeverity.Warning:
      return "warning";
    case vscode.DiagnosticSeverity.Information:
      return "info";
    case vscode.DiagnosticSeverity.Hint:
      return "hint";
    default:
      return "unknown";
  }
}

function parseEnv(raw: string): Record<string, string> {
  const out: Record<string, string> = {};
  const text = String(raw || "").trim();
  if (!text) {
    return out;
  }

  const lines = text.replace(/\r\n/g, "\n").split("\n");
  for (const line of lines) {
    const t = line.trim();
    if (!t) {
      continue;
    }
    const idx = t.indexOf("=");
    if (idx <= 0) {
      continue;
    }
    const key = t.slice(0, idx).trim();
    const value = t.slice(idx + 1);
    out[key] = value;
  }
  return out;
}

function formatRunning(s: TerminalSession): string {
  const status = s.running ? "running" : `completed(exit=${s.exitCode})`;
  let text = `terminal_id: ${s.id}\ncommand: ${s.command}\nstatus: ${status}`;
  if (s.output) {
    text += `\n${s.output}`;
  }
  return text;
}

async function readPayload(req: http.IncomingMessage): Promise<Record<string, JSONValue>> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }

  if (chunks.length === 0) {
    return {};
  }

  const body = Buffer.concat(chunks).toString("utf8").trim();
  if (!body) {
    return {};
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch {
    return {};
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return {};
  }
  return parsed as Record<string, JSONValue>;
}

function setJSONHeaders(res: http.ServerResponse): void {
  res.setHeader("Content-Type", "application/json");
  res.setHeader("Access-Control-Allow-Origin", "*");
}

function sendJSON(res: http.ServerResponse, status: number, payload: ToolResponse): void {
  res.statusCode = status;
  res.end(JSON.stringify(payload));
}

function errMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

function normalizePath(p: string): string {
  return path.normalize(String(p || "").trim());
}
