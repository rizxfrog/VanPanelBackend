package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
)

type toolCallMatch struct {
	Name     string
	ArgsJSON string
}

var (
	toolCallOpen  = string([]byte{60, 124, 116, 111, 111, 108, 95, 99, 97, 108, 108, 124, 62})
	toolCallClose = string([]byte{60, 47, 124, 116, 111, 111, 108, 95, 99, 97, 108, 108, 124, 62})
	funcOpen      = string([]byte{60, 102, 117, 110, 99, 116, 105, 111, 110, 61})
	funcClose     = string([]byte{60, 47, 102, 117, 110, 99, 116, 105, 111, 110, 62})
	paramOpen     = string([]byte{60, 112, 97, 114, 97, 109, 101, 116, 101, 114, 61})
	paramClose    = string([]byte{60, 47, 112, 97, 114, 97, 109, 101, 116, 101, 114, 62})

	textToolCallRegex *regexp.Regexp
	paramRegex        *regexp.Regexp
	// Anthropic/Claude 风格的工具调用格式
	altFuncCallRegex *regexp.Regexp
	altInvokeRegex   *regexp.Regexp
	altParamRegex    *regexp.Regexp
)

func init() {
	textToolCallRegex = regexp.MustCompile("(?s)" + regexp.QuoteMeta(toolCallOpen) + "\\s*" + regexp.QuoteMeta(funcOpen) + "([^>]+)>" + "(.*?)" + regexp.QuoteMeta(funcClose) + "\\s*" + regexp.QuoteMeta(toolCallClose))
	paramRegex = regexp.MustCompile("(?s)" + regexp.QuoteMeta(paramOpen) + "([^>]+)>" + "(.*?)" + regexp.QuoteMeta(paramClose))

	// Anthropic/Claude 风格的工具调用格式
	altFuncCallRegex = regexp.MustCompile(`(?s)<function_calls>\s*(.*?)\s*</function_calls>`)
	altInvokeRegex = regexp.MustCompile(`(?s)<invoke\s+name="([^"]+)"\s*>(.*?)</invoke>`)
	altParamRegex = regexp.MustCompile(`(?s)<parameter\s+name="([^"]+)"\s*>(.*?)</parameter>`)
}

func buildToolDescriptions(tools []tool.BaseTool) string {
	var sb strings.Builder
	sb.WriteString("\n\n## 可用工具列表\n")
	sb.WriteString("当需要调用工具时，你必须严格按照以下 XML 格式输出工具调用：\n")
	sb.WriteString(" " + toolCallOpen + "\n")
	sb.WriteString("  " + funcOpen + "工具名>\n")
	sb.WriteString("  " + paramOpen + "参数名>参数值" + paramClose + "\n")
	sb.WriteString(" " + funcClose + "\n")
	sb.WriteString(" " + toolCallClose + "\n\n")
	sb.WriteString("当前可用工具：\n")
	for _, t := range tools {
		it, ok := t.(tool.InvokableTool)
		if !ok {
			continue
		}
		info, err := it.Info(context.Background())
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", info.Name, info.Desc))
	}
	return sb.String()
}

func parseTextToolCalls(content string) []toolCallMatch {
	// 优先匹配自定义格式: <|tool_call|>...<function=name>...</function>...<|/tool_call|>
	matches := textToolCallRegex.FindAllStringSubmatch(content, -1)
	if len(matches) > 0 {
		var calls []toolCallMatch
		for _, m := range matches {
			if len(m) < 3 {
				continue
			}
			call := toolCallMatch{Name: strings.TrimSpace(m[1])}
			params := paramRegex.FindAllStringSubmatch(m[2], -1)
			args := make(map[string]interface{})
			for _, p := range params {
				if len(p) >= 3 {
					args[strings.TrimSpace(p[1])] = strings.TrimSpace(p[2])
				}
			}
			if len(args) > 0 {
				b, _ := json.Marshal(args)
				call.ArgsJSON = string(b)
			} else {
				call.ArgsJSON = "{}"
			}
			calls = append(calls, call)
		}
		return calls
	}

	// 备选匹配 Anthropic/Claude XML 格式: <function_calls><invoke name="tool">...</invoke></function_calls>
	altMatches := altFuncCallRegex.FindAllStringSubmatch(content, -1)
	if len(altMatches) == 0 {
		return nil
	}
	var calls []toolCallMatch
	for _, m := range altMatches {
		if len(m) < 2 {
			continue
		}
		invokes := altInvokeRegex.FindAllStringSubmatch(m[1], -1)
		for _, inv := range invokes {
			if len(inv) < 3 {
				continue
			}
			call := toolCallMatch{Name: strings.TrimSpace(inv[1])}
			params := altParamRegex.FindAllStringSubmatch(inv[2], -1)
			args := make(map[string]interface{})
			for _, p := range params {
				if len(p) >= 3 {
					args[strings.TrimSpace(p[1])] = strings.TrimSpace(p[2])
				}
			}
			if len(args) > 0 {
				b, _ := json.Marshal(args)
				call.ArgsJSON = string(b)
			} else {
				call.ArgsJSON = "{}"
			}
			calls = append(calls, call)
		}
	}
	return calls
}

