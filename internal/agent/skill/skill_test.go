package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func testLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

func TestNewSkillStore(t *testing.T) {
	logger := testLogger()

	// 测试有效目录
	store, err := NewSkillStore("", logger)
	if err != nil {
		t.Fatalf("创建 SkillStore 失败: %v", err)
	}
	if store.baseDir != defaultBaseDir {
		t.Errorf("期望 baseDir = %s, 实际 = %s", defaultBaseDir, store.baseDir)
	}

	// 验证 .usage.json 已创建
	if _, err := os.Stat(store.usagePath); os.IsNotExist(err) {
		t.Errorf(".usage.json 未创建")
	}

	// 测试自定义目录
	tmpDir := t.TempDir()
	customDir := filepath.Join(tmpDir, "custom-skills")
	store2, err := NewSkillStore(customDir, logger)
	if err != nil {
		t.Fatalf("创建自定义目录 SkillStore 失败: %v", err)
	}
	if store2.baseDir != customDir {
		t.Errorf("期望 baseDir = %s, 实际 = %s", customDir, store2.baseDir)
	}
}

func TestCreateSkillAndGetSkill(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	meta := SkillMeta{
		Name:        "test-skill",
		Description: "A test skill for testing",
		Category:    "testing",
		Tags:        []string{"test", "example"},
		Version:     "1.0.0",
	}
	content := "这是测试 skill 的内容。\n\n## 使用方法\n使用此 skill 进行测试。"

	skill, err := store.CreateSkill(ctx, meta, content, SkillSourceAgent)
	if err != nil {
		t.Fatalf("创建 skill 失败: %v", err)
	}
	if skill.Meta.Name != "test-skill" {
		t.Errorf("期望 name = test-skill, 实际 = %s", skill.Meta.Name)
	}
	if skill.State != SkillStateActive {
		t.Errorf("期望 state = active, 实际 = %s", skill.State)
	}

	// 验证文件已创建
	skillMDPath := filepath.Join(skill.Path, "SKILL.md")
	if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
		t.Error("SKILL.md 未创建")
	}

	// 验证 references 和 templates 目录已创建
	for _, sub := range []string{"references", "templates"} {
		subPath := filepath.Join(skill.Path, sub)
		if info, err := os.Stat(subPath); err != nil || !info.IsDir() {
			t.Errorf("%s 目录未创建", sub)
		}
	}

	// GetSkill roundtrip
	loaded, err := store.GetSkill(ctx, "test-skill")
	if err != nil {
		t.Fatalf("加载 skill 失败: %v", err)
	}
	if loaded.Meta.Name != "test-skill" {
		t.Errorf("期望 name = test-skill, 实际 = %s", loaded.Meta.Name)
	}
	if loaded.Meta.Description != "A test skill for testing" {
		t.Errorf("期望 description 匹配, 实际 = %s", loaded.Meta.Description)
	}
	if loaded.Content != content {
		t.Errorf("内容不匹配\n期望: %q\n实际: %q", content, loaded.Content)
	}
	if loaded.ViewCount != 1 {
		t.Errorf("期望 ViewCount = 1, 实际 = %d", loaded.ViewCount)
	}
}

