package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	agentskill "github.com/rizxfrog/VanPanelBackend/internal/agent/skill"
)

func testSkillService(t *testing.T) (*SkillService, *agentskill.SkillStore, string) {
	t.Helper()
	tmpDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	store, err := agentskill.NewSkillStore(tmpDir, logger)
	if err != nil {
		t.Fatalf("create skill store: %v", err)
	}
	clawHub := agentskill.NewClawHubClient(agentskill.ClawHubConfig{})
	svc := NewSkillService(store, clawHub, SkillsConfig{BaseDir: tmpDir}, logger)
	return svc, store, tmpDir
}

func writeSampleSkill(t *testing.T, dir, name string) {
	t.Helper()
	categoryDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(categoryDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `---
name: hello-world
description: A sample skill
version: "1.0.0"
category: demo
emoji: "👋"
homepage: "https://example.com"
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
    bins:
      - sh
skill_card:
  path: SKILL.md
---

# hello-world

This is a sample skill.
`
	skillPath := filepath.Join(categoryDir, name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func TestSkillServiceStatus(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	writeSampleSkill(t, tmpDir, "hello-world")

	ctx := context.Background()
	report, err := svc.Status(ctx, "")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if len(report.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(report.Skills))
	}

	skill := report.Skills[0]
	if skill.SkillKey != "hello-world" {
		t.Errorf("expected skillKey hello-world, got %s", skill.SkillKey)
	}
	if skill.Source != "installed" {
		t.Errorf("expected source installed, got %s", skill.Source)
	}
	if skill.Emoji != "👋" {
		t.Errorf("expected emoji, got %s", skill.Emoji)
	}
	if !skill.Eligible {
		t.Errorf("expected skill to be eligible")
	}
	if len(skill.Requirements.Bins) != 1 || skill.Requirements.Bins[0] != "sh" {
		t.Errorf("unexpected bins: %v", skill.Requirements.Bins)
	}
	if len(skill.Install) != 1 {
		t.Errorf("expected 1 install option, got %d", len(skill.Install))
	}
	if skill.SkillCard == nil || !skill.SkillCard.Present {
		t.Errorf("expected skill card present")
	}
}

func TestSkillServiceGetSkillCard(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	writeSampleSkill(t, tmpDir, "hello-world")

	ctx := context.Background()
	card, err := svc.GetSkillCard(ctx, "hello-world")
	if err != nil {
		t.Fatalf("get skill card failed: %v", err)
	}
	if card.Schema != "agentops.skills.skill-card.v1" {
		t.Errorf("unexpected schema: %s", card.Schema)
	}
	if card.SkillKey != "hello-world" {
		t.Errorf("unexpected skillKey: %s", card.SkillKey)
	}
	if card.SizeBytes == 0 {
		t.Errorf("expected non-zero size")
	}
	if card.Content == "" {
		t.Errorf("expected content")
	}
}

func TestSkillServiceUpdateSkill(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	writeSampleSkill(t, tmpDir, "hello-world")

	ctx := context.Background()
	enabled := false
	if err := svc.UpdateSkill(ctx, UpdateSkillRequest{SkillKey: "hello-world", Enabled: &enabled}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	report, err := svc.Status(ctx, "")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !report.Skills[0].Disabled {
		t.Errorf("expected skill to be disabled")
	}
}

func TestSkillServiceUpdateSkillAPIKey(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	writeSampleSkill(t, tmpDir, "hello-world")

	ctx := context.Background()
	if err := svc.UpdateSkill(ctx, UpdateSkillRequest{SkillKey: "hello-world", APIKey: "secret123"}); err != nil {
		t.Fatalf("update api key failed: %v", err)
	}

	secretsPath := filepath.Join(tmpDir, ".secrets.json")
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		t.Errorf("secrets file not created")
	}
}

func TestSkillServiceSecurityVerdictsEmpty(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	writeSampleSkill(t, tmpDir, "hello-world")

	ctx := context.Background()
	verdicts, err := svc.SecurityVerdicts(ctx, SecurityVerdictsRequest{})
	if err != nil {
		t.Fatalf("security verdicts failed: %v", err)
	}
	if verdicts.Schema != "agentops.skills.security-verdicts.v1" {
		t.Errorf("unexpected schema: %s", verdicts.Schema)
	}
	// No linked clawhub skills, so items should be empty.
	if len(verdicts.Items) != 0 {
		t.Errorf("expected 0 verdicts, got %d", len(verdicts.Items))
	}
}
