package builtin

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// SystemInspectTool 查看当前系统运行时信息（hostname/os/arch/pid）。
type SystemInspectTool struct{}

// NewSystemInspectTool 创建 SystemInspectTool 实例。
func NewSystemInspectTool() *SystemInspectTool {
	return &SystemInspectTool{}
}

func (t *SystemInspectTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "sys.inspect",
		Desc: "查看当前 Agent 宿主机的运行时信息（主机名、操作系统、架构、进程ID、Go版本）",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"detail": {Type: schema.String, Desc: "可选：full 显示完整信息，summary 显示摘要"},
		}),
	}, nil
}

func (t *SystemInspectTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	hostname, _ := os.Hostname()
	now := time.Now().Format(time.RFC3339)

	return formatKV(
		"hostname", hostname,
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"pid", os.Getpid(),
		"goVersion", runtime.Version(),
		"time", now,
	), nil
}

func formatKV(kv ...interface{}) string {
	result := ""
	for i := 0; i < len(kv); i += 2 {
		if i > 0 {
			result += "\n"
		}
		result += kv[i].(string) + ": " + formatVal(kv[i+1])
	}
	return result
}

func formatVal(v interface{}) string {
	switch val := v.(type) {
	case int:
		return intToString(val)
	case string:
		return val
	default:
		return ""
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