func findTool(tools []tool.BaseTool, name string) (tool.InvokableTool, bool) {
	for _, t := range tools {
		if it, ok := t.(tool.InvokableTool); ok {
			info, err := it.Info(context.Background())
			if err == nil && info.Name == name {
				return it, true
			}
		}
	}
	return nil, false
}

type textReActLoop struct {
	chatModel model.ChatModel
	tools     []tool.BaseTool
	maxStep   int
	logger    *zap.Logger
	riskEval  *risk.Evaluator
	sessionID string
	userID    uint
	username  string
	auditFn   func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string)
}

func newTextReActLoop(chatModel model.ChatModel, tools []tool.BaseTool, maxStep int, logger *zap.Logger) *textReActLoop {
	if maxStep <= 0 {
		maxStep = 10
	}
	return &textReActLoop{chatModel: chatModel, tools: tools, maxStep: maxStep, logger: logger}
}

func (l *textReActLoop) withGuard(
	riskEval *risk.Evaluator,
	sessionID string,
	userID uint,
	username string,
	auditFn func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string),
) *textReActLoop {
	l.riskEval = riskEval
	l.sessionID = sessionID
	l.userID = userID
	l.username = username
	l.auditFn = auditFn
	return l
}

func (l *textReActLoop) Generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	messages = l.injectToolPrompt(messages)
	for step := 0; step < l.maxStep; step++ {
		resp, err := l.chatModel.Generate(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("LLM 调用失败 (step %d): %w", step, err)
		}
		calls := parseTextToolCalls(resp.Content)
		if len(calls) == 0 {
			return resp, nil
		}
		l.logger.Info("检测到文本工具调用", zap.Int("step", step), zap.Int("tool_count", len(calls)))
		messages = append(messages, resp)
		for _, call := range calls {
			result := l.executeTool(ctx, call)
			messages = append(messages, &schema.Message{Role: schema.User, Content: fmt.Sprintf("[tool:%s]\n%s", call.Name, result)})
		}
	}
	return &schema.Message{Role: schema.Assistant, Content: "（已达到最大工具调用步数限制）"}, nil
}

