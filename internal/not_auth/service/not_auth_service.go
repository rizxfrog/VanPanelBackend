package service

import (
	"go.uber.org/zap"
)

type NotAuthService interface {
}

type notAuthService struct {
	l *zap.Logger
}

func NewNotAuthService(l *zap.Logger) NotAuthService {
	return &notAuthService{
		l: l,
	}
}
