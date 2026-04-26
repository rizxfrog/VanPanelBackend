/*
 * MIT License
 *
 * Copyright (c) 2024 Bamboo
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 */

package di

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config application configuration.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Log          LogConfig          `mapstructure:"log"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Database     DatabaseConfig     `mapstructure:"database"`
	MySQL        MySQLConfig        `mapstructure:"mysql"`
	Tree         TreeConfig         `mapstructure:"tree"`
	K8s          K8sConfig          `mapstructure:"k8s"`
	Prometheus   PrometheusConfig   `mapstructure:"prometheus"`
	Mock         MockConfig         `mapstructure:"mock"`
	Notification NotificationConfig `mapstructure:"notification"`
	Webhook      WebhookConfig      `mapstructure:"webhook"`
}

type ServerConfig struct {
	Port string `mapstructure:"port" .env:"SERVER_PORT" default:"8889"`
}

type LogConfig struct {
	Dir   string `mapstructure:"dir" .env:"LOG_DIR" default:"./logs"`
	Level string `mapstructure:"level" .env:"LOG_LEVEL" default:"debug"`
}

type JWTConfig struct {
	Key1       string `mapstructure:"key1" .env:"JWT_KEY1" default:"ebe3vxIP7sblVvUHXb7ZaiMPuz4oXo0l"`
	Key2       string `mapstructure:"key2" .env:"JWT_KEY2" default:"ebe3vxIP7sblVvUHXb7ZaiMPuz4oXo0z"`
	Issuer     string `mapstructure:"issuer" .env:"JWT_ISSUER" default:"K5mBPBYNQeNWEBvCTE5msog3KSGTdhmx"`
	Expiration int64  `mapstructure:"expiration" .env:"JWT_EXPIRATION" default:"3600"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr" .env:"REDIS_ADDR" default:"localhost:6379"`
	Password string `mapstructure:"password" .env:"REDIS_PASSWORD" default:""`
}

// DatabaseConfig is the primary runtime database configuration.
type DatabaseConfig struct {
	Driver string `mapstructure:"driver" .env:"DATABASE_DRIVER" default:"mysql"`
	DSN    string `mapstructure:"dsn" .env:"DATABASE_DSN" default:"root:root@tcp(localhost:3306)/cloudops?charset=utf8mb4&parseTime=True&loc=Local"`
}

// MySQLConfig is kept for backward-compatible config fallback.
type MySQLConfig struct {
	Addr string `mapstructure:"addr" .env:"MYSQL_ADDR" default:"root:root@tcp(localhost:3306)/cloudops?charset=utf8mb4&parseTime=True&loc=Local"`
}

type TreeConfig struct {
	CheckStatusCron       string `mapstructure:"check_status_cron" .env:"TREE_CHECK_STATUS_CRON" default:"@every 300s"`
	PasswordEncryptionKey string `mapstructure:"password_encryption_key" .env:"TREE_PASSWORD_ENCRYPTION_KEY" default:"ebe3vxIP7sblVvUHXb7ZaiMPuz4oXo0l"`
}

type K8sConfig struct {
	RefreshCron string `mapstructure:"refresh_cron" .env:"K8S_REFRESH_CRON" default:"@every 300s"`
}

type PrometheusConfig struct {
	RefreshCron         string `mapstructure:"refresh_cron" .env:"PROMETHEUS_REFRESH_CRON" default:"@every 15s"`
	EnableAlert         int    `mapstructure:"enable_alert" .env:"PROMETHEUS_ENABLE_ALERT" default:"0"`
	EnableRecord        int    `mapstructure:"enable_record" .env:"PROMETHEUS_ENABLE_RECORD" default:"0"`
	AlertWebhookAddr    string `mapstructure:"alert_webhook_addr" .env:"PROMETHEUS_ALERT_WEBHOOK_ADDR" default:"http://localhost:8889/api/v1/alerts/receive"`
	AlertWebhookFileDir string `mapstructure:"alert_webhook_file_dir" .env:"PROMETHEUS_ALERT_WEBHOOK_FILE_DIR" default:"/tmp/webhook_files"`
	HttpSdAPI           string `mapstructure:"httpSdAPI" .env:"PROMETHEUS_HTTP_SD_API" default:"http://localhost:8888/api/not_auth/getTreeNodeBindIps"`
}

type MockConfig struct {
	Enabled bool `mapstructure:"enabled" .env:"MOCK_ENABLED" default:"true"`
}

type NotificationConfig struct {
	Email  *EmailConfig  `mapstructure:"email"`
	Feishu *FeishuConfig `mapstructure:"feishu"`
}

func (c *NotificationConfig) GetEmail() *EmailConfig {
	return c.Email
}

func (c *NotificationConfig) GetFeishu() *FeishuConfig {
	return c.Feishu
}

