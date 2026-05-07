package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

var ErrToolNotFound = errors.New("agent tool not found")

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]any) (map[string]any, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	registry := &Registry{tools: make(map[string]Tool)}
	for _, tool := range []Tool{
		systemInspectTool{},
		staticTool{name: "process.list", description: "List running processes summary", payload: map[string]any{"message": "process listing is not wired in MVP"}},
		staticTool{name: "disk.analyze", description: "Analyze disk usage", payload: map[string]any{"message": "disk analyzer is not wired in MVP"}},
		staticTool{name: "log.query", description: "Query logs", payload: map[string]any{"message": "log query is not wired in MVP"}},
		staticTool{name: "file.scan", description: "Scan files", payload: map[string]any{"message": "file scan is not wired in MVP"}},
		terminalSuggestTool{},
		staticTool{name: "container.inspect", description: "Inspect containers", payload: map[string]any{"message": "container inspect is not wired in MVP"}},
		staticTool{name: "prometheus.query", description: "Query Prometheus", payload: map[string]any{"message": "prometheus query is not wired in MVP"}},
	} {
		registry.Register(tool)
	}
	return registry
}

func (r *Registry) Register(tool Tool) {
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	r.tools[tool.Name()] = tool
}

func (r *Registry) List() []map[string]string {
	out := make([]map[string]string, 0, len(r.tools))
	for _, tool := range r.tools {
		out = append(out, map[string]string{
			"name":        tool.Name(),
			"description": tool.Description(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"] < out[j]["name"] })
	return out
}

func (r *Registry) Execute(ctx context.Context, call agentmodel.ToolCall) (agentmodel.ToolResult, error) {
	tool, ok := r.tools[call.Name]
	if !ok {
		return agentmodel.ToolResult{ToolName: call.Name, Error: ErrToolNotFound.Error()}, ErrToolNotFound
	}

	output, err := tool.Execute(ctx, call.Args)
	result := agentmodel.ToolResult{ToolName: call.Name, Output: output}
	if err != nil {
		result.Error = err.Error()
	}
	return result, err
}

type systemInspectTool struct{}

func (systemInspectTool) Name() string { return "system.inspect" }

func (systemInspectTool) Description() string {
	return "Inspect current backend host runtime and basic process information"
}

func (systemInspectTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	hostname, _ := os.Hostname()
	return map[string]any{
		"hostname":  hostname,
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"goVersion": runtime.Version(),
		"pid":       os.Getpid(),
		"time":      time.Now().Format(time.RFC3339),
	}, nil
}

type terminalSuggestTool struct{}

func (terminalSuggestTool) Name() string { return "terminal.suggest" }

func (terminalSuggestTool) Description() string {
	return "Return a terminal command suggestion without executing it"
}

func (terminalSuggestTool) Execute(_ context.Context, args map[string]any) (map[string]any, error) {
	command := strings.TrimSpace(fmt.Sprint(args["command"]))
	if command == "" {
		command = "uname -a"
	}
	return map[string]any{
		"command": command,
		"notice":  "suggestion only; command was not executed",
	}, nil
}

type staticTool struct {
	name        string
	description string
	payload     map[string]any
}

func (t staticTool) Name() string { return t.name }

func (t staticTool) Description() string { return t.description }

func (t staticTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(t.payload))
	for key, value := range t.payload {
		out[key] = value
	}
	return out, nil
}
