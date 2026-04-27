package dao

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestUserDAOGetByUsernameWithNilDBReturnsError(t *testing.T) {
	dao := NewUserDAO(nil, zap.NewNop())

	_, err := dao.GetByUsername(context.Background(), "admin")
	if err == nil {
		t.Fatal("expected error when db is nil")
	}
}
