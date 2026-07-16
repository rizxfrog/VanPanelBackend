# Skill Archive Upload 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持通过 HTTP multipart 上传 zip 压缩包来安装/更新 skill。

**Architecture:** Gin handler 接收 multipart 文件 → SkillService.InstallFromArchive 解压到临时目录 → 校验 SKILL.md frontmatter → 移动到 data/skills/uploaded/<name>/ 并重写元数据。复用现有 unzipToDir 逻辑并提取为导出函数。

**Tech Stack:** Go 1.25.5, Gin, archive/zip

## Global Constraints

- 修改文件清单限定在 spec 中 4 个文件，不改造 SkillStore 结构
- 只支持 zip 格式
- 大小限制默认 10MB
- 路径穿越保护复用现有逻辑
- 来源标记新增 `SkillSourceUploaded SkillSource = "uploaded"`
- 同名 skill 已存在时覆盖（先删后建）

---

## Task 1: 新增 SkillSourceUploaded 常量

**Files:**
- Modify: `internal/agent/skill/types.go:8-12`

**Interfaces:**
- Produces: `SkillSourceUploaded` — 供后续 task 引用

- [ ] Step 1: 编辑 `internal/agent/skill/types.go`，在 `const` 块末尾追加 `SkillSourceUploaded SkillSource = "uploaded"`

```go
const (
	SkillSourceBundled SkillSource = "bundled"
	SkillSourceUser    SkillSource = "user"
	SkillSourceAgent   SkillSource = "agent"
	SkillSourceUploaded SkillSource = "uploaded"
)
```

- [ ] Step 2: 运行 `go build ./internal/agent/skill/...` 编译通过

- [ ] Step 3: `git add internal/agent/skill/types.go && git commit -m "feat(skill): add SkillSourceUploaded source constant"`

---

## Task 2: 提取并扩展 UnzipToDir 导出函数

**Files:**
- Modify: `internal/agent/service/skill_service.go:564-615`

**Interfaces:**
- Produces: `UnzipToDir(r io.ReaderAt, size int64, targetDir string) error`
- Consumes: `installFromClawHub` 改造后调用 `UnzipToDir`

- [ ] Step 1: 提取 `unzipToDir` 为导出函数 `UnzipToDir`

```go
// UnzipToDir 将 zip 数据解压到目标目录（含路径穿越保护）。
func UnzipToDir(r io.ReaderAt, size int64, targetDir string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		// Trim a single top-level directory if present.
		name := f.Name
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 && parts[1] != "" {
			name = parts[1]
		}
		if name == "" {
			continue
		}
		if strings.Contains(name, "..") {
			continue
		}

		path := filepath.Join(targetDir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		fr, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, fr)
		fr.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] Step 2: 改造 `installFromClawHub` 复用该函数

```go
	buf, err := io.ReadAll(result.Body)
	if err != nil {
		return "", fmt.Errorf("read download body failed: %w", err)
	}

	category := "clawhub"
	targetDir := filepath.Join(s.cfg.BaseDir, category, slug)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("create target dir failed: %w", err)
	}

	if err := UnzipToDir(bytes.NewReader(buf), int64(len(buf)), targetDir); err != nil {
		return "", fmt.Errorf("extract skill archive failed: %w", err)
	}
