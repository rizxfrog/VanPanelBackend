package guard

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// FirewallMetrics 安全相关 Prometheus 指标
type FirewallMetrics struct {
	InputChecks    *prometheus.CounterVec
	InputLatency   *prometheus.HistogramVec
	OutputChecks   *prometheus.CounterVec
	OutputLatency  *prometheus.HistogramVec
	BehaviorAlerts *prometheus.CounterVec
	RateLimitHits  *prometheus.CounterVec
	ToolCallRisk   *prometheus.CounterVec
}

var (
	defaultMetricsOnce sync.Once
	defaultMetrics     *FirewallMetrics
)

// DefaultFirewallMetrics 获取全局安全指标实例（懒初始化，线程安全）
func DefaultFirewallMetrics() *FirewallMetrics {
	defaultMetricsOnce.Do(func() {
		defaultMetrics = NewFirewallMetrics()
	})
	return defaultMetrics
}

// NewFirewallMetrics 创建安全指标
func NewFirewallMetrics() *FirewallMetrics {
	return &FirewallMetrics{
		InputChecks: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "agent_firewall_input_checks_total",
			Help: "Total input firewall checks",
		}, []string{"result", "reason"}),
		InputLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "agent_firewall_input_latency_seconds",
			Help:    "Input firewall check latency",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
		}, []string{"check_type"}),
		OutputChecks: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "agent_firewall_output_checks_total",
			Help: "Total output firewall checks",
		}, []string{"result", "action"}),
		OutputLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "agent_firewall_output_latency_seconds",
			Help:    "Output firewall check latency",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
		}, []string{}),
		BehaviorAlerts: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "agent_behavior_alerts_total",
			Help: "Total behavior anomaly alerts",
		}, []string{"pattern", "severity"}),
		RateLimitHits: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "agent_ratelimit_hits_total",
			Help: "Total rate limit hits",
		}, []string{"route", "result"}),
		ToolCallRisk: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "agent_tool_call_risk_total",
			Help: "Tool call risk level distribution",
		}, []string{"tool", "risk_level"}),
	}
}
