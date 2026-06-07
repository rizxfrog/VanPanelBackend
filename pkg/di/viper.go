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
	"os"
	"reflect"
	"strings"

	"github.com/scaleway/scaleway-sdk-go/logger"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// InitViper 初始化viper配置，支持环境变量优先级：环境变量 > 配置文件 > 默认值
func InitViper() error {
	// 支持通过命令行参数 --config 指定任意配置文件
	configFile := pflag.String("config", "", "配置文件路径")
	pflag.Parse()

	// 如果未通过命令行指定，则根据环境变量ENV选择默认配置文件
	if *configFile == "" {
		env := os.Getenv("ENV")
		if env == "" {
			env = "development"
		}
		switch env {
		case "production":
			logger.Infof("ENV is set to 'production', using production config file")
			*configFile = "config/config.production.yaml"
		case "test":
			logger.Infof("ENV is set to 'test', using test config file")
			*configFile = "config/config.test.yaml"
		default:
			logger.Infof("ENV is set to 'development', using development config file")
			*configFile = "config/config.development.yaml"
		}
	}

	// 设置配置文件类型和路径
	viper.SetConfigFile(*configFile)

	// 设置默认值（最低优先级）
	setDefaults()

	// 启用环境变量支持
	viper.AutomaticEnv()

	// 将点号替换为下划线以支持嵌套配置的环境变量
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 读取配置文件（中等优先级）
	if err := viper.ReadInConfig(); err != nil {
		// 如果配置文件不存在，只是打印警告，继续使用环境变量和默认值
		fmt.Printf("Warning: Failed to read config file %s: %v\n", *configFile, err)
		fmt.Println("Using environment variables and default values only.")
	}

	// 绑定环境变量（最高优先级）- 必须在读取配置文件之后
	bindEnvVars()

	// 加载配置到全局变量
	if err := viper.Unmarshal(GlobalConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return nil
}

// setDefaults 设置所有配置的默认值
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8889")

	// Log defaults
	viper.SetDefault("log.dir", "./logs")
	viper.SetDefault("log.level", "debug")

	// JWT defaults
	viper.SetDefault("jwt.key1", "ebe3vxIP7sblVvUHXb7ZaiMPuz4oXo0l")
	viper.SetDefault("jwt.key2", "ebe3vxIP7sblVvUHXb7ZaiMPuz4oXo0z")
	viper.SetDefault("jwt.issuer", "K5mBPBYNQeNWEBvCTE5msog3KSGTdhmx")
	viper.SetDefault("jwt.expiration", 3600)

	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")

	// Database defaults
	viper.SetDefault("database.driver", "mysql")
	viper.SetDefault("database.dsn", "root:root@tcp(localhost:3306)/cloudops?charset=utf8mb4&parseTime=True&loc=Local")

	// Legacy MySQL defaults
	viper.SetDefault("mysql.addr", "root:root@tcp(localhost:3306)/cloudops?charset=utf8mb4&parseTime=True&loc=Local")

	// Mock defaults
	viper.SetDefault("mock.enabled", true)

	// File Manager defaults
	viper.SetDefault("file_manager.enabled", true)
	viper.SetDefault("file_manager.allow_full_disk", false)
	viper.SetDefault("file_manager.max_edit_size_mb", 5)
	viper.SetDefault("file_manager.max_preview_size_mb", 10)

	// Agent defaults
	viper.SetDefault("agent.llm.provider", "openai")
	viper.SetDefault("agent.llm.base_url", "https://api.openai.com/v1")
	viper.SetDefault("agent.llm.model", "gpt-4o")
	viper.SetDefault("agent.llm.temperature", 0.7)
	viper.SetDefault("agent.llm.max_tokens", 4096)
	viper.SetDefault("agent.max_history", 20)
	viper.SetDefault("agent.risk.approval_timeout", "10m")
	viper.SetDefault("agent.risk.shell.default_risk", "low")
	viper.SetDefault("agent.risk.shell.timeout", "30s")
	viper.SetDefault("agent.risk.shell.max_output_bytes", 65536)
	viper.SetDefault("agent.hub.plugin_dir", "./data/plugins")
	viper.SetDefault("agent.hub.max_plugin_size", 52428800)
	viper.SetDefault("agent.hub.max_concurrent_plugins", 10)
}

// bindEnvVars 绑定环境变量
func bindEnvVars() {
	// 使用反射自动绑定所有配置项到环境变量
	bindStructEnvVars(reflect.TypeOf(Config{}), "")
}

// bindStructEnvVars 递归绑定结构体中的环境变量
func bindStructEnvVars(t reflect.Type, prefix string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 获取mapstructure标签作为配置键
		mapstructureTag := field.Tag.Get("mapstructure")
		if mapstructureTag == "" {
			// 如果没有 mapstructure 标签，跳过这个字段
			continue
		}

		// 构建完整的配置键
		var configKey string
		if prefix == "" {
			configKey = mapstructureTag
		} else {
			configKey = prefix + "." + mapstructureTag
		}

		// 获取实际类型（处理指针类型）
		actualType := field.Type
		if actualType.Kind() == reflect.Ptr {
			actualType = actualType.Elem()
		}

		// 如果是嵌套结构体，递归处理
		if actualType.Kind() == reflect.Struct {
			bindStructEnvVars(actualType, configKey)
		} else {
			// 对于非结构体字段，绑定环境变量
			// 获取env标签作为环境变量名
			envTag := field.Tag.Get(".env")
			if envTag != "" {
				viper.BindEnv(configKey, envTag)
			} else {
				// 如果没有env标签，使用配置键生成环境变量名
				envName := strings.ToUpper(strings.ReplaceAll(configKey, ".", "_"))
				viper.BindEnv(configKey, envName)
			}
		}
	}
}
