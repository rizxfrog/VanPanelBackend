package di

import (
	"fmt"
	"os"
	"strings"

	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"gorm.io/gorm"
)

func InitTables(db *gorm.DB) error {
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

		// 文件分享系统
		&model.FileShare{},
		&model.FileShareItem{},

		// agent
		&model.AgentConfig{},
	)
}

// InitStoredProcedures 初始化存储过程
func InitStoredProcedures(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("数据库连接为空，跳过存储过程初始化")
	}

	if db.Dialector.Name() != "postgres" {
		return nil
	}

	sqlFile := "scripts/sql/file_share_functions.sql"
	content, err := os.ReadFile(sqlFile)
	if err != nil {
		fmt.Printf("跳过存储过程初始化: %v\n", err)
		return nil
	}

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
