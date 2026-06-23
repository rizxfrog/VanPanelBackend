package dao

import (
	"context"

	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"gorm.io/gorm"
)

type AgentConfigDAO struct {
	db *gorm.DB
}

func NewAgentConfigDAO(db *gorm.DB) *AgentConfigDAO {
	return &AgentConfigDAO{db: db}
}

// GetByKey returns a single config by key
func (d *AgentConfigDAO) GetByKey(ctx context.Context, key string) (*model.AgentConfig, error) {
	var cfg model.AgentConfig
	err := d.db.WithContext(ctx).Where("config_key = ?", key).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// List returns all configs ordered by id
func (d *AgentConfigDAO) List(ctx context.Context) ([]model.AgentConfig, error) {
	var cfgs []model.AgentConfig
	err := d.db.WithContext(ctx).Order("id ASC").Find(&cfgs).Error
	return cfgs, err
}

// Upsert creates or updates a config entry (PostgreSQL ON CONFLICT)
func (d *AgentConfigDAO) Upsert(ctx context.Context, key string, value string, desc string) error {
	cfg := model.AgentConfig{
		ConfigKey:   key,
		ConfigValue: value,
		Description: desc,
	}
	return d.db.WithContext(ctx).
		Where("config_key = ?", key).
		Assign(cfg).
		FirstOrCreate(&cfg).Error
}

// Delete removes a config entry by key.
func (d *AgentConfigDAO) Delete(ctx context.Context, key string) error {
	return d.db.WithContext(ctx).Where("config_key = ?", key).Delete(&model.AgentConfig{}).Error
}
