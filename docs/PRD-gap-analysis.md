# VanPanel PRD 完成度分析报告

**日期**: 2026-06-09  
**目的**: 对照第十五届中国软件杯 A 组赛题 PRD，逐项评估当前实现完成度，识别缺口与优先级

---

## 一、总览

| PRD 需求 | 权重(估算) | 完成度 | 评级 |
|----------|-----------|--------|------|
| OS 环境深度感知 | 15% | 95% | ★★★★★ |
| MCP 运维插件化 | 15% | 85% | ★★★★☆ |
| 安全意图校验器 | 15% | 75% | ★★★☆☆ |
| 最小权限代理执行 | 10% | 20% | ★☆☆☆☆ |
| 推理链路溯源 | 10% | 85% | ★★★★☆ |
| 智能化根因分析 | 10% | 5% | ☆☆☆☆☆ |
| 非功能性（确定性/抗注入） | 10% | 65% | ★★★☆☆ |
| 竞赛文档与演示 | 20% | ~10% | ★☆☆☆☆ |
| LoongArch + Kylin V11 适配 | — | 未验证 | — |

---

## 二、逐项分析

### 2.1 OS 环境深度感知 — 完成度 95%

**PRD 原文要求**：
> Agent 能够自动调用底层工具（如 lsof, netstat, journalctl）获取进程、网络、日志等实时上下文。

**已实现**：
- 23 个内置工具覆盖 9 大类别（网络/日志/进程/磁盘/系统/服务/Shell/文件/容器/监控），PRD 点名的 lsof、netstat、journalctl 全部实现
- 第二套工具系统（`tools/builtin/`）直接读取 `/proc` 文件系统（CPU load/memory/disk via syscall），无需 shell 子进程
- 跨平台支持：Linux bash + Windows PowerShell 双命令路径
- **2026-06-09 新增**：`file.scan`（文件扫描/列表/搜索/读取）、`container.inspect`（Docker/Podman 容器探测）、`prometheus.query`（PromQL 指标查询）、`sys.inspect`（运行时巡检）、`shell.suggest`（安全命令推荐）
- 旧 `internal/agent/tools/` 死代码已清理

**缺口**：
| # | 问题 | 严重程度 |
|---|------|---------|
| 1 | ~~6 个 MVP stub~~ 已解决 — 新增 3 个核心工具 + 迁移 2 个旧工具 | ✅ |
| 2 | 缺少配置漂移检测能力 — PRD 业务场景明确提到"配置文件漂移"是运维痛点 | 低（加分项） |

**建议**：
- 配置漂移属于差异化加分项，可留到迭代二

---

### 2.2 MCP 运维插件化 — 完成度 85%

**PRD 原文要求**：
> 采用插件化架构（参考 MCP 协议），将常用运维动作封装为 Agent 可调用的 Tools。

**已实现**：
- Local MCP Client（stdio 子进程，支持 MCP initialize/list_tools/call_tool/ping）
- Remote MCP Client（SSE/Streamable HTTP，支持 bearer/basic auth）
- Plugin Hub Service（上传/安装/卸载/启停，manifest 解析，SHA-256 校验，0o755 权限）
- ToolManager 从三个来源聚合工具（内置 + 本地 MCP + 远程 MCP），MCP 工具通过 `MCPToolAdapter` 适配为 Eino `InvokableTool`

**缺口**：
| # | 问题 | 严重程度 |
|---|------|---------|
| 1 | 缺少 L2 技能编排 — 设计文档已定义 YAML Skill 格式但未实现 Runtime 执行器 | 低（加分项） |
| 2 | 缺少 Remote Agent 部署模式 — 设计文档已定义三种远程模式（MCP Remote Agent / SSH / Database MCP），但均未实现 | 低（加分项） |

**建议**：
- L2 Skill 和 Remote Agent 属于差异化加分项，在基础功能完善后再考虑
- 当前 MCP 插件系统已足够满足比赛要求

---

### 2.3 安全意图校验器 — 完成度 75%

**PRD 原文要求**：
> 建立风险识别模型或规则库，对 LLM 生成的原始指令进行"二次过滤"，识别高危参数（如 rm 等参数、不安全的 chmod 等）。