type EmailConfig struct {
	Enabled       bool   `mapstructure:"enabled" .env:"NOTIFICATION_EMAIL_ENABLED" default:"false"`
	SMTPHost      string `mapstructure:"smtp_host" .env:"NOTIFICATION_EMAIL_SMTP_HOST" default:"smtp.gmail.com"`
	SMTPPort      int    `mapstructure:"smtp_port" .env:"NOTIFICATION_EMAIL_SMTP_PORT" default:"587"`
	Username      string `mapstructure:"username" .env:"NOTIFICATION_EMAIL_USERNAME" default:""`
	Password      string `mapstructure:"password" .env:"NOTIFICATION_EMAIL_PASSWORD" default:""`
	FromName      string `mapstructure:"from_name" .env:"NOTIFICATION_EMAIL_FROM_NAME" default:"AI-CloudOps"`
	MaxRetries    int    `mapstructure:"max_retries" .env:"NOTIFICATION_EMAIL_MAX_RETRIES" default:"3"`
	RetryInterval string `mapstructure:"retry_interval" .env:"NOTIFICATION_EMAIL_RETRY_INTERVAL" default:"5m"`
	Timeout       string `mapstructure:"timeout" .env:"NOTIFICATION_EMAIL_TIMEOUT" default:"30s"`
	UseTLS        bool   `mapstructure:"use_tls" .env:"NOTIFICATION_EMAIL_USE_TLS" default:"true"`
}

func (c *EmailConfig) IsEnabled() bool {
	return viper.GetBool("notification.email.enabled")
}

func (c *EmailConfig) GetMaxRetries() int {
	retries := viper.GetInt("notification.email.max_retries")
	if retries <= 0 {
		return 3
	}
	return retries
}

func (c *EmailConfig) GetRetryInterval() time.Duration {
	interval := viper.GetString("notification.email.retry_interval")
	if interval == "" {
		return 5 * time.Minute
	}
	if d, err := time.ParseDuration(interval); err == nil {
		return d
	}
	return 5 * time.Minute
}

func (c *EmailConfig) GetTimeout() time.Duration {
	timeout := viper.GetString("notification.email.timeout")
	if timeout == "" {
		return 30 * time.Second
	}
	if d, err := time.ParseDuration(timeout); err == nil {
		return d
	}
	return 30 * time.Second
}

func (c *EmailConfig) GetChannelName() string {
	return "email"
}

func (c *EmailConfig) Validate() error {
	if !c.IsEnabled() {
		return nil
	}
	if viper.GetString("notification.email.smtp_host") == "" {
		return fmt.Errorf("SMTP host is required")
	}
	port := viper.GetInt("notification.email.smtp_port")
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid SMTP port: %d", port)
	}
	if viper.GetString("notification.email.username") == "" {
		return fmt.Errorf("username is required")
	}
	if viper.GetString("notification.email.password") == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

func (c *EmailConfig) GetSMTPHost() string {
	return viper.GetString("notification.email.smtp_host")
}

func (c *EmailConfig) GetSMTPPort() int {
	return viper.GetInt("notification.email.smtp_port")
}

func (c *EmailConfig) GetUsername() string {
	return viper.GetString("notification.email.username")
}

func (c *EmailConfig) GetPassword() string {
	return viper.GetString("notification.email.password")
}

func (c *EmailConfig) GetFromName() string {
	fromName := viper.GetString("notification.email.from_name")
	if fromName == "" {
		return "AI-CloudOps"
	}
	return fromName
}

func (c *EmailConfig) GetUseTLS() bool {
	return viper.GetBool("notification.email.use_tls")
}

type FeishuConfig struct {
	Enabled              bool   `mapstructure:"enabled" .env:"NOTIFICATION_FEISHU_ENABLED" default:"false"`
	AppID                string `mapstructure:"app_id" .env:"NOTIFICATION_FEISHU_APP_ID" default:""`
	AppSecret            string `mapstructure:"app_secret" .env:"NOTIFICATION_FEISHU_APP_SECRET" default:""`
	WebhookURL           string `mapstructure:"webhook_url" .env:"NOTIFICATION_FEISHU_WEBHOOK_URL" default:"https://open.feishu.cn/open-apis/bot/v2/hook/"`
	PrivateMessageAPI    string `mapstructure:"private_message_api" .env:"NOTIFICATION_FEISHU_PRIVATE_MESSAGE_API" default:"https://open.feishu.cn/open-apis/im/v1/messages"`
	TenantAccessTokenAPI string `mapstructure:"tenant_access_token_api" .env:"NOTIFICATION_FEISHU_TENANT_ACCESS_TOKEN_API" default:"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"`
	MaxRetries           int    `mapstructure:"max_retries" .env:"NOTIFICATION_FEISHU_MAX_RETRIES" default:"3"`
	RetryInterval        string `mapstructure:"retry_interval" .env:"NOTIFICATION_FEISHU_RETRY_INTERVAL" default:"5m"`
	Timeout              string `mapstructure:"timeout" .env:"NOTIFICATION_FEISHU_TIMEOUT" default:"10s"`
}

func (c *FeishuConfig) IsEnabled() bool {
	return viper.GetBool("notification.feishu.enabled")
}

func (c *FeishuConfig) GetMaxRetries() int {
	retries := viper.GetInt("notification.feishu.max_retries")
	if retries <= 0 {
		return 3
	}
	return retries
}