func TestListSkills(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// 创建多个 skill
	skills := []struct {
		meta    SkillMeta
		content string
	}{
		{SkillMeta{Name: "skill-a", Description: "Skill A", Category: "cat1"}, "content A"},
		{SkillMeta{Name: "skill-b", Description: "Skill B", Category: "cat1"}, "content B"},
		{SkillMeta{Name: "skill-c", Description: "Skill C", Category: "cat2"}, "content C"},
	}

	for _, s := range skills {
		_, err := store.CreateSkill(ctx, s.meta, s.content, SkillSourceAgent)
		if err != nil {
			t.Fatalf("创建 skill %s 失败: %v", s.meta.Name, err)
		}
	}

	list, err := store.ListSkills(ctx)
	if err != nil {
		t.Fatalf("列出 skill 失败: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("期望 3 个 skill, 实际 = %d", len(list))
	}

	// 验证 Content 未加载（渐进式披露）
	for _, s := range list {
		if s.Content != "" {
			t.Errorf("ListSkills 不应加载内容, skill %s 有内容", s.Meta.Name)
		}
	}
}

func TestPatchSkill(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	meta := SkillMeta{
		Name:        "patch-test",
		Description: "Test patching",
		Category:    "test",
	}
	content := "line1\nline2\nline3"

	_, err = store.CreateSkill(ctx, meta, content, SkillSourceAgent)
	if err != nil {
		t.Fatal(err)
	}

	// 修补：替换 line2 为 replaced
	patched, err := store.PatchSkill(ctx, "patch-test", "line2", "replaced")
	if err != nil {
		t.Fatalf("修补 skill 失败: %v", err)
	}
	if patched.Content != "line1\nreplaced\nline3" {
		t.Errorf("修补内容不匹配\n期望: line1\\nreplaced\\nline3\\n\n实际: %s", patched.Content)
	}
	if patched.PatchCount != 1 {
		t.Errorf("期望 PatchCount = 1, 实际 = %d", patched.PatchCount)
	}
}

func TestDeleteSkill(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	meta := SkillMeta{
		Name:        "delete-me",
		Description: "To be deleted",
		Category:    "test",
	}

	skill, err := store.CreateSkill(ctx, meta, "content", SkillSourceAgent)
	if err != nil {
		t.Fatal(err)
	}

	// 删除
	if err := store.DeleteSkill(ctx, "delete-me"); err != nil {
		t.Fatalf("删除 skill 失败: %v", err)
	}

	// 验证目录已删除
	if _, err := os.Stat(skill.Path); !os.IsNotExist(err) {
		t.Error("skill 目录未被删除")
	}

	// 验证 GetSkill 返回错误
	_, err = store.GetSkill(ctx, "delete-me")
	if err == nil {
		t.Error("删除后 GetSkill 应该返回错误")
	}
}

func TestFormatSkillsForPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// 创建跨分类的 skill
	skills := []struct {
		meta    SkillMeta
		content string
	}{
		{SkillMeta{Name: "k8s-pod-debugging", Description: "诊断和修复 Kubernetes Pod 启动失败、CrashLoopBackOff、ImagePullBackOff 等常见问题", Category: "kubernetes"}, "content"},
		{SkillMeta{Name: "k8s-deployment", Description: "管理 Deployment 的创建更新回滚缩放及健康检查配置", Category: "kubernetes"}, "content"},
		{SkillMeta{Name: "network-diagnostics", Description: "网络连通性诊断 DNS 路由防火墙规则检查以及网络延迟排查工具", Category: "network"}, "content"},
	}

	for _, s := range skills {
		_, err := store.CreateSkill(ctx, s.meta, s.content, SkillSourceAgent)
		if err != nil {
			t.Fatalf("创建 skill %s 失败: %v", s.meta.Name, err)
		}
	}

	prompt := store.FormatSkillsForPrompt(ctx)
	if prompt == "" {
		t.Fatal("生成的提示为空")
	}

	// 验证必需要素
	if !strings.Contains(prompt, "## Skills (mandatory)") {
		t.Error("缺少 '## Skills (mandatory)' 标题")
	}
	if !strings.Contains(prompt, "<available_skills>") {
		t.Error("缺少 <available_skills> 标签")
	}
	if !strings.Contains(prompt, "</available_skills>") {
		t.Error("缺少 </available_skills> 标签")
	}
	if !strings.Contains(prompt, "kubernetes") {
		t.Error("缺少 kubernetes 分类")
	}
	if !strings.Contains(prompt, "k8s-pod-debugging") {
		t.Error("缺少 k8s-pod-debugging skill")
	}

	// 验证描述被截断
	t.Logf("生成的提示:\n%s", prompt)

	// 验证提示以 <available_skills> 结束
	if !strings.HasSuffix(strings.TrimSpace(prompt), "</available_skills>") {
		t.Error("提示应以 </available_skills> 结尾")
	}
}

func TestNameValidation(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// 有效的名称
	for _, name := range []string{"test", "my-skill", "k8s-deploy", "123", "a-b-c"} {
		meta := SkillMeta{Name: name, Description: "test", Category: "test"}
		_, err := store.CreateSkill(ctx, meta, "content", SkillSourceAgent)
		if err != nil {
			t.Errorf("有效名称 '%s' 被拒绝: %v", name, err)
		}
	}

	// 无效的名称
	for _, name := range []string{"Test", "My_Skill", "hello world", "skill.name", "skill!", ""} {
		meta := SkillMeta{Name: name, Description: "test", Category: "test"}
		_, err := store.CreateSkill(ctx, meta, "content", SkillSourceAgent)
		if err == nil {
			t.Errorf("无效名称 '%s' 应被拒绝", name)
		}
	}
}

func TestGetSkillFilePathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	meta := SkillMeta{
		Name:        "safe-skill",
		Description: "Test path safety",
		Category:    "test",
	}

	// 在 references 中创建一个文件
	_, err = store.CreateSkill(ctx, meta, "content", SkillSourceAgent)
	if err != nil {
		t.Fatal(err)
	}

	// 创建合法的引用文件
	skillPath := filepath.Join(tmpDir, "test", "safe-skill", "references", "readme.md")
	if err := os.WriteFile(skillPath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// 正常路径应正常工作
	data, err := store.GetSkillFile(ctx, "safe-skill", "references/readme.md")
	if err != nil {
		t.Errorf("正常路径失败: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("期望 'hello', 实际 = %q", string(data))
	}

	// 路径穿越应被禁止
	_, err = store.GetSkillFile(ctx, "safe-skill", "../other/file")
	if err == nil {
		t.Error("路径穿越应被拒绝")
	}
	t.Logf("路径穿越正确拒绝: %v", err)

	// 带..的相对路径应被拒绝
	_, err = store.GetSkillFile(ctx, "safe-skill", "references/../../etc/passwd")
	if err == nil {
		t.Error("带..的路径穿越应被拒绝")
	}
	t.Logf("带..的路径穿越正确拒绝: %v", err)
}

func TestPinUnpinSkill(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	meta := SkillMeta{
		Name:        "pinnable",
		Description: "Test pin/unpin",
		Category:    "test",
	}
	_, err = store.CreateSkill(ctx, meta, "content", SkillSourceAgent)
	if err != nil {
		t.Fatal(err)
	}

	// 置顶
	if err := store.PinSkill(ctx, "pinnable"); err != nil {
		t.Fatalf("置顶失败: %v", err)
	}

	skill, err := store.GetSkill(ctx, "pinnable")
	if err != nil {
		t.Fatal(err)
	}
	if skill.State != SkillStatePinned {
		t.Errorf("期望 state = pinned, 实际 = %s", skill.State)
	}

	// 取消置顶
	if err := store.UnpinSkill(ctx, "pinnable"); err != nil {
		t.Fatalf("取消置顶失败: %v", err)
	}

	skill, err = store.GetSkill(ctx, "pinnable")
	if err != nil {
		t.Fatal(err)
	}
	if skill.State == SkillStatePinned {
		t.Error("取消置顶后不应为 pinned")
	}
}

func TestUpdateState(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	meta := SkillMeta{
		Name:        "state-test",
		Description: "Test state update",
		Category:    "test",
	}
	_, err = store.CreateSkill(ctx, meta, "content", SkillSourceAgent)
	if err != nil {
		t.Fatal(err)
	}

	// 更新为 stale
	if err := store.UpdateState(ctx, "state-test", SkillStateStale); err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}

	skill, err := store.GetSkill(ctx, "state-test")
	if err != nil {
		t.Fatal(err)
	}
	if skill.State != SkillStateStale {
		t.Errorf("期望 state = stale, 实际 = %s", skill.State)
	}

	// 归档
	if err := store.UpdateState(ctx, "state-test", SkillStateArchived); err != nil {
		t.Fatalf("归档失败: %v", err)
	}

	skill, err = store.GetSkill(ctx, "state-test")
	if err != nil {
		t.Fatal(err)
	}
	if skill.State != SkillStateArchived {
		t.Errorf("期望 state = archived, 实际 = %s", skill.State)
	}
}

func TestRecordUseAndView(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	meta := SkillMeta{
		Name:        "counter-test",
		Description: "Test counters",
		Category:    "test",
	}
	_, err = store.CreateSkill(ctx, meta, "content", SkillSourceAgent)
	if err != nil {
		t.Fatal(err)
	}

	// 记录多次使用和查看
	for i := 0; i < 5; i++ {
		store.RecordUse(ctx, "counter-test")
	}
	for i := 0; i < 3; i++ {
		store.RecordView(ctx, "counter-test")
	}

	skill, err := store.GetSkill(ctx, "counter-test")
	if err != nil {
		t.Fatal(err)
	}
	// GetSkill 也会触发一次 RecordView，所以实际是 4
	if skill.UseCount != 5 {
		t.Errorf("期望 UseCount = 5, 实际 = %d", skill.UseCount)
	}
	if skill.ViewCount != 4 {
		t.Errorf("期望 ViewCount = 4, 实际 = %d", skill.ViewCount)
	}
}

func TestEmptySkillsPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSkillStore(tmpDir, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	prompt := store.FormatSkillsForPrompt(ctx)
	if prompt != "" {
		t.Errorf("空 skills 应返回空字符串, 实际 = %q", prompt)
	}
}