**已实现**：
- **四层防御架构**：SPI Custom Rules → Rule Engine（正则黑/白名单 + 保护路径）→ Auditor Model（独立 LLM 二次审查）→ Approval Decider（人机协同）
- **注入检测**：正则匹配 role hijacking / jailbreak / instruction override / code injection 四种模式
- **配置化规则**：`highRiskPatterns` 和 `shellBlacklistPatterns` 可配置，无需改代码
- **Fail-closed + 优雅降级**：Auditor 不可用时低风险操作进入审批流而非直接阻断

**缺口**：
| # | 问题 | 严重程度 |
|---|------|---------|
| 1 | 注入检测仅用正则，无法识别语义级绕过（如分步诱导、多角色扮演），没有基于 LLM 的注入检测 | 中 |
| 2 | Auditor Model 需要单独配置和部署（`AGENT_AUDITOR_MODEL` / `AGENT_AUDITOR_BASE_URL`），默认未启用 | 中 |
| 3 | 规则库覆盖面有限：仅硬编码了 `rm -rf /`、`dd if=`、`mkfs.`、`shutdown`、`reboot`、fork bomb，缺少对 `chmod`、`chown` 的细粒度管控 | 低 |
| 4 | Evaluator 默认将所有 `shell.exec` 调用标记为 `RiskLow`（需要审批），但这在实际对话中产生大量误报 | 中 |

**建议**：
- P0：扩展 `highRiskPatterns` 和 `shellBlacklistPatterns` 默认配置，覆盖 chmod/chown 等
- P1：接入 LLM 注入检测（当前 Auditor Model 基础设施可复用）
- P2：根据工具类别分级默认风险，降低误报（如 `df -h` = safe 而非 low）

---

### 2.4 最小权限代理执行 — 完成度 20%（最大缺口）

**PRD 原文要求**：
> 实现 Agent 的权限隔离，核心运维动作需在受限的 Account 下运行，非必要不使用 root。

**当前状态**：
- Agent 的所有 shell 命令以 Go 进程的当前用户身份执行，无权限降级
- 唯一的"权限控制"是风险等级门控（safe/low/high）和审批流，这是 **逻辑层面的访问控制**，而非 **OS 层面的权限隔离**

**缺口分析**：

| 层次 | 需求 | 现状 |
|------|------|------|
| OS 用户隔离 | 非 root 受限账户执行运维命令 | ❌ 未实现，与 Go 进程同用户 |
| 命令白名单 | 受限账户只能执行特定命令 | ❌ 未实现 |
| sudo 细粒度控制 | 必要时通过 sudo 执行特定操作 | ❌ 未实现 |
| 文件系统隔离 | 受限账户只能访问特定目录 | ⚠️ 仅 regex 保护路径，非 OS 级 |

**设计方案**（待实现）：

1. 创建专用系统账户 `vanpanel-agent`（无 sudo 权限，home 目录受限）
2. `shell.go` 中的 `runCommand()` 改造：
   ```go
   // 根据风险评估决定执行身份
   if riskLevel == RiskLow || riskLevel == RiskHigh {
       cmd = exec.CommandContext(ctx, "sudo", "-u", "vanpanel-agent", shell, shellFlag, cmdLine)
   } else {
       cmd = exec.CommandContext(ctx, shell, shellFlag, cmdLine)
   }
   ```
3. sudoers 配置仅允许 `vanpanel-agent` 执行白名单命令：
   ```
   vanpanel-agent ALL=(vanpanel-agent) NOPASSWD: /usr/bin/lsof, /usr/bin/ss, ...
   ```
4. 高危命令即使审批通过，也要在受限账户下执行

**建议**：这是 PRD 明确要求的核心功能，也是比赛中与竞品拉开差距的关键点。**优先级 P0。**

---

### 2.5 推理链路溯源 — 完成度 85%

**PRD 原文要求**：
> 完整记录"接收指令 -> 感知环境 -> 推理决策 -> 安全校验 -> 执行结果"的闭环日志，支持异常回溯。

