package di

import (
	"time"

	terminalpty "github.com/rizxfrog/VanPanelBackend/internal/terminal/pty"
	terminalservice "github.com/rizxfrog/VanPanelBackend/internal/terminal/service"
	terminalssh "github.com/rizxfrog/VanPanelBackend/internal/terminal/ssh"
	treeservice "github.com/rizxfrog/VanPanelBackend/internal/tree/service"
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