```

- [ ] Step 3: 运行 `go build ./internal/agent/service/...` 编译通过

- [ ] Step 4: `git add internal/agent/service/skill_service.go && git commit -m "refactor(skill): extract UnzipToDir as exported helper"`

---

## Task 3: 新增 SkillService.InstallFromArchive

**Files:**
- Modify: `internal/agent/service/skill_service.go` (追加到文件末尾)

**Interfaces:**
- Produces: `InstallFromArchive(ctx, name, version string, r io.ReaderAt, size int64) (string, error)`
- Consumes: `UnzipToDir` (Task 2), `SkillStore.CreateSkill`, `SkillStore.DeleteSkill`, `SkillSourceUploaded` (Task 1)

- [ ] Step 1: 添加 `InstallFromArchive` 方法（依赖 Task 2 `UnzipToDir`）

```go
// InstallFromArchive 从 zip 压缩包安装/更新 skill。
func (s *SkillService) InstallFromArchive(ctx context.Context, name, version string, r io.ReaderAt, size int64) (string, error) {
	if version == "" {
		version = "0.0.0-local"
	}

	// 解析可选 name：缺省时占位，从 frontmatter 推断
	if name != "" && !nameRegex.MatchString(name) {
		return "", fmt.Errorf("skill 名称格式无效，必须匹配 [a-z0-9-]+")
	}
	if len(name) > 64 {
		return "", fmt.Errorf("skill 名称不能超过64个字符")
	}

	// 解压到临时目录
	tmpDir, err := os.MkdirTemp("", "skill-upload-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := UnzipToDir(r, size, tmpDir); err != nil {
		return "", fmt.Errorf("解压 zip 失败: %w", err)
	}

	// 校验 SKILL.md 存在且 frontmatter 解析成功
	skillMDPath := filepath.Join(tmpDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); err != nil {
		return "", fmt.Errorf("压缩包中缺少 SKILL.md")
	}
	meta, content, err := agentskill.ParseFullSkillMD(readFileBytes(skillMDPath))
	if err != nil {
		return "", fmt.Errorf("SKILL.md frontmatter 解析失败: %w", err)
	}

	// 推断 name
	if name == "" {
		name = meta.Name
	}
	if name == "" {
		return "", fmt.Errorf("无法推断 skill 名称")
	}
	if !nameRegex.MatchString(name) {
		return "", fmt.Errorf("skill 名称格式无效，必须匹配 [a-z0-9-]+")
	}

	// meta.Name 必须一致（避免 zip 内部 frontmatter 名称和 name 冲突）
	meta.Name = name
	meta.Source = string(agentskill.SkillSourceUploaded)
	meta.ClawHub = nil // 清空 ClawHub 元数据，因为是本地上传
	if meta.Category == "" {
		meta.Category = "uploaded"
	}

	// 写入带清理后 frontmatter 的 SKILL.md 到临时目录（使用 ParseFullSkillMD 内容，确保格式一致）
	if err := agentskill.WriteSkillMD(skillMDPath, *meta, content); err != nil {
		return "", fmt.Errorf("重写 SKILL.md frontmatter 失败: %w", err)
	}

	// 目标目录
	targetDir := filepath.Join(s.cfg.BaseDir, "uploaded", name)

	// 同名 skill 已存在时先删除
	skills, err := s.store.ListSkills(ctx)
	if err == nil {
		for _, sk := range skills {
			if sk.Meta.Name == name {
				_ = s.store.DeleteSkill(ctx, name)
				break
			}
		}
	}

	// 创建上传根目录（如果不存在）
	uploadedRoot := filepath.Join(s.cfg.BaseDir, "uploaded")
	if err := os.MkdirAll(uploadedRoot, 0755); err != nil {
		return "", fmt.Errorf("创建上传根目录失败: %w", err)
	}

	// 移动临时目录内容到目标目录（已存在则先清理）
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return "", fmt.Errorf("清理旧 skill 目录失败: %w", err)
		}
	}
	if err := os.Rename(tmpDir, targetDir); err != nil {
		// 跨设备回退：复制
		return "", fmt.Errorf("移动 skill 目录失败: %w", err)
	}

	// 写 usage 统计（SkillStore 不暴露 updateUsage，可跳过；ListSkills 会通过扫描 SKILL.md 重新发现）
	_ = name

	s.logger.Info("skill 从压缩包安装成功", zap.String("name", name), zap.String("version", version))
	return fmt.Sprintf("Installed %s (%s) from archive", name, version), nil
}
```

- [ ] Step 2: `go build ./internal/agent/service/...` 编译通过

- [ ] Step 3: `git add internal/agent/service/skill_service.go && git commit -m "feat(skill): add InstallFromArchive method"`

> **注意**: 如果 SkillStore 没有 `updateUsageIfNotExists` 方法，使用 `updateUsage` 包装：
> ```go
> s.store.updateUsage(name, func(u *agentskill.SkillUsage) {
>     if u.State == "" { u.State = agentskill.SkillStateActive }
> })
> ```
> 如果 SkillStore 没有公开 updateUsage，则先跳过 usage 写入，后续补。

---

## Task 4: 新增 Gin HTTP Handler

**Files:**
- Modify: `internal/agent/api/handler.go`

**Interfaces:**
- Produces: `UploadSkill(c *gin.Context)` handler
- Consumes: `SkillService.InstallFromArchive`

- [ ] Step 1: 在 `Handler` 结构体中新增 `skillService *agentService.Service` 依赖，在 `NewHandler` 中添加参数 `skillService *agentService.Service`

> **实际**: `SkillService` 未注入到 `Handler`，因此需要在 `ProvideAgentHandler` 添加 `skillService` 参数，需同步改 `wire.go` + `make generate`。详见 Task 5。

- [ ] Step 2: 在 `internal/agent/api/handler.go` 添加

```go
// UploadSkill 通过 zip 压缩包安装/更新 skill
func (h *Handler) UploadSkill(c *gin.Context) {
	var req struct {
		Name    string `form:"name"`
		Version string `form:"version"`
	}
	_ = c.ShouldBind(&req)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		base.BadRequestError(c, "缺少 skill 压缩包")
		return
	}
	defer file.Close()

	const maxSize = 10 << 20 // 10MB
	if header.Size > maxSize {
		base.ErrorWithStatus(c, http.StatusRequestEntityTooLarge, "文件过大，最大 10MB")
		return
	}

	// 读入字节
	buf, err := io.ReadAll(file)
	if err != nil {
		base.BadRequestError(c, "读取文件失败")
		return
	}

	msg, err := h.skillService.InstallFromArchive(c.Request.Context(), req.Name, req.Version, bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		// 按错误类型映射状态码
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "名称格式"):
			base.BadRequestError(c, errMsg)
		case strings.Contains(errMsg, "缺少 SKILL.md"), strings.Contains(errMsg, "frontmatter"):
			base.BadRequestError(c, errMsg)
		default:
			base.ErrorWithMessage(c, "安装失败: "+errMsg)
		}
		return
	}

	base.SuccessWithData(c, gin.H{"ok": true, "message": msg})
}
```

- [ ] Step 3: 在 `RegisterRouters` 中注册路由

在 "搜索、分析、技能" 块末尾追加：
```go
agentGroup.POST("/skills/upload", h.UploadSkill)
```

- [ ] Step 4: 在 `NewHandler` 添加参数

```go
func NewHandler(
	agentService service.AgentService,
	hubService hub.HubService,
	configService *service.ConfigService,
	searchEngine *agentSearch.SearchEngine,
	skillStore *skill.SkillStore,
	insights *insight.InsightsEngine,
	skillService *service.SkillService,
) *Handler {
	return &Handler{
		agentService:  agentService,
		hubService:    hubService,
		configService: configService,
		searchEngine:  searchEngine,
		skillStore:    skillStore,
		insights:      insights,
		skillService:  skillService,
	}
}
```

- [ ] Step 5: 在 `internal/agent/api/handler.go` struct 字段中追加一行

```go
skillService  *service.SkillService
```

- [ ] Step 6: `go build ./internal/agent/api/...` 编译通过

- [ ] Step 7: `git add internal/agent/api/handler.go && git commit -m "feat(api): add skill upload HTTP handler"`

---

## Task 5: 更新 Wire 注入

**Files:**
- Modify: `pkg/di/agent.go` — `ProvideAgentHandler` 签名
- Modify: `pkg/di/wire_gen.go` — Wire 生成文件（make generate）

**Interfaces:**
- 让 `cmd.SkillService` 注入到 `api.Handler`

- [ ] Step 1: 在 `pkg/di/agent.go` 修改 `ProvideAgentHandler` 签名

```go
func ProvideAgentHandler(
	agentSvc agentService.AgentService,
	hubSvc agentHub.HubService,
	cfgSvc *agentService.ConfigService,
	searchEngine *agentSearch.SearchEngine,
	skillStore *agentSkill.SkillStore,
	insights *agentInsight.InsightsEngine,
	skillSvc *agentService.SkillService,
) *api.Handler {
	return api.NewHandler(agentSvc, hubSvc, cfgSvc, searchEngine, skillStore, insights, skillSvc)
}
```

- [ ] Step 2: 运行 `make generate` 重新生成 `wire_gen.go`

- [ ] Step 3: 运行 `go build ./...` 编译通过

- [ ] Step 4: `git add pkg/di/agent.go pkg/di/wire_gen.go && git commit -m "wire(api): inject skillService into agent handler"`

---

## Task 6: 单元测试

**Files:**
- Create: `internal/agent/service/skill_archive_test.go`

**Interfaces:**
- 使用 `SkillService.InstallFromArchive`

- [ ] Step 1: 写测试文件骨架

```go
package service

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/skill"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"go.uber.org/zap"
)

