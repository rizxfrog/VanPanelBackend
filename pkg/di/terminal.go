package di

import (
	"time"

	terminalpty "github.com/GoSimplicity/AI-CloudOps/internal/terminal/pty"
	terminalservice "github.com/GoSimplicity/AI-CloudOps/internal/terminal/service"
	terminalssh "github.com/GoSimplicity/AI-CloudOps/internal/terminal/ssh"
	treeservice "github.com/GoSimplicity/AI-CloudOps/internal/tree/service"
	"go.uber.org/zap"
)

func ProvideTerminalConfig() terminalservice.Config {
	return terminalservice.Config{IdleTimeout: 30 * time.Minute}
}

func ProvideTerminalLocalAdapter() terminalservice.LocalAdapter {
	return terminalpty.NewLocalAdapter()
}

func ProvideTerminalSSHAdapter(treeLocal treeservice.TreeLocalService, logger *zap.Logger) terminalservice.SSHAdapter {
	return terminalssh.NewAdapter(treeLocal, logger)
}

func ProvideTerminalService(logger *zap.Logger, local terminalservice.LocalAdapter, ssh terminalservice.SSHAdapter, config terminalservice.Config) terminalservice.TerminalService {
	return terminalservice.NewTerminalService(logger, local, ssh, config)
}
