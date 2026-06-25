package runtime

import (
	"context"
	"regexp"

	"go.uber.org/zap"
)

// SanitizedResult 脱敏后的结果
type SanitizedResult struct {
	SafeContent    string
	MaskedCount    int
	InjectDetected bool
	InjectReason   string
	Untrusted      bool
}

// SensitivePattern 敏感信息匹配规则
type SensitivePattern struct {
	Name    string
	Pattern *regexp.Regexp
	Action  string // "mask" / "remove"
}

// ToolResultSanitizer 工具结果脱敏器
type ToolResultSanitizer struct {
	sensitiveRules []SensitivePattern
	injectPatterns []*regexp.Regexp
	logger         *zap.Logger
}

// NewToolResultSanitizer 创建脱敏器
func NewToolResultSanitizer(logger *zap.Logger) *ToolResultSanitizer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ToolResultSanitizer{
		logger: logger,
		sensitiveRules: []SensitivePattern{
			{Name: "api_key", Pattern: regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,}|api[_-]?key[=:]\s*['"][^'"]+['"])`), Action: "mask"},
			{Name: "password", Pattern: regexp.MustCompile(`(?i)(password|passwd|pwd)[=:]\s*['"][^'"]+['"]`), Action: "mask"},
			{Name: "internal_ip", Pattern: regexp.MustCompile(`(10\.\d+\.\d+\.\d+|172\.(1[6-9]|2\d|3[01])\.\d+\.\d+|192\.168\.\d+\.\d+)`), Action: "mask"},
			{Name: "jwt_token", Pattern: regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`), Action: "mask"},
			{Name: "private_key", Pattern: regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`), Action: "mask"},
			{Name: "aws_key", Pattern: regexp.MustCompile(`(?i)(AKIA[0-9A-Z]{16})`), Action: "mask"},
			{Name: "db_connection", Pattern: regexp.MustCompile(`(?i)(mysql|postgres|mongodb)://[^:]+:[^@]+@[^/]+`), Action: "mask"},
		},
		injectPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(ignore|forget|disregard)\s+(previous|all|above)\s+(instructions|prompts|rules)`),
			regexp.MustCompile(`(?i)(you\s+are\s+now|act\s+as|pretend\s+to\s+be|you\s+must\s+obey)`),
			regexp.MustCompile(`(?i)(new\s+system\s+prompt|override\s+system|system:\s*you)`),
			regexp.MustCompile(`(?i)(from\s+now\s+on|starting\s+now|your\s+new\s+role)`),
			regexp.MustCompile(`(?i)(execute\s+this|run\s+this\s+command|call\s+this\s+function)`),
			regexp.MustCompile(`[\x{200B}-\x{200F}\x{FEFF}\x{2060}]`),
		},
	}
}

// Sanitize 对工具输出进行脱敏和注入检测
func (s *ToolResultSanitizer) Sanitize(ctx context.Context, output string) *SanitizedResult {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx

	result := &SanitizedResult{SafeContent: output, Untrusted: true}

	// 1. 检测注入指令
	for _, pattern := range s.injectPatterns {
		if pattern.MatchString(output) {
			result.InjectDetected = true
			result.InjectReason = "检测到指令型内容: " + pattern.String()
			result.SafeContent = "[UNTRUSTED_TOOL_RESULT]\n" + output + "\n[/UNTRUSTED_TOOL_RESULT]"
			return result
		}
	}

	// 2. 脱敏敏感信息
	for _, rule := range s.sensitiveRules {
		if rule.Pattern.MatchString(result.SafeContent) {
			result.SafeContent = rule.Pattern.ReplaceAllString(result.SafeContent, "***")
			result.MaskedCount++
		}
	}

	return result
}