func (c *FeishuConfig) GetRetryInterval() time.Duration {
	interval := viper.GetString("notification.feishu.retry_interval")
	if interval == "" {
		return 5 * time.Minute
	}
	if d, err := time.ParseDuration(interval); err == nil {
		return d
	}
	return 5 * time.Minute
}

func (c *FeishuConfig) GetTimeout() time.Duration {
	timeout := viper.GetString("notification.feishu.timeout")
	if timeout == "" {
		return 10 * time.Second
	}
	if d, err := time.ParseDuration(timeout); err == nil {
		return d
	}
	return 10 * time.Second
}

func (c *FeishuConfig) GetChannelName() string {
	return "feishu"
}

func (c *FeishuConfig) Validate() error {
	if !c.IsEnabled() {
		return nil
	}
	if viper.GetString("notification.feishu.app_id") == "" {
		return fmt.Errorf("app_id is required")
	}
	if viper.GetString("notification.feishu.app_secret") == "" {
		return fmt.Errorf("app_secret is required")
	}
	if viper.GetString("notification.feishu.webhook_url") == "" {
		return fmt.Errorf("webhook_url is required")
	}
	if viper.GetString("notification.feishu.private_message_api") == "" {
		return fmt.Errorf("private_message_api is required")
	}
	if viper.GetString("notification.feishu.tenant_access_token_api") == "" {
		return fmt.Errorf("tenant_access_token_api is required")
	}
	return nil
}

func (c *FeishuConfig) GetAppID() string {
	return viper.GetString("notification.feishu.app_id")
}

func (c *FeishuConfig) GetAppSecret() string {
	return viper.GetString("notification.feishu.app_secret")
}

func (c *FeishuConfig) GetWebhookURL() string {
	return viper.GetString("notification.feishu.webhook_url")
}

func (c *FeishuConfig) GetPrivateMessageAPI() string {
	return viper.GetString("notification.feishu.private_message_api")
}

func (c *FeishuConfig) GetTenantAccessTokenAPI() string {
	return viper.GetString("notification.feishu.tenant_access_token_api")
}

type WebhookConfig struct {
	Port                          string         `mapstructure:"port" .env:"WEBHOOK_PORT" default:"8888"`
	FixedWorkers                  int            `mapstructure:"fixed_workers" .env:"WEBHOOK_FIXED_WORKERS" default:"10"`
	FrontDomain                   string         `mapstructure:"front_domain" .env:"WEBHOOK_FRONT_DOMAIN" default:"http://localhost:3000"`
	BackendDomain                 string         `mapstructure:"backend_domain" .env:"WEBHOOK_BACKEND_DOMAIN" default:"http://localhost:8889"`
	DefaultUpgradeMinutes         int            `mapstructure:"default_upgrade_minutes" .env:"WEBHOOK_DEFAULT_UPGRADE_MINUTES" default:"60"`
	AlertManagerAPI               string         `mapstructure:"alert_manager_api" .env:"WEBHOOK_ALERT_MANAGER_API" default:"http://localhost:9093"`
	CommonMapRenewIntervalSeconds int            `mapstructure:"common_map_renew_interval_seconds" .env:"WEBHOOK_COMMON_MAP_RENEW_INTERVAL_SECONDS" default:"300"`
	ImFeishu                      ImFeishuConfig `mapstructure:"im_feishu"`
}

type ImFeishuConfig struct {
	GroupMessageAPI       string `mapstructure:"group_message_api" .env:"WEBHOOK_IM_FEISHU_GROUP_MESSAGE_API" default:"https://open.feishu.cn/open-apis/im/v1/messages"`
	RequestTimeoutSeconds int    `mapstructure:"request_timeout_seconds" .env:"WEBHOOK_IM_FEISHU_REQUEST_TIMEOUT_SECONDS" default:"10"`
	PrivateRobotAppID     string `mapstructure:"private_robot_app_id" .env:"WEBHOOK_IM_FEISHU_PRIVATE_ROBOT_APP_ID" default:""`
	PrivateRobotAppSecret string `mapstructure:"private_robot_app_secret" .env:"WEBHOOK_IM_FEISHU_PRIVATE_ROBOT_APP_SECRET" default:""`
	TenantAccessTokenAPI  string `mapstructure:"tenant_access_token_api" .env:"WEBHOOK_IM_FEISHU_TENANT_ACCESS_TOKEN_API" default:"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"`
}

type LLMConfig struct {
	APIKey  string `.env:"LLM_API_KEY" default:""`
	BaseURL string `.env:"LLM_BASE_URL" default:""`
}

type AliyunConfig struct {
	AccessKeyID     string `.env:"ALIYUN_ACCESS_KEY_ID" default:""`
	AccessKeySecret string `.env:"ALIYUN_ACCESS_KEY_SECRET" default:""`
}

type TavilyConfig struct {
	APIKey string `.env:"TAVILY_API_KEY" default:""`
}

type ExternalConfig struct {
	LLM    LLMConfig    `mapstructure:"llm"`
	Aliyun AliyunConfig `mapstructure:"aliyun"`
	Tavily TavilyConfig `mapstructure:"tavily"`
}

var GlobalConfig = &Config{}
var GlobalExternalConfig = &ExternalConfig{}
