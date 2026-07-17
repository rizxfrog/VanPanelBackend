# Skill Archive Upload 设计

**日期**: 2026-07-16
**状态**: approved

## 需求摘要

支持通过 HTTP multipart 上传 zip 压缩包来安装/更新 skill。当前仅 ClawHub 远程安装可用，本地开发调试和私有 skill 需要直接上传 zip 的路径。

## 路由

```
POST /api/system/agent/skills/upload
Content-Type: multipart/form-data
Access: ScopeAdmin (复用现有 admin scope)
```

Multipart 字段：
- `file` (必需) — zip 压缩包
- `name` (可选) — skill 名称；缺省时从 zip 内 SKILL.md frontmatter 推断
- `version` (可选) — 版本号；缺省 "0.0.0-local"

## 大小限制

- 默认 10MB，通过 `MemorySize` 限制 + 磁盘临时文件回退
- 超限返回 413

## 处理流程

```
1. 通过 c.Request.ParseMultipartForm(10 << 20) 解析，超限返回 413
2. 读取 multipart "file" 字段；缺失返回 400
3. 校验可选 name 字段格式（a-z0-9-，<=64字符）；缺省时暂置空
4. 将 zip 文件读入字节（已在 ParseMultipartForm 限制的内存内）
5. 解压到临时目录（os.MkdirTemp），失败返回 400
6. 校验临时目录中必须存在 SKILL.md 且 frontmatter 解析成功
7. 若 name 仍为空，从 frontmatter 中取 meta.Name；仍为空返回 422
8. 校验 name 格式（a-z0-9-，<=64字符）；失败返回 422
9. 目标路径 = data/skills/uploaded/<name>/；已存在同名 skill 则先 DeleteSkill
10. 将临时目录 rename/移到目标路径（os.Rename 或 copyfallback）
11. 重写目标 SKILL.md frontmatter：设置 source=uploaded、确保无 ClawHub 字段（清空）
12. 更新 usage 统计状态为 active
13. 返回 {ok: true, name, version}
14. defer 清理临时目录（无论成功与否）
```

## 解复用

- 提取 `internal/agent/service/skill_service.go` 中的匿名 zip 解压逻辑为独立导出函数 `UnzipToDir(r io.ReaderAt, size int64, targetDir string) error`
- `installFromClawHub` 和 `InstallFromArchive` 都复用该函数

## 来源标记

- 新增 `SkillSourceUploaded SkillSource = "uploaded"`
- 写入 SKILL.md frontmatter 时设置 `source: uploaded`
- ClawHub 字段不设置（区别于 clawhub 安装）

## 幂等性

- 同名 skill 已存在时覆盖（先删后建）
- 临时目录通过 t.Cleanup 确保清理

## 错误码

| 场景 | HTTP | 业务码 |
|------|------|--------|
| 缺文件 | 400 | "缺少 skill 压缩包" |
| 大小超限 | 413 | "文件过大，最大 10MB" |
| zip 损坏 | 400 | "无效的 zip 压缩包" |
| 缺 SKILL.md | 400 | "压缩包中缺少 SKILL.md" |
| frontmatter 无法解析 | 422 | "SKILL.md frontmatter 解析失败" |
| 名称为空 | 422 | "无法推断 skill 名称" |
| 名称格式非法 | 422 | "skill 名称格式无效" |

## 修改文件清单

| 文件 | 改动 |
|------|------|
| `internal/agent/skill/types.go` | 新增 `SkillSourceUploaded` |
| `internal/agent/api/handler.go` | 新增 `UploadSkill` handler + route |
| `internal/agent/service/skill_service.go` | 新增 `InstallFromArchive`；提取 `UnzipToDir`；改造 `installFromClawHub` 复用 |
| `pkg/di/agent.go` | 无改动 |

## 不修改

- `skills.upload.begin/chunk/commit` 空 stub — 保留作为未来的 Gateway 分块上传接口
- `SkillStore` — 已有 `CreateSkill`、`DeleteSkill`、`ListSkills`

## 测试

- 单元测试 `TestSkillService_InstallFromArchive`：用内存 zip 验证解压 + 元数据写入
- 缺文件 / 超大小 / 无 SKILL.md / 坏 frontmatter 的边界用例
