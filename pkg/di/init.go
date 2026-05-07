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
	"path/filepath"
	"strings"

	"github.com/GoSimplicity/AI-CloudOps/internal/model"
	"gorm.io/gorm"
)

func InitTables(db *gorm.DB) error {
	// 检查数据库连接是否为nil
	if db == nil {
		return fmt.Errorf("数据库连接为空，跳过表初始化")
	}

	return db.AutoMigrate(
		// auth
		&model.User{},
		&model.Api{},
		&model.AuditLog{},
		&model.Role{},
		&model.RoleApi{},
		&model.UserRole{},

		// tree
		&model.TreeNode{},
		&model.TreeLocalResource{},
		&model.TreeCloudResource{},
		&model.CloudAccount{},
		&model.K8sCluster{},
		&model.CloudResourceSyncHistory{},
		&model.CloudResourceChangeLog{},

		// prometheus
		&model.MonitorScrapePool{},
		&model.MonitorScrapeJob{},
		&model.MonitorAlertManagerPool{},
		&model.MonitorAlertRule{},
		&model.MonitorRecordRule{},
		&model.MonitorOnDutyHistory{},
		&model.MonitorOnDutyGroup{},
		&model.MonitorSendGroup{},
		&model.MonitorOnDutyChange{},
		&model.MonitorAlertEvent{},
		&model.MonitorConfig{},

		// 工单系统
		&model.WorkorderFormDesign{},
		&model.WorkorderInstance{},
		&model.WorkorderInstanceFlow{},
		&model.WorkorderInstanceComment{},
		&model.WorkorderProcess{},
		&model.WorkorderTemplate{},
		&model.WorkorderCategory{},
		&model.WorkorderNotification{},
		&model.WorkorderNotificationLog{},
		&model.WorkorderInstanceTimeline{},

		// 定时任务系统
		&model.CronJob{},

		// 文件分享系统
		&model.FileShare{},
		&model.FileShareItem{},
	)
}

// InitStoredProcedures 初始化存储过程
func InitStoredProcedures(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("数据库连接为空，跳过存储过程初始化")
	}

	// 只在 PostgreSQL 下执行存储过程
	if db.Dialector.Name() != "postgres" {
		return nil
	}

	// 查找 SQL 文件
	sqlFile := filepath.Join("scripts", "sql", "file_share_functions.sql")
	content, err := os.ReadFile(sqlFile)
	if err != nil {
		// 文件不存在时跳过（可能是部署方式不同）
		fmt.Printf("跳过存储过程初始化: %v\n", err)
		return nil
	}

	// 按分号+空行分割为独立语句逐条执行，避免批量执行时单条失败导致全部跳过
	statements := splitSQLStatements(string(content))
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("执行存储过程第 %d 条失败: %w\nSQL: %s", i+1, err, truncateStr(stmt, 200))
		}
	}

	fmt.Println("存储过程初始化完成")
	return nil
}

// splitSQLStatements 按 $$ ... $$ 块分割 SQL 语句，保留每个完整的函数定义
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inDollarQuote := false

	for i := 0; i < len(sql); i++ {
		if i+1 < len(sql) && sql[i] == '$' && sql[i+1] == '$' {
			inDollarQuote = !inDollarQuote
			current.WriteString("$$")
			i++
			continue
		}
		if sql[i] == ';' && !inDollarQuote {
			current.WriteByte(sql[i])
			statements = append(statements, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(sql[i])
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		statements = append(statements, s)
	}
	return statements
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