**已实现**：
- 5 类审计事件完整覆盖：`agent.receive` → `tool.evaluate` → `tool.execute` → `tool.blocked` → `agent.complete`
- 内存 + DB 双写（MemoryStore FIFO 2000 条上限 + MySQL `cl_agent_audit_events` 表）
- 会话级过滤（`ListBySession`）
- 系统级 HTTP 审计中间件（`cl_audit_logs` 表）作为第二层

**缺口**：
| # | 问题 | 严重程度 |
|---|------|---------|
| 1 | 推理决策过程（LLM 的中间思考步骤）未记录 — 当前审计只覆盖输入/输出/工具执行，不记录 ReAct 循环中的 Thought/Observation | 中 |
| 2 | 缺少审计日志的前端 UI（搜索/过滤/导出/时间线展示） | 中 |
| 3 | 缺少异常回溯辅助工具（如按 session ID 一键回放完整推理链路） | 低 |

**建议**：
- P1：在审计事件中增加 `thought` 字段，记录 ReAct 每一步的推理过程
- P2：前端审计面板

---

### 2.6 智能化根因分析 — 完成度 5%（第二大缺口）

**PRD 原文要求**：
> 评分项占比：智能化根因分析能力（属于功能完整性 55% 的一部分）

**当前状态**：
- `DefaultIntentAnalyzer` 仅做关键词分类（inspect/diagnose/dangerous/query）
- 没有任何指标关联分析、因果推理或故障定位能力
- 没有诊断知识库（如 "磁盘满 → 排查路径：df → du → lsof 找大文件"）

**缺口分析**：

| 能力 | 需求 | 现状 |
|------|------|------|
| 指标关联 | CPU 高 + 磁盘 I/O 高 → 可能是 Swap 颠簸 | ❌ 未实现 |
| 因果链推理 | 僵尸进程堆积 → 父进程未回收 → systemd 配置问题 | ❌ 未实现 |
| 诊断知识库 | 常见故障的诊断步骤 SOP | ❌ 未实现 |
| 多工具协同诊断 | 自动编排 df → du → lsof → journalctl 形成诊断链 | ⚠️ ReAct 框架支持，但无引导 |

**设计方案**（待实现）：

1. **诊断知识图谱**：定义 `Symptom → Cause Chain → Diagnostic Path → Tool Sequence`
   ```
   磁盘使用率>90%:
     Step 1: disk.df → 定位哪个分区满
     Step 2: disk.du {path} → 定位哪个目录大
     Step 3: proc.pgrep {writing to full disk} → 定位写入进程
     Step 4: log.journalctl {PID} → 查看相关日志
     Step 5: 综合输出诊断报告 + 建议清理方案
   ```
2. **指标关联引擎**：接收多个工具的输出，做时序/数值关联分析
3. **诊断模板库**：YAML 定义的常见故障诊断 SOP（复用 L2 Skill 基础设施）

**建议**：这是比赛评分的重要组成部分，也是演示中最能体现智能化的环节。**优先级 P0。**

---

### 2.7 非功能性需求 — 完成度 65%

**2.7.1 确定性与可靠性**

PRD 要求："严禁 Agent 在未授权情况下修改系统关键配置文件"

- ✅ 保护路径：`/boot`、`/etc`、`/root`、`/usr`、`/var/lib/docker` 写操作被拦截
- ✅ 保护服务：`firewalld`、`sshd`、`ssh`、`docker`、`kubelet` 重启操作被拦截
- ⚠️ 拦截在 Go 层而非 OS 层，若 Go 进程本身以 root 运行，理论上存在绕过风险
- ❌ 缺少自动化测试验证"所有关键路径均不可写"

**2.7.2 抗注入能力**

- ✅ 4 种注入模式的正则检测
- ⚠️ 不支持语义级注入（分步绕过、多语言、编码混淆）
- ❌ 缺少注入对抗测试用例集

---

## 三、竞赛交付物缺口

初赛要求提交 9 项文档，当前状态：

