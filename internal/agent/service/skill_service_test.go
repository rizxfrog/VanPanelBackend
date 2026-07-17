package service

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
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

// makeSkillZip creates an in-memory zip with the given SKILL.md content and optional extra files.
func makeSkillZip(t *testing.T, frontmatter, body string, extraFiles map[string]string) ([]byte, int64) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	md := "---\n" + frontmatter + "---\n\n" + body + "\n"
	f, err := zw.Create("SKILL.md")
	if err != nil {
		t.Fatalf("create SKILL.md entry: %v", err)
	}
	if _, err := f.Write([]byte(md)); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	for name, content := range extraFiles {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s entry: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes(), int64(buf.Len())
}

func makeSkillZipNoSkillMD(t *testing.T, extraFiles map[string]string) ([]byte, int64) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range extraFiles {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s entry: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes(), int64(buf.Len())
}

func TestInstallFromArchive_Success(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	zipBytes, size := makeSkillZip(t,
		"name: hello-world\nversion: 1.0.0\ndescription: A sample skill",
		"# hello-world\n\nThis is a sample skill.",
		nil,
	)

	msg, err := svc.InstallFromArchive(context.Background(), "", "1.0.0", bytes.NewReader(zipBytes), size)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "hello-world") {
		t.Errorf("expected message to contain name, got %q", msg)
	}
	if !strings.Contains(msg, "1.0.0") {
		t.Errorf("expected message to contain version, got %q", msg)
	}

	target := filepath.Join(tmpDir, "uploaded", "hello-world", "SKILL.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("target SKILL.md missing: %v", err)
	}
	if !strings.Contains(string(data), "source: uploaded") {
		t.Errorf("expected source=uploaded in SKILL.md, got:\n%s", string(data))
	}
}

func TestInstallFromArchive_ExplicitName(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	zipBytes, size := makeSkillZip(t,
		"name: ignored-name\ndescription: test",
		"body",
		nil,
	)

	msg, err := svc.InstallFromArchive(context.Background(), "my-skill", "", bytes.NewReader(zipBytes), size)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "my-skill") {
		t.Errorf("expected message to contain explicit name, got %q", msg)
	}

	target := filepath.Join(tmpDir, "uploaded", "my-skill", "SKILL.md")
	if _, err := os.Stat(target); err != nil {
		t.Errorf("target SKILL.md missing: %v", err)
	}
}

func TestInstallFromArchive_Idempotent(t *testing.T) {
	svc, _, _ := testSkillService(t)
	zipBytes, size := makeSkillZip(t,
		"name: dup-skill\ndescription: version1",
		"body1",
		nil,
	)
	if _, err := svc.InstallFromArchive(context.Background(), "", "1.0.0", bytes.NewReader(zipBytes), size); err != nil {
		t.Fatalf("first install: %v", err)
	}

	zipBytes2, size2 := makeSkillZip(t,
		"name: dup-skill\ndescription: version2",
		"body2",
		nil,
	)
	if _, err := svc.InstallFromArchive(context.Background(), "", "2.0.0", bytes.NewReader(zipBytes2), size2); err != nil {
		t.Fatalf("second install: %v", err)
	}
}

func TestInstallFromArchive_MissingSkillMD(t *testing.T) {
	svc, _, _ := testSkillService(t)
	zipBytes, size := makeSkillZipNoSkillMD(t, map[string]string{"other/file.txt": "content"})

	_, err := svc.InstallFromArchive(context.Background(), "test", "", bytes.NewReader(zipBytes), size)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "SKILL.md") {
		t.Errorf("expected error about missing SKILL.md, got: %v", err)
	}
}

func TestInstallFromArchive_BadFrontmatter(t *testing.T) {
	svc, _, _ := testSkillService(t)
	// Invalid YAML frontmatter
	zipBytes, size := makeSkillZip(t, "{{{{invalid yaml", "body", nil)

	_, err := svc.InstallFromArchive(context.Background(), "bad", "", bytes.NewReader(zipBytes), size)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "frontmatter") {
		t.Errorf("expected error about frontmatter, got: %v", err)
	}
}

func TestInstallFromArchive_NameFormatInvalid(t *testing.T) {
	svc, _, _ := testSkillService(t)
	zipBytes, size := makeSkillZip(t, "name: Invalid Name!\ndescription: test", "body", nil)

	// Name from frontmatter should be rejected because it has spaces/caps
	_, err := svc.InstallFromArchive(context.Background(), "", "", bytes.NewReader(zipBytes), size)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "名称格式") {
		t.Errorf("expected error about invalid name format, got: %v", err)
	}
}

func TestInstallFromArchive_NameExplicitInvalid(t *testing.T) {
	svc, _, _ := testSkillService(t)
	zipBytes, size := makeSkillZip(t, "name: valid\ndescription: test", "body", nil)

	_, err := svc.InstallFromArchive(context.Background(), "Invalid-Name", "", bytes.NewReader(zipBytes), size)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "名称格式") {
		t.Errorf("expected error about invalid name format, got: %v", err)
	}
}

func TestInstallFromArchive_ClearsClawHub(t *testing.T) {
	svc, _, tmpDir := testSkillService(t)
	zipBytes, size := makeSkillZip(t,
		"name: no-clawhub\ndescription: test\nclawhub:\n  registry: test\n  slug: test",
		"body",
		nil,
	)

	if _, err := svc.InstallFromArchive(context.Background(), "", "", bytes.NewReader(zipBytes), size); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	target := filepath.Join(tmpDir, "uploaded", "no-clawhub", "SKILL.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}
	if strings.Contains(string(data), "clawhub:") {
		t.Errorf("expected ClawHub metadata to be cleared, got:\n%s", string(data))
	}
}
