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

import ()

// Config application configuration.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Log         LogConfig         `mapstructure:"log"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Database    DatabaseConfig    `mapstructure:"database"`
	MySQL       MySQLConfig       `mapstructure:"mysql"`
	Mock        MockConfig        `mapstructure:"mock"`
	FileManager FileManagerConfig `mapstructure:"file_manager"`
	Agent       AgentConfig       `mapstructure:"agent"`
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

type MockConfig struct {
	Enabled bool `mapstructure:"enabled" .env:"MOCK_ENABLED" default:"true"`
}

type FileManagerRootConfig struct {
	Name string `mapstructure:"name" .env:"FILE_MANAGER_ROOT_NAME" default:"VanPanel"`
	Path string `mapstructure:"path" .env:"FILE_MANAGER_ROOT_PATH" default:"."`
}

type FileManagerConfig struct {
	Enabled             bool                    `mapstructure:"enabled" .env:"FILE_MANAGER_ENABLED" default:"true"`
	AllowFullDisk       bool                    `mapstructure:"allow_full_disk" .env:"FILE_MANAGER_ALLOW_FULL_DISK" default:"false"`
	Roots               []FileManagerRootConfig `mapstructure:"roots"`
	MaxEditSizeMB       int                     `mapstructure:"max_edit_size_mb" .env:"FILE_MANAGER_MAX_EDIT_SIZE_MB" default:"5"`
	MaxPreviewSizeMB    int                     `mapstructure:"max_preview_size_mb" .env:"FILE_MANAGER_MAX_PREVIEW_SIZE_MB" default:"10"`
	AllowedArchiveTypes []string                `mapstructure:"allowed_archive_types"`
}

type AgentConfig struct {
	LLM        AgentLLMConfig  `mapstructure:"llm"`
	Risk       AgentRiskConfig `mapstructure:"risk"`
	Hub        AgentHubConfig  `mapstructure:"hub"`
	MaxHistory int             `mapstructure:"max_history" .env:"AGENT_MAX_HISTORY" default:"20"`
}

type AgentLLMConfig struct {
	Provider    string  `mapstructure:"provider" .env:"AGENT_LLM_PROVIDER" default:"openai"`
	BaseURL     string  `mapstructure:"base_url" .env:"AGENT_LLM_BASE_URL" default:"https://api.openai.com/v1"`
	APIKey      string  `mapstructure:"api_key" .env:"AGENT_LLM_API_KEY" default:""`
	Model       string  `mapstructure:"model" .env:"AGENT_LLM_MODEL" default:"gpt-4o"`
	Temperature float64 `mapstructure:"temperature" .env:"AGENT_LLM_TEMPERATURE" default:"0.7"`
	MaxTokens   int     `mapstructure:"max_tokens" .env:"AGENT_LLM_MAX_TOKENS" default:"4096"`
}

type AgentRiskConfig struct {
	HighRiskPatterns []string         `mapstructure:"high_risk_patterns"`
	ProtectedPaths   []string         `mapstructure:"protected_paths"`
	ApprovalTimeout  string           `mapstructure:"approval_timeout" .env:"AGENT_RISK_APPROVAL_TIMEOUT" default:"10m"`
	Shell            AgentShellConfig `mapstructure:"shell"`
}

type AgentShellConfig struct {
	DefaultRisk    string   `mapstructure:"default_risk" .env:"AGENT_SHELL_DEFAULT_RISK" default:"low"`
	Timeout        string   `mapstructure:"timeout" .env:"AGENT_SHELL_TIMEOUT" default:"30s"`
	MaxOutputBytes int      `mapstructure:"max_output_bytes" .env:"AGENT_SHELL_MAX_OUTPUT" default:"65536"`
	Blacklist      []string `mapstructure:"blacklist"`
	Whitelist      []string `mapstructure:"whitelist"`
}

type AgentHubConfig struct {
	PluginDir            string `mapstructure:"plugin_dir" .env:"AGENT_HUB_PLUGIN_DIR" default:"./data/plugins"`
	MaxPluginSize        int    `mapstructure:"max_plugin_size" .env:"AGENT_HUB_MAX_SIZE" default:"52428800"`
	MaxConcurrentPlugins int    `mapstructure:"max_concurrent_plugins" .env:"AGENT_HUB_MAX_CONCURRENT" default:"10"`
}

var GlobalConfig = &Config{}
