package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ContainerInspectTool 检测 Docker/Podman 容器状态。
type ContainerInspectTool struct {
	runtime string // "docker" or "podman" (detected at startup)
	timeout time.Duration
}

// NewContainerInspectTool 创建 ContainerInspectTool 实例。
// 自动检测 docker/podman，优先使用 docker。
func NewContainerInspectTool() *ContainerInspectTool {
	t := &ContainerInspectTool{
		runtime: detectContainerRuntime(),
		timeout: 30 * time.Second,
	}
	return t
}

// detectContainerRuntime 检测可用的容器运行时。
func detectContainerRuntime() string {
	for _, bin := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(bin); err == nil {
			return bin
		}
	}
	return ""
}

func (t *ContainerInspectTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	desc := t.buildDescription()
	return &schema.ToolInfo{
		Name: "container.inspect",
		Desc: desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action":    {Type: schema.String, Desc: "操作类型: ps(列表), inspect(详情), stats(资源), logs(日志)"},
			"container": {Type: schema.String, Desc: "容器名称或 ID（inspect/logs 需要）"},
		}),
	}, nil
}

func (t *ContainerInspectTool) buildDescription() string {
	if t.runtime == "" {
		return "Inspect Docker/Podman containers（未检测到容器运行时，请安装 Docker 或 Podman）"
	}
	return fmt.Sprintf("Inspect %s containers（列表、详情、资源、日志）", t.runtime)
}

func (t *ContainerInspectTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	if t.runtime == "" {
		return "", fmt.Errorf("未检测到 Docker 或 Podman，请安装容器运行时")
	}

	var p struct {
		Action    string `json:"action"`
		Container string `json:"container"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		// 参数解析失败时默认执行 ps
		p.Action = "ps"
	}

	cmdLine := t.buildCommand(p.Action, p.Container)
	return t.runContainerCmd(ctx, cmdLine)
}

func (t *ContainerInspectTool) buildCommand(action, container string) string {
	switch action {
	case "inspect":
		return fmt.Sprintf("%s inspect %s", t.runtime, container)
	case "stats":
		return fmt.Sprintf("%s stats --no-stream --all", t.runtime)
	case "logs":
		if container == "" {
			return fmt.Sprintf("%s ps -a", t.runtime)
		}
		return fmt.Sprintf("%s logs --tail 200 %s", t.runtime, container)
	case "ps", "":
		return fmt.Sprintf("%s ps -a", t.runtime)
	default:
		return fmt.Sprintf("%s %s", t.runtime, action)
	}
}

// runContainerCmd 执行容器命令（复用 findShell 逻辑但使用动态命令）。
func (t *ContainerInspectTool) runContainerCmd(ctx context.Context, cmdLine string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	shell, shellFlag, err := findShell()
	if err != nil {
		return "", err
	}

	// Windows 不支持容器命令
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("container.inspect 不支持 Windows，请在 Linux 环境使用")
	}

	cmd := exec.CommandContext(ctx, shell, shellFlag, cmdLine)
	output, err := runCmdCapture(cmd, 65536)
	if err != nil {
		return "", err
	}
	return output, nil
}

// runCmdCapture 执行命令并截断输出（与 baseCommandTool 复用逻辑）。
func runCmdCapture(cmd *exec.Cmd, maxOutput int) (string, error) {
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("命令执行失败: %s: %w", stderr.String(), err)
	}

	output := stdout.String()
	if maxOutput > 0 && len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (输出已截断)"
	}
	return output, nil
}