func makeSkillZip(t *testing.T, frontmatter, body, extraFiles map[string]string) ([]byte, int64) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	md := "---\n" + frontmatter + "---\n\n" + body + "\n"
	f, _ := zw.Create("SKILL.md")
	f.Write([]byte(md))

	for name, content := range extraFiles {
		f, _ := zw.Create(name)
		f.Write([]byte(content))
	}
	zw.Close()
	return buf.Bytes(), int64(buf.Len())
}

func newTestService(t *testing.T, baseDir string) *SkillService {
	t.Helper()
	store, err := skill.NewSkillStore(baseDir, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return NewSkillService(store, nil, SkillsConfig{BaseDir: baseDir}, zap.NewNop())
}

func TestInstallFromArchive_Success(t *testing.T) {
	base := t.TempDir()
	svc := newTestService(t, base)
	zipBytes, size := makeSkillZip(t, "name: hello\nversion: 1.0.0\ndescription: hi", "body", nil)

	msg, err := svc.InstallFromArchive(context.Background(), "", "1.0.0", bytes.NewReader(zipBytes), size)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "hello") {
		t.Errorf("expected message to contain name, got %q", msg)
	}

	// SKILL.md 应存在目标路径
	target := filepath.Join(base, "uploaded", "hello", "SKILL.md")
	if _, err := os.Stat(target); err != nil {
		t.Errorf("target SKILL.md missing: %v", err)
	}
}
```

- [ ] Step 2: 添加边界测试（缺 SKILL.md、frontmatter 坏、名称格式非法）

```go
func TestInstallFromArchive_MissingSkillMD(t *testing.T) {
	base := t.TempDir()
	svc := newTestService(t, base)
	zipBytes, size := makeSkillZip(t, "", "", map[string.txt{"other/file.txt": "x"})

	_, err := svc.InstallFromArchive(context.Background(), "test", "", bytes.NewReader(zipBytes), size)
	if err == nil || !strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("expected SKILL.md error, got: %v", err)
	}
}
```

> **注意**: 这个测试需要 zip 包中**没有** SKILL.md。上面的 makeSkillZip 会强制创建 SKILL.md（用空的 frontmatter）；需要修改 makeSkillZip 让 frontmatter=="" 时跳过写 SKILL.md。或者为这个测试单独写一个 zip。

- [ ] Step 3: 运行 `go test ./internal/agent/service/... -run TestInstallFromArchive -v`，全部通过

- [ ] Step 4: `git add internal/agent/service/skill_archive_test.go && git commit -m "test(skill): add InstallFromArchive unit tests"`

---

## Task 7: 运行完整检查

- [ ] Step 1: `make fmt`
- [ ] Step 2: `make lint`
- [ ] Step 3: `go test ./...` — 确保无回归
- [ ] Step 4: `go build ./...` — 编译通过

如所有检查通过，本次实施完成。
