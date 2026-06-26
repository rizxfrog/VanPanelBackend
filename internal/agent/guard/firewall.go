package guard

import (
	"context"
	"regexp"
	"time"

	"go.uber.org/zap"
)

// FirewallConfig 防火墙配置
type FirewallConfig struct {
	BaseURL      string
	APIKey       string
	Model        string
	InputFilter  InputFilterConfig
	OutputFilter OutputFilterConfig
}

// InputFilterConfig 输入过滤配置
type InputFilterConfig struct {
	Enabled         bool
	JailbreakPrompt string
	InjectionPrompt string
	MaxTokens       int
}

// OutputFilterConfig 输出过滤配置
type OutputFilterConfig struct {
	Enabled        bool
	SensitiveRules []SensitivePattern
	SystemPrompt   string
	MaxTokens      int
}

// SensitivePattern 敏感信息匹配规则
type SensitivePattern struct {
	Name    string
	Pattern *regexp.Regexp
	Action  string
}

// FirewallDecision 防火墙决策
type FirewallDecision struct {
	Allowed bool
	Action  string
	Reason  string
}

// ModelFirewall 模型安全防火墙
type ModelFirewall struct {
	cfg            FirewallConfig
	jailbreakRules []*regexp.Regexp
	sensitiveRules []SensitivePattern
	logger         *zap.Logger
	metrics        *FirewallMetrics
}

// NewModelFirewall 创建模型防火墙
func NewModelFirewall(cfg FirewallConfig, metrics *FirewallMetrics) *ModelFirewall {
	fw := &ModelFirewall{
		cfg:     cfg,
		logger:  zap.NewNop(),
		metrics: metrics,
	}

	// 越狱检测规则
	fw.jailbreakRules = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(ignore|forget|disregard)\s+(previous|all|above)\s+(instructions|prompts|rules)`),
		regexp.MustCompile(`(?i)(you\s+are\s+now|act\s+as|pretend\s+to\s+be|you\s+must\s+obey)`),
		regexp.MustCompile(`(?i)(new\s+system\s+prompt|override\s+system|system:\s*you)`),
		regexp.MustCompile(`(?i)(execute\s+this\s+code|run\s+this\s+command\s+without\s+question)`),
		regexp.MustCompile(`[\x{200B}-\x{200F}\x{FEFF}\x{2060}]`),
	}

	// 敏感信息规则
	fw.sensitiveRules = []SensitivePattern{
		{"api_key", regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,}|api[_-]?key[=:]\s*['"][^'"]+['"])`), "mask"},
		{"password", regexp.MustCompile(`(?i)(password|passwd|pwd)[=:]\s*['"][^'"]+['"]`), "mask"},
		{"internal_ip", regexp.MustCompile(`(10\.\d+\.\d+\.\d+|172\.(1[6-9]|2\d|3[01])\.\d+\.\d+|192\.168\.\d+\.\d+)`), "mask"},
		{"jwt_token", regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`), "mask"},
		{"private_key", regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`), "mask"},
	}

	return fw
}

// CheckInput 检查用户输入
func (fw *ModelFirewall) CheckInput(ctx context.Context, input string) *FirewallDecision {
	start := time.Now()
	defer func() {
		if fw.metrics != nil && fw.metrics.InputLatency != nil {
			fw.metrics.InputLatency.WithLabelValues("input").Observe(time.Since(start).Seconds())
		}
	}()

	if !fw.cfg.InputFilter.Enabled {
		fw.recordInput("disabled", "input_filter_disabled")
		return &FirewallDecision{Allowed: true}
	}

	// 1. 越狱检测
	for _, re := range fw.jailbreakRules {
		if re.MatchString(input) {
			decision := &FirewallDecision{
				Allowed: false,
				Action:  "block",
				Reason:  "检测到越狱攻击: " + re.String(),
			}
			fw.recordInput("blocked", "jailbreak")
			return decision
		}
	}

	// 2. Base64 编码检测
	if hasBase64Content(input) {
		fw.recordInput("blocked", "base64")
		return &FirewallDecision{
			Allowed: false,
			Action:  "block",
			Reason:  "检测到可疑的 Base64 编码内容",
		}
	}

	fw.recordInput("allowed", "none")
	return &FirewallDecision{Allowed: true}
}

func (fw *ModelFirewall) recordInput(result, reason string) {
	if fw.metrics != nil && fw.metrics.InputChecks != nil {
		fw.metrics.InputChecks.WithLabelValues(result, reason).Inc()
	}
}

func (fw *ModelFirewall) recordOutput(result, action string) {
	if fw.metrics != nil && fw.metrics.OutputChecks != nil {
		fw.metrics.OutputChecks.WithLabelValues(result, action).Inc()
	}
}

// CheckOutput 检查 LLM 输出
func (fw *ModelFirewall) CheckOutput(ctx context.Context, output string) *FirewallDecision {
	start := time.Now()
	defer func() {
		if fw.metrics != nil && fw.metrics.OutputLatency != nil {
			fw.metrics.OutputLatency.WithLabelValues("output").Observe(time.Since(start).Seconds())
		}
	}()

	if !fw.cfg.OutputFilter.Enabled {
		fw.recordOutput("disabled", "output_filter_disabled")
		return &FirewallDecision{Allowed: true}
	}

	for _, rule := range fw.sensitiveRules {
		if rule.Pattern.MatchString(output) {
			decision := &FirewallDecision{
				Allowed: false,
				Action:  rule.Action,
				Reason:  "检测到敏感信息: " + rule.Name,
			}
			fw.recordOutput("blocked", rule.Action)
			return decision
		}
	}

	fw.recordOutput("allowed", "none")
	return &FirewallDecision{Allowed: true}
}

func hasBase64Content(s string) bool {
	// 简单检测：连续 50+ 个 Base64 字符
	b64 := regexp.MustCompile(`[A-Za-z0-9+/]{50,}={0,2}`)
	return b64.MatchString(s)
}
