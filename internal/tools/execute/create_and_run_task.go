package execute

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolCreateAndRunTask = "execute_createAndRunTask"

var createAndRunTaskTool = mcp.NewTool(
	toolCreateAndRunTask,
	mcp.WithDescription("Create or update a workspace shell task in .vscode/tasks.json and run it."),
	mcp.WithString("workspaceFolder",
		mcp.Required(),
		mcp.Description("Absolute path of workspace folder where tasks.json is created."),
	),
	mcp.WithString("label",
		mcp.Required(),
		mcp.Description("Task label."),
	),
	mcp.WithString("command",
		mcp.Required(),
		mcp.Description("Shell command to run."),
	),
	mcp.WithString("args",
		mcp.Description("Optional command args. Accepts string or array-style input."),
	),
	mcp.WithString("group",
		mcp.Description("Optional task group."),
	),
	mcp.WithString("problemMatcher",
		mcp.Description("Optional problem matcher (string or array-style input)."),
	),
	mcp.WithNumber("timeout",
		mcp.Description("Optional sync timeout in milliseconds; if exceeded, returns running terminal id."),
	),
)

func registerCreateAndRunTask(s *server.MCPServer, cfg config.Provider) {
	h := policy.EnforceWithCommand(cfg, toolCreateAndRunTask, "command")(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleCreateAndRunTask(ctx, req, cfg)
	})
	h = policy.EnforceWithPaths(cfg, toolCreateAndRunTask, "workspaceFolder")(h)
	s.AddTool(createAndRunTaskTool, h)
}

func handleCreateAndRunTask(_ context.Context, req mcp.CallToolRequest, _ config.Provider) (*mcp.CallToolResult, error) {
	workspaceFolder, err := req.RequireString("workspaceFolder")
	if err != nil || strings.TrimSpace(workspaceFolder) == "" {
		return mcp.NewToolResultError("workspaceFolder is required and must be a string"), nil
	}
	label, err := req.RequireString("label")
	if err != nil || strings.TrimSpace(label) == "" {
		return mcp.NewToolResultError("label is required and must be a string"), nil
	}
	command, err := req.RequireString("command")
	if err != nil || strings.TrimSpace(command) == "" {
		return mcp.NewToolResultError("command is required and must be a string"), nil
	}

	args := parseStringListArg(req.GetArguments()["args"])
	problemMatcher := parseStringListArg(req.GetArguments()["problemMatcher"])
	group := strings.TrimSpace(req.GetString("group", ""))
	isBackground := false
	if raw, ok := req.GetArguments()["isBackground"]; ok {
		if b, ok := raw.(bool); ok {
			isBackground = b
		}
	}
	timeout := time.Duration(req.GetInt("timeout", 0)) * time.Millisecond

	tasksPath := filepath.Join(workspaceFolder, ".vscode", "tasks.json")
	if err := os.MkdirAll(filepath.Dir(tasksPath), 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create tasks directory: %v", err)), nil
	}
	tasksDoc, err := loadTasksDoc(tasksPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("load tasks.json: %v", err)), nil
	}

	task := map[string]any{
		"label":   label,
		"type":    "shell",
		"command": command,
	}
	if len(args) > 0 {
		task["args"] = args
	}
	if isBackground {
		task["isBackground"] = true
	}
	if len(problemMatcher) == 1 {
		task["problemMatcher"] = problemMatcher[0]
	} else if len(problemMatcher) > 1 {
		task["problemMatcher"] = problemMatcher
	}
	if group != "" {
		task["group"] = group
	}

	upsertTask(tasksDoc, task)
	if err := writeTasksDoc(tasksPath, tasksDoc); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write tasks.json: %v", err)), nil
	}

	fullCommand := command
	if len(args) > 0 {
		fullCommand += " " + strings.Join(shellQuoteAll(args), " ")
	}

	snap, err := terminalstate.StartWithOptions(fullCommand, terminalstate.StartOptions{Cwd: workspaceFolder})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("exec task: %v", err)), nil
	}
	if isBackground {
		return mcp.NewToolResultText("task updated at " + tasksPath + "\n" + formatRunningSnapshot(snap)), nil
	}

	snap, completed, err := terminalstate.Wait(snap.ID, timeout)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if !completed {
		return mcp.NewToolResultText("task updated at " + tasksPath + "\n" + formatRunningSnapshot(snap)), nil
	}

	result := mcp.NewToolResultText("task updated at " + tasksPath + "\n" + formatCompletedSnapshot(snap))
	if snap.HasExitCode && snap.ExitCode != 0 {
		result.IsError = true
	}
	return result, nil
}

func loadTasksDoc(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"version": "2.0.0", "tasks": []any{}}, nil
		}
		return nil, err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if _, ok := doc["version"]; !ok {
		doc["version"] = "2.0.0"
	}
	if _, ok := doc["tasks"]; !ok {
		doc["tasks"] = []any{}
	}
	return doc, nil
}

func writeTasksDoc(path string, doc map[string]any) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func upsertTask(doc map[string]any, task map[string]any) {
	raw := doc["tasks"]
	tasks, ok := raw.([]any)
	if !ok {
		tasks = []any{}
	}
	label, _ := task["label"].(string)
	for i, item := range tasks {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		existingLabel, _ := m["label"].(string)
		if existingLabel == label {
			tasks[i] = task
			doc["tasks"] = tasks
			return
		}
	}
	doc["tasks"] = append(tasks, task)
}

func shellQuoteAll(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, strconv.Quote(arg))
	}
	return out
}
