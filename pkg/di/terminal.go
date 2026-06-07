package di

import (
	"time"

	terminalpty "github.com/rizxfrog/VanPanelBackend/internal/terminal/pty"
	terminalservice "github.com/rizxfrog/VanPanelBackend/internal/terminal/service"
	"go.uber.org/zap"
)

func ProvideTerminalConfig() terminalservice.Config {
	return terminalservice.Config{IdleTimeout: 30 * time.Minute}
}

func ProvideTerminalLocalAdapter() terminalservice.LocalAdapter {
	return terminalpty.NewLocalAdapter()
}

func ProvideTerminalService(logger *zap.Logger, local terminalservice.LocalAdapter, config terminalservice.Config) terminalservice.TerminalService {
	return terminalservice.NewTerminalService(logger, local, nil, config)
}
