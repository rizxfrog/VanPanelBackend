package startup

import (
	"context"

	"go.uber.org/zap"
)

type ApplicationBootstrap interface {
	Initialize(ctx context.Context) error
}

type applicationBootstrap struct {
	logger *zap.Logger
}

func NewApplicationBootstrap(logger *zap.Logger) ApplicationBootstrap {
	return &applicationBootstrap{
		logger: logger,
	}
}

func (ab *applicationBootstrap) Initialize(ctx context.Context) error {
	ab.logger.Info("application bootstrap completed")
	return nil
}