func (l *textReActLoop) Stream(ctx context.Context, messages []*schema.Message, writer io.Writer, writeSSE func(io.Writer, string, interface{}) error) (string, error) {
	messages = l.injectToolPrompt(messages)
	var finalContent string
	for step := 0; step < l.maxStep; step++ {
		sr, err := l.chatModel.Stream(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("LLM Stream 失败 (step %d): %w", step, err)
		}
		var fullContent string
		for {
			chunk, err := sr.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				sr.Close()
				return fullContent, fmt.Errorf("Stream 接收错误 (step %d): %w", step, err)
			}
			if chunk.Content != "" {
				fullContent += chunk.Content
			}
			deltaData := map[string]interface{}{"role": "assistant", "content": chunk.Content}
			if chunk.Extra != nil {
				// 过滤 reasoning_content，不暴露 LLM 内部思考过程给前端
				filteredExtra := make(map[string]interface{}, len(chunk.Extra))
				for k, v := range chunk.Extra {
					if k != "reasoning_content" {
						filteredExtra[k] = v
					}
				}
				if len(filteredExtra) > 0 {
					deltaData["extra"] = filteredExtra
				}
			}
			if chunk.ResponseMeta != nil {
				deltaData["response_meta"] = chunk.ResponseMeta
			}
			if err := writeSSE(writer, "delta", deltaData); err != nil {
				sr.Close()
				return fullContent, err
			}
		}
		sr.Close()
		calls := parseTextToolCalls(fullContent)
		if len(calls) == 0 {
			finalContent = fullContent
			break
		}
		l.logger.Info("检测到文本工具调用（流式）", zap.Int("step", step), zap.Int("tool_count", len(calls)))
		// 通知前端工具调用，附带清理后的文本（去除 XML 标签）
		cleanText := stripToolCallXML(fullContent)
		for _, call := range calls {
			writeSSE(writer, "tool_call", map[string]interface{}{"name": call.Name, "args": call.ArgsJSON, "clean_text": cleanText})
		}
		messages = append(messages, &schema.Message{Role: schema.Assistant, Content: fullContent})
		for _, call := range calls {
			result := l.executeTool(ctx, call)
			messages = append(messages, &schema.Message{Role: schema.User, Content: fmt.Sprintf("[tool:%s]\n%s", call.Name, truncateString(result, 4000))})
			writeSSE(writer, "tool_result", map[string]interface{}{"name": call.Name, "result": truncateString(result, 2000)})
		}
	}
	return finalContent, nil
}

func (l *textReActLoop) audit(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
	if l.auditFn == nil {
		return
	}
	l.auditFn(ctx, action, toolName, reason, riskLevel, allowed, args, result)
}

func (l *textReActLoop) executeTool(ctx context.Context, call toolCallMatch) string {
	t, found := findTool(l.tools, call.Name)
	if !found {
		msg := fmt.Sprintf("工具 %s 不存在，请检查工具名称", call.Name)
		l.audit(ctx, "tool.blocked", call.Name, "工具不存在", agentmodel.RiskSafe, false, call.ArgsJSON, msg)
		return msg
	}

	// 安全校验
	if l.riskEval != nil {
		evalResult := l.riskEval.Evaluate(call.Name, call.ArgsJSON)
		l.audit(ctx, "tool.evaluate", call.Name, evalResult.Reason, agentmodel.RiskLevel(evalResult.Level), !evalResult.Blocked, call.ArgsJSON, "")

		if evalResult.Blocked {
			blockedMsg := fmt.Sprintf("[安全拦截] 操作被安全策略阻止\n原因: %s\n工具: %s\n建议: 请尝试更安全的替代方案",
				evalResult.Reason, call.Name)
			l.audit(ctx, "tool.blocked", call.Name, evalResult.Reason, agentmodel.RiskLevel(evalResult.Level), false, call.ArgsJSON, blockedMsg)
			return blockedMsg
		}
	}

	// 执行工具
	result, err := t.InvokableRun(ctx, call.ArgsJSON)
	if err != nil {
		errMsg := fmt.Sprintf("工具 %s 执行失败: %v", call.Name, err)
		l.audit(ctx, "tool.execute", call.Name, "", agentmodel.RiskSafe, true, call.ArgsJSON, errMsg)
		return errMsg
	}

	l.audit(ctx, "tool.execute", call.Name, "", agentmodel.RiskSafe, true, call.ArgsJSON, truncateString(result, 2000))
	return result
}

func (l *textReActLoop) injectToolPrompt(messages []*schema.Message) []*schema.Message {
	toolDesc := buildToolDescriptions(l.tools)
	result := make([]*schema.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == schema.System {
			result = append(result, &schema.Message{Role: schema.System, Content: m.Content + toolDesc})
		} else {
			result = append(result, m)
		}
	}
	return result
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// stripToolCallXML 从文本中移除所有工具调用 XML 标签，保留普通文本内容。
func stripToolCallXML(content string) string {
	// 移除自定义格式 <|tool_call|>...</|tool_call|>
	clean := textToolCallRegex.ReplaceAllString(content, "")
	// 移除 Anthropic 格式 <function_calls>...</function_calls>
	clean = altFuncCallRegex.ReplaceAllString(clean, "")
	return strings.TrimSpace(clean)
}
