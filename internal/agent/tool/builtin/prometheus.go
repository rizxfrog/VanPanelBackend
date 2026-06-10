package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// prometheusBaseURL 从环境变量或默认值读取 Prometheus 地址。
func prometheusBaseURL() string {
	if u := os.Getenv("PROMETHEUS_URL"); u != "" {
		return u
	}
	return "http://localhost:9090"
}

// PrometheusQueryTool 通过 HTTP API 查询 Prometheus 指标。
type PrometheusQueryTool struct {
	baseURL string
	client  *http.Client
}

// NewPrometheusQueryTool 创建 PrometheusQueryTool 实例。
func NewPrometheusQueryTool() *PrometheusQueryTool {
	return &PrometheusQueryTool{
		baseURL: prometheusBaseURL(),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *PrometheusQueryTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "prometheus.query",
		Desc: fmt.Sprintf("Query Prometheus metrics using PromQL（%s）。支持即时查询和范围查询", t.baseURL),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {Type: schema.String, Desc: "PromQL 表达式，如 up, rate(http_requests_total[5m])", Required: true},
			"type":  {Type: schema.String, Desc: "查询类型: instant(即时,默认) 或 range(范围)"},
			"start": {Type: schema.String, Desc: "范围查询起始时间 (RFC3339 格式，如 2024-01-01T00:00:00Z)"},
			"end":   {Type: schema.String, Desc: "范围查询结束时间 (RFC3339 格式)"},
			"step":  {Type: schema.String, Desc: "范围查询步长，如 1m, 5m, 1h"},
		}),
	}, nil
}

func (t *PrometheusQueryTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Query string `json:"query"`
		Type  string `json:"type"`
		Start string `json:"start"`
		End   string `json:"end"`
		Step  string `json:"step"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if p.Query == "" {
		return "", fmt.Errorf("query 参数不能为空")
	}

	switch p.Type {
	case "range":
		return t.queryRange(ctx, p.Query, p.Start, p.End, p.Step)
	default:
		return t.queryInstant(ctx, p.Query)
	}
}

// promInstantResponse Prometheus 即时查询 API 响应。
type promInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [timestamp, "value"]
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

func (t *PrometheusQueryTool) queryInstant(ctx context.Context, query string) (string, error) {
	reqURL := fmt.Sprintf("%s/api/v1/query?query=%s", t.baseURL, url.QueryEscape(query))
	return t.doRequest(ctx, reqURL)
}

func (t *PrometheusQueryTool) queryRange(ctx context.Context, query, start, end, step string) (string, error) {
	if step == "" {
		step = "1m"
	}
	reqURL := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%s&end=%s&step=%s",
		t.baseURL, url.QueryEscape(query), url.PathEscape(start), url.PathEscape(end), url.PathEscape(step))
	return t.doRequest(ctx, reqURL)
}

func (t *PrometheusQueryTool) doRequest(ctx context.Context, reqURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Prometheus 请求失败 (%s): %w\n建议: 检查 PROMETHEUS_URL 环境变量和网络连通性", t.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB 限制
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Prometheus 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return t.formatResponse(body)
}

// formatResponse 格式化 Prometheus JSON 响应为可读文本。
func (t *PrometheusQueryTool) formatResponse(body []byte) (string, error) {
	var r promInstantResponse
	if err := json.Unmarshal(body, &r); err != nil {
		// 解析失败时返回原始响应
		return string(body), nil
	}

	if r.Status != "success" {
		return "", fmt.Errorf("Prometheus 查询失败: %s", r.Error)
	}

	if len(r.Data.Result) == 0 {
		return "(无数据)", nil
	}

	// 格式化输出：每行一个 metric + value
	lines := make([]string, 0, len(r.Data.Result)+1)
	lines = append(lines, fmt.Sprintf("查询结果 (%d 条):", len(r.Data.Result)))
	for _, result := range r.Data.Result {
		metricStr := metricToString(result.Metric)
		valueStr := formatPromValue(result.Value)
		if metricStr != "" {
			lines = append(lines, fmt.Sprintf("  %s => %s", metricStr, valueStr))
		} else {
			lines = append(lines, fmt.Sprintf("  %s", valueStr))
		}
	}
	return joinLines(lines), nil
}

func metricToString(m map[string]string) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		if k == "__name__" {
			parts = append([]string{v}, parts...)
		} else {
			parts = append(parts, fmt.Sprintf("%s=%q", k, v))
		}
	}
	return joinParts(parts)
}

func formatPromValue(value []interface{}) string {
	if len(value) < 2 {
		return fmt.Sprint(value)
	}
	return fmt.Sprintf("%v", value[1])
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
