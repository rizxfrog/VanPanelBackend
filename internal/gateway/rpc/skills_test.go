package rpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	agentskill "github.com/rizxfrog/VanPanelBackend/internal/agent/skill"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func setupSkillService(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	store, err := agentskill.NewSkillStore(tmpDir, logger)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	clawHub := agentskill.NewClawHubClient(agentskill.ClawHubConfig{})
	svc := service.NewSkillService(store, clawHub, service.SkillsConfig{BaseDir: tmpDir}, logger)
	SetSkillService(svc)

	categoryDir := filepath.Join(tmpDir, "demo")
	if err := os.MkdirAll(categoryDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `---
name: hello-world
description: A sample skill
version: "1.0.0"
category: demo
emoji: "👋"
always: false
bundled: false
source: installed
requirements:
  bins:
    - sh
  env:
    - HOME
  config: []
  os:
    - linux
    - darwin
install:
  - id: hello-world-sh
    kind: download
    label: Download sample
skill_card:
  path: SKILL.md
---

# hello-world

Sample skill.
`
	skillPath := filepath.Join(categoryDir, "hello-world", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return tmpDir
}

func TestHandleSkillsStatus(t *testing.T) {
	setupSkillService(t)
	conn := &gateway.GatewayConnection{}
	params, _ := json.Marshal(map[string]interface{}{})

	resp, err := handleSkillsStatus(context.Background(), conn, params)
	if err != nil {
		t.Fatalf("handleSkillsStatus failed: %v", err)
	}
	report, ok := resp.(*service.SkillStatusReport)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}
	if len(report.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(report.Skills))
	}
	if report.Skills[0].SkillKey != "hello-world" {
		t.Errorf("unexpected skillKey: %s", report.Skills[0].SkillKey)
	}
}

func TestHandleSkillsSkillCard(t *testing.T) {
	setupSkillService(t)
	conn := &gateway.GatewayConnection{}
	params, _ := json.Marshal(map[string]interface{}{"skillKey": "hello-world"})

	resp, err := handleSkillsSkillCard(context.Background(), conn, params)
	if err != nil {
		t.Fatalf("handleSkillsSkillCard failed: %v", err)
	}
	card, ok := resp.(*service.SkillCardResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}
	if card.Schema != "agentops.skills.skill-card.v1" {
		t.Errorf("unexpected schema: %s", card.Schema)
	}
	if card.SkillKey != "hello-world" {
		t.Errorf("unexpected skillKey: %s", card.SkillKey)
	}
}

func TestHandleSkillsUpdate(t *testing.T) {
	setupSkillService(t)
	conn := &gateway.GatewayConnection{}
	enabled := false
	params, _ := json.Marshal(map[string]interface{}{"skillKey": "hello-world", "enabled": enabled})

	resp, err := handleSkillsUpdate(context.Background(), conn, params)
	if err != nil {
		t.Fatalf("handleSkillsUpdate failed: %v", err)
	}
	respMap, ok := resp.(map[string]bool)
	if !ok || !respMap["ok"] {
		t.Errorf("expected ok response")
	}
}

func TestHandleSkillsSecurityVerdicts(t *testing.T) {
	setupSkillService(t)
	conn := &gateway.GatewayConnection{}
	params, _ := json.Marshal(map[string]interface{}{})

	resp, err := handleSkillsSecurityVerdicts(context.Background(), conn, params)
	if err != nil {
		t.Fatalf("handleSkillsSecurityVerdicts failed: %v", err)
	}
	verdicts, ok := resp.(*agentskill.ClawHubSecurityVerdictsResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}
	if verdicts.Schema != "agentops.skills.security-verdicts.v1" {
		t.Errorf("unexpected schema: %s", verdicts.Schema)
	}
	if len(verdicts.Items) != 0 {
		t.Errorf("expected 0 verdicts, got %d", len(verdicts.Items))
	}
}
