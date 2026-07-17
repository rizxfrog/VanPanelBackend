package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// fileService 通过 DI 注入的 FileService（可为 nil，回退到 shell 命令）。
var fileService interface {
	List(ctx context.Context, req struct {
		Path       string
		Page       int
		Size       int
		Search     string
		ShowHidden bool
	}) (interface{ List() interface{} }, error)
	Content(ctx context.Context, req struct{ Path string }) (map[string]interface{}, error)
}

// FileScanTool 扫描文件和目录（支持列表、搜索、内容读取）。
type FileScanTool struct {
	baseCommandTool
}

// NewFileScanTool 创建 FileScanTool 实例。
// 如果通过 SetFileServiceAndFS 注入了 FileService，优先使用它；
// 否则回退到 find/ls 等 shell 命令。
func NewFileScanTool() *FileScanTool {
	return &FileScanTool{baseCommandTool{
		name:        "file.scan",
		description: "Scan files and directories（列表、搜索、内容读取）",
		command:     "", // 回退到 shell 命令时动态构建
		timeout:     30 * time.Second,
		maxOutput:   65536,
	}}
}

// assert *FileScanTool implements tool.BaseTool
var _ tool.BaseTool = (*FileScanTool)(nil)

func (t *FileScanTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "file.scan",
		Desc: "Scan files and directories（列表、搜索、内容读取）" +
			"\n- action=list: 列出指定目录下的文件" +
			"\n- action=search: 按名称模式搜索文件" +
			"\n- action=read: 读取文件内容",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action":  {Type: schema.String, Desc: "操作类型: list(列出目录), search(搜索), read(读取)", Required: true},
			"path":    {Type: schema.String, Desc: "路径（目录或文件路径）", Required: true},
			"pattern": {Type: schema.String, Desc: "搜索模式（search 操作时使用，支持 glob，如 *.log）"},
		}),
	}, nil
}

func (t *FileScanTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Action  string `json:"action"`
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	switch p.Action {
	case "search":
		return t.search(ctx, p.Path, p.Pattern)
	case "read":
		return t.readFile(ctx, p.Path)
	case "list", "":
		return t.listDir(ctx, p.Path)
	default:
		return "", fmt.Errorf("不支持的操作: %s (支持: list, search, read)", p.Action)
	}
}

// listDir 列出目录内容。
func (t *FileScanTool) listDir(ctx context.Context, path string) (string, error) {
	path = normalizePath(path)
	return t.runCommand(ctx, "-la "+escapePath(path))
}

// search 按名称模式搜索文件。
func (t *FileScanTool) search(ctx context.Context, path, pattern string) (string, error) {
	path = normalizePath(path)
	if pattern == "" {
		pattern = "*"
	}
	opt := "-maxdepth 3"
	if path == "/" {
		opt += " -xdev" // 避免遍历挂载点
	}
	cmd := fmt.Sprintf("%s -name %s -type f 2>/dev/null | head -100", escapePath(path), pattern)
	return t.runCommand(ctx, opt+" "+cmd)
}

// readFile 读取文件内容（前 1000 行）。
func (t *FileScanTool) readFile(ctx context.Context, path string) (string, error) {
	path = normalizePath(path)
	// 使用 find 后接 head 安全读取文件内容，限制大小
	return t.runCommand(ctx, "head -n 1000 "+escapePath(path))
}

func normalizePath(path string) string {
	if path == "" {
		return "."
	}
	return path
}

// escapePath 对路径进行基础 shell 转义，防止命令注入。
func escapePath(path string) string {
	// 移除危险字符，保留安全的路径字符
	dangerous := []string{";", "|", "&", "`", "$", "(", ")", "{", "}", ">", "<", "\n", "\r", "\\"}
	result := strings.TrimSpace(path)
	for _, c := range dangerous {
		result = strings.ReplaceAll(result, c, "")
	}
	if result == "" {
		return "."
	}
	return result
}