| # | 交付物 | 状态 | 备注 |
|---|--------|------|------|
| 1 | 软件功能需求分析文档 | ❌ 未开始 | 可基于 PRD.md 扩展 |
| 2 | 软件功能设计文档 | ⚠️ 部分完成 | 有 `architecture-design-philosophy.md` 和多个 spec 文档，需整合 |
| 3 | 软件产品说明书 | ❌ 未开始 | — |
| 4 | 软件功能测试报告 | ❌ 未开始 | — |
| 5 | 软件性能测试报告 | ❌ 未开始 | — |
| 6 | 软件安装包及部署文档 | ❌ 未开始 | 需验证 LoongArch + Kylin V11 部署流程 |
| 7 | 软件源代码文件 | ✅ 已有 | 代理模块代码已齐全 |
| 8 | 软件功能演示 PPT | ❌ 未开始 | — |
| 9 | 功能演示视频（≤7 分钟） | ❌ 未开始 | — |

---

## 四、优先级排序建议

### P0 — 必须完成（直接影响评分）

| 序号 | 任务 | 预估工作量 | 对应 PRD 需求 |
|------|------|-----------|-------------|
| P0-1 | **最小权限代理执行** — OS 级权限隔离 | 中 | 基本功能需求 #4 |
| P0-2 | **智能化根因分析** — 诊断知识图谱 + 指标关联 | 大 | 评分项：智能化根因分析 |
| P0-3 | ~~**完善 stub 工具** — container.inspect / prometheus.query / file.scan~~ ✅ 已完成 (2026-06-09) | — | 基本功能需求 #1 |
| P0-4 | **LoongArch + Kylin V11 适配验证** | 中 | 实现条件 |

### P1 — 重要但不阻塞

| 序号 | 任务 | 预估工作量 | 说明 |
|------|------|-----------|------|
| P1-1 | 扩展安全规则库（chmod/chown 细粒度管控） | 小 | 提升安全校验器覆盖度 |
| P1-2 | 增强审计链路（记录 ReAct Thought + 前端 UI） | 中 | 提升推理链路溯源完整度 |
| P1-3 | LLM 注入检测（复用 Auditor Model 基础设施） | 中 | 提升抗注入能力 |
| P1-4 | 安全护栏自动化测试 | 中 | 确保非功能性需求达标 |

### P2 — 加分项（差异化竞争）

| 序号 | 任务 | 说明 |
|------|------|------|
| P2-1 | L2 Skill 编排 Runtime | YAML 工作流执行器 |
| P2-2 | Remote Agent 部署模式 | MCP Remote Agent 子进程 |
| P2-3 | 配置漂移检测 | PRD 业务场景的差异化实现 |

### 文档 — 持续进行

| 序号 | 任务 |
|------|------|
| D-1 | 软件功能需求分析文档 |
| D-2 | 软件功能设计文档 |
| D-3 | 软件产品说明书 |
| D-4 | 软件功能测试报告 |
| D-5 | 软件性能测试报告 |
| D-6 | 软件安装包及部署文档 |
| D-7 | PPT + 演示视频（最后交付） |

---

## 五、风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| LoongArch 架构兼容性未知 | 可能导致无法部署 | 尽早获取麒麟虚拟机进行验证 |
| 根因分析设计复杂度高 | 工作量超预期 | 先实现 3-5 个核心诊断场景，不求全 |
| 权限隔离可能影响现有工具功能 | 某些命令需要 root 权限 | 通过 sudoers 白名单 + Capability 机制精细控制 |
| 时间不足完成全部交付物 | 文档 + PPT + 视频耗时 | 代码和文档并行推进，PPT/视频最后两周集中冲刺 |

---

## 六、架构演进路线

```
当前状态                    近期目标 (P0)                   中期目标 (P1)                   远期目标 (P2)
─────────────────────────────────────────────────────────────────────────────────────────────────
4层安全护栏                 + OS级权限隔离                   + LLM注入检测                   + L2 Skill编排
23个内置工具 (9大类别)      + 根因分析引擎                   + 审计前端UI                    + Remote Agent
MCP插件系统                                               + 安全自动化测试                 + 配置漂移检测
3层持久记忆                                
完整审计追踪                                
─────────────────────────────────────────────────────────────────────────────────────────────────
```
