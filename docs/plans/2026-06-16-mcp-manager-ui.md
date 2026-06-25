# MCP Manager UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the existing `/mcp` config-only page with a full MCP management page that manages plugins, remote servers, and builtin tools via the backend REST API.

**Architecture:** Self-contained Lit view component that uses `fetch()` for REST API calls (not the WebSocket gateway). The view owns its own state and renders three tabbed sub-panels. The replacement happens by modifying the `case "mcp"` branch in `app-render.ts`.

**Tech Stack:** TypeScript, Lit (html tagged templates), Vite dev proxy (`/api` → `localhost:8889`)

**API Base:** `/api/system/agent`

**Response Format (backend):**
```json
{ "code": 0, "data": ..., "message": "请求成功" }
```
`code: 0` = success, `code: 1` = error.

---

### Task 1: Create MCP Manager Controller

**Files:**
- Create: `webui/src/ui/controllers/mcp-manager.ts`

**Step 1: Write the controller**

The controller handles all data fetching and state management for the MCP manager. It exports types and async load functions.

```typescript
// webui/src/ui/controllers/mcp-manager.ts

const API_BASE = "/api/system/agent";

// --- Response types matching backend models ---

export interface McpPlugin {
  id: number;
  name: string;
  display_name: string;
  description: string;
  version: string;
  author: string;
  category: string;
  icon_url: string;
  status: string;       // "active" | "disabled" | "error"
  binary_path: string;
  created_at: string;
  updated_at: string;
}

export interface RemoteMcpConfig {
  id: number;
  user_id: number;
  name: string;
  description: string;
  transport: string;    // "sse" | "streamable-http"
  url: string;
  auth_type: string;    // "none" | "bearer" | "basic"
  auth_config: Record<string, unknown>;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface BuiltinTool {
  name: string;
  display_name: string;
  description: string;
  category: string;
  enabled: boolean;
  config: Record<string, unknown>;
}

interface ApiResponse<T> {
  code: number;
  data: T;
  message: string;
}

interface ListResponse<T> {
  list: T[];
  total: number;
}

// --- API helpers ---

async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  const json: ApiResponse<T> = await res.json();
  if (json.code !== 0) throw new Error(json.message || "Request failed");
  return json.data;
}

async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const json: ApiResponse<T> = await res.json();
  if (json.code !== 0) throw new Error(json.message || "Request failed");
  return json.data;
}

async function apiPut(path: string, body?: unknown): Promise<void> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "PUT",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const json: ApiResponse<unknown> = await res.json();
  if (json.code !== 0) throw new Error(json.message || "Request failed");
}

async function apiDelete(path: string): Promise<void> {
  const res = await fetch(`${API_BASE}${path}`, { method: "DELETE" });
  const json: ApiResponse<unknown> = await res.json();
  if (json.code !== 0) throw new Error(json.message || "Request failed");
}

async function apiPostFormData<T>(path: string, formData: FormData): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    body: formData,
  });
  const json: ApiResponse<T> = await res.json();
  if (json.code !== 0) throw new Error(json.message || "Request failed");
  return json.data;
}

// --- Plugin operations ---

export async function listPlugins(): Promise<McpPlugin[]> {
  const data = await apiGet<ListResponse<McpPlugin>>("/hub/plugins/list");
  return data.list ?? [];
}

export async function uploadPlugin(
  manifest: string,
  binary: File,
  onProgress?: (pct: number) => void,
): Promise<McpPlugin> {
  const fd = new FormData();
  fd.append("manifest", manifest);
  fd.append("binary", binary);
  // Note: progress tracking requires XMLHttpRequest; for simplicity, skip for now
  return apiPostFormData<McpPlugin>("/hub/plugins/upload", fd);
}

export async function installPlugin(pluginId: number): Promise<void> {
  await apiPost(`/hub/plugins/${pluginId}/install`);
}

export async function uninstallPlugin(pluginId: number): Promise<void> {
  await apiDelete(`/hub/plugins/${pluginId}/uninstall`);
}

export async function togglePlugin(pluginId: number): Promise<void> {
  await apiPut(`/hub/plugins/${pluginId}/toggle`);
}

// --- Remote MCP Config operations ---

export async function listRemoteConfigs(): Promise<RemoteMcpConfig[]> {
  const data = await apiGet<ListResponse<RemoteMcpConfig>>("/remote-mcps/list");
  return data.list ?? [];
}

export async function createRemoteConfig(config: {
  name: string;
  description?: string;
  transport: string;
  url: string;
  auth_type: string;
  auth_config?: Record<string, unknown>;
}): Promise<RemoteMcpConfig> {
  return apiPost<RemoteMcpConfig>("/remote-mcps/create", config);
}

export async function updateRemoteConfig(
  id: number,
  config: {
    name: string;
    description?: string;
    transport: string;
    url: string;
    auth_type: string;
    auth_config?: Record<string, unknown>;
  },
): Promise<void> {
  await apiPut(`/remote-mcps/${id}/update`, config);
}

export async function deleteRemoteConfig(id: number): Promise<void> {
  await apiDelete(`/remote-mcps/${id}/delete`);
}

export async function toggleRemoteConfig(id: number): Promise<void> {
  await apiPut(`/remote-mcps/${id}/toggle`);
}

export async function testRemoteConfig(id: number): Promise<{ ok: boolean; message: string }> {
  return apiPost(`/remote-mcps/${id}/test`);
}

// --- Builtin Tool operations ---

export async function listBuiltinTools(): Promise<BuiltinTool[]> {
  const data = await apiGet<ListResponse<BuiltinTool>>("/builtin-tools/list");
  return data.list ?? [];
}

export async function toggleBuiltinTool(name: string): Promise<void> {
  await apiPut(`/builtin-tools/${name}/toggle`);
}
```

**Step 2: No test to run yet**

---

### Task 2: Create MCP Manager View

**Files:**
- Create: `webui/src/ui/views/mcp-manager.ts`

**Step 1: Write the view component**

The view is a self-contained render function with internal state. It uses Lit's `html` tagged templates.

```typescript
// webui/src/ui/views/mcp-manager.ts

import { html, nothing, type TemplateResult } from "lit";
import { ref, createRef, type Ref } from "lit/directives/ref.js";
import {
  listPlugins,
  togglePlugin,
  installPlugin,
  uninstallPlugin,
  uploadPlugin,
  listRemoteConfigs,
  toggleRemoteConfig,
  deleteRemoteConfig,
  testRemoteConfig,
  listBuiltinTools,
  toggleBuiltinTool,
  type McpPlugin,
  type RemoteMcpConfig,
  type BuiltinTool,
} from "../controllers/mcp-manager.ts";

// --- Types ---

type SubTab = "plugins" | "remote" | "tools";

interface McpManagerState {
  activeTab: SubTab;
  loading: boolean;
  error: string | null;
  // Plugin data
  plugins: McpPlugin[];
  pluginsLoading: boolean;
  pluginsError: string | null;
  uploadOpen: boolean;
  uploading: boolean;
  // Remote config data
  remoteConfigs: RemoteMcpConfig[];
  remoteLoading: boolean;
  remoteError: string | null;
  editOpen: boolean;
  editingId: number | null;
  testing: Set<number>;
  testResults: Map<number, { ok: boolean; message: string }>;
  // Builtin tool data
  builtinTools: BuiltinTool[];
  builtinLoading: boolean;
  builtinError: string | null;
  categoriesExpanded: Set<string>;
}

// --- Render entry ---

export function renderMcpManager(): TemplateResult {
  // State is maintained via a module-level closure
  const state: McpManagerState = {
    activeTab: "plugins",
    loading: false,
    error: null,
    plugins: [],
    pluginsLoading: false,
    pluginsError: null,
    uploadOpen: false,
    uploading: false,
    remoteConfigs: [],
    remoteLoading: false,
    remoteError: null,
    editOpen: false,
    editingId: null,
    testing: new Set(),
    testResults: new Map(),
    builtinTools: [],
    builtinLoading: false,
    builtinError: null,
    categoriesExpanded: new Set(),
  };

  // Placeholder for now - will be fleshed out in Task 3
  return html`
    <section class="mcp-manager">
      <div class="mcp-manager__tabs">
        <button class="mcp-manager__tab ${state.activeTab === "plugins" ? "active" : ""}">
          🔌 插件市场
        </button>
        <button class="mcp-manager__tab ${state.activeTab === "remote" ? "active" : ""}">
          🌐 远程服务器
        </button>
        <button class="mcp-manager__tab ${state.activeTab === "tools" ? "active" : ""}">
          🔧 内置工具
        </button>
      </div>
      <div class="mcp-manager__content">
        ${renderPluginsPanel(state)} ${renderRemotePanel(state)} ${renderToolsPanel(state)}
      </div>
    </section>
  `;
}

// --- Plugin Panel ---

function renderPluginsPanel(state: McpManagerState): TemplateResult {
  return html`<div>Plugin panel placeholder</div>`;
}

// --- Remote Server Panel ---

function renderRemotePanel(state: McpManagerState): TemplateResult {
  return html`<div>Remote panel placeholder</div>`;
}

// --- Builtin Tools Panel ---

function renderToolsPanel(state: McpManagerState): TemplateResult {
  return html`<div>Tools panel placeholder</div>`;
}
```

**Step 2: No test to run yet**

---

### Task 3: Implement Plugin Panel

**Files:**
- Modify: `webui/src/ui/views/mcp-manager.ts`

**Step 1: Implement the full plugin panel rendering**

The plugin panel shows a list of plugins with upload, install, uninstall, and toggle actions.

```typescript
function renderPluginsPanel(state: McpManagerState): TemplateResult {
  if (state.activeTab !== "plugins") return html``;

  return html`
    <div class="mcp-panel">
      <div class="mcp-panel__header">
        <h3>插件市场</h3>
        <button
          class="btn btn--sm primary"
          @click=${() => openUploadDialog(state)}>
          + 上传插件
        </button>
      </div>

      ${state.pluginsError
        ? html`<div class="error-banner">${state.pluginsError}</div>`
        : nothing}
      ${state.pluginsLoading
        ? html`<div class="loading-spinner">加载中...</div>`
        : state.plugins.length === 0
          ? html`<div class="empty-state">暂无插件</div>`
          : html`
              <table class="data-table">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>版本</th>
                    <th>状态</th>
                    <th>安装时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  ${state.plugins.map((p) => renderPluginRow(state, p))}
                </tbody>
              </table>
            `}

      ${state.uploadOpen ? renderUploadDialog(state) : nothing}
    </div>
  `;
}

function renderPluginRow(state: McpManagerState, p: McpPlugin): TemplateResult {
  const statusClass = p.status === "active" ? "pill--ok" : p.status === "error" ? "pill--danger" : "pill--warn";
  const statusText = p.status === "active" ? "已启用" : p.status === "error" ? "异常" : "已禁用";
  return html`
    <tr>
      <td>
        <div class="plugin-name">${p.display_name || p.name}</div>
        <div class="plugin-desc">${p.description || ""}</div>
      </td>
      <td>${p.version || "-"}</td>
      <td><span class="pill ${statusClass}">${statusText}</span></td>
      <td>${formatDate(p.created_at)}</td>
      <td class="actions-cell">
        <button class="btn btn--sm" @click=${() => togglePluginAction(state, p)}>
          ${p.status === "active" ? "禁用" : "启用"}
        </button>
        <button class="btn btn--sm" @click=${() => uninstallPluginAction(state, p)}>
          卸载
        </button>
      </td>
    </tr>
  `;
}

function openUploadDialog(state: McpManagerState) {
  state.uploadOpen = true;
  requestHostUpdate();
}

function renderUploadDialog(state: McpManagerState): TemplateResult {
  const manifestRef: Ref<HTMLTextAreaElement> = createRef();
  const fileRef: Ref<HTMLInputElement> = createRef();

  const handleUpload = async () => {
    const manifest = manifestRef.value?.value;
    const file = fileRef.value?.files?.[0];
    if (!manifest || !file) return;
    state.uploading = true;
    requestHostUpdate();
    try {
      await uploadPlugin(manifest, file);
      state.uploadOpen = false;
      await loadPlugins(state);
    } catch (e: unknown) {
      state.pluginsError = (e as Error).message;
    } finally {
      state.uploading = false;
      requestHostUpdate();
    }
  };

  return html`
    <div class="modal-overlay" @click=${() => { state.uploadOpen = false; requestHostUpdate(); }}>
      <div class="modal" @click=${(e: Event) => e.stopPropagation()}>
        <h4>上传插件</h4>
        <label>插件清单 (JSON):</label>
        <textarea ${ref(manifestRef)} rows="4" class="form-input"></textarea>
        <label>二进制文件:</label>
        <input ${ref(fileRef)} type="file" class="form-input" />
        <div class="modal-actions">
          <button class="btn" @click=${() => { state.uploadOpen = false; requestHostUpdate(); }}>
            取消
          </button>
          <button class="btn primary" ?disabled=${state.uploading} @click=${handleUpload}>
            ${state.uploading ? "上传中..." : "上传"}
          </button>
        </div>
      </div>
    </div>
  `;
}

async function loadPlugins(state: McpManagerState) {
  state.pluginsLoading = true;
  state.pluginsError = null;
  requestHostUpdate();
  try {
    state.plugins = await listPlugins();
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
  } finally {
    state.pluginsLoading = false;
    requestHostUpdate();
  }
}

async function togglePluginAction(state: McpManagerState, p: McpPlugin) {
  try {
    await togglePlugin(p.id);
    await loadPlugins(state);
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
    requestHostUpdate();
  }
}

async function uninstallPluginAction(state: McpManagerState, p: McpPlugin) {
  try {
    await uninstallPlugin(p.id);
    await loadPlugins(state);
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
    requestHostUpdate();
  }
}
```

---

### Task 4: Implement Remote Server Panel

**Files:**
- Modify: `webui/src/ui/views/mcp-manager.ts`

**Step 1: Implement remote server panel rendering**

```typescript
function renderRemotePanel(state: McpManagerState): TemplateResult {
  if (state.activeTab !== "remote") return html``;

  return html`
    <div class="mcp-panel">
      <div class="mcp-panel__header">
        <h3>远程 MCP 服务器</h3>
        <button class="btn btn--sm primary" @click=${() => openEditDialog(state, null)}>
          + 添加服务器
        </button>
      </div>

      ${state.remoteError
        ? html`<div class="error-banner">${state.remoteError}</div>`
        : nothing}
      ${state.remoteLoading
        ? html`<div class="loading-spinner">加载中...</div>`
        : state.remoteConfigs.length === 0
          ? html`<div class="empty-state">暂无远程服务器配置</div>`
          : html`
              <table class="data-table">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>URL</th>
                    <th>传输</th>
                    <th>认证</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  ${state.remoteConfigs.map((c) => renderRemoteRow(state, c))}
                </tbody>
              </table>
            `}

      ${state.editOpen ? renderEditDialog(state) : nothing}
    </div>
  `;
}

function renderRemoteRow(state: McpManagerState, c: RemoteMcpConfig): TemplateResult {
  const enabledClass = c.enabled ? "pill--ok" : "pill--warn";
  const testing = state.testing.has(c.id);
  const testResult = state.testResults.get(c.id);

  return html`
    <tr>
      <td>
        <div class="server-name">${c.name}</div>
        ${c.description ? html`<div class="server-desc">${c.description}</div>` : nothing}
      </td>
      <td class="mono">${c.url}</td>
      <td>${c.transport}</td>
      <td>${c.auth_type === "none" ? "无" : c.auth_type}</td>
      <td>
        <span class="pill ${enabledClass}">${c.enabled ? "启用" : "禁用"}</span>
      </td>
      <td class="actions-cell">
        <button class="btn btn--sm" @click=${() => testRemoteAction(state, c)}>
          ${testing ? "测试中..." : testResult ? (testResult.ok ? "✓" : "✗") : "测试"}
        </button>
        <button class="btn btn--sm" @click=${() => openEditDialog(state, c)}>编辑</button>
        <button class="btn btn--sm" @click=${() => toggleRemoteAction(state, c)}>
          ${c.enabled ? "禁用" : "启用"}
        </button>
        <button class="btn btn--sm danger" @click=${() => deleteRemoteAction(state, c)}>删除</button>
      </td>
    </tr>
  `;
}

// --- Edit Dialog ---

function openEditDialog(state: McpManagerState, config: RemoteMcpConfig | null) {
  state.editOpen = true;
  state.editingId = config?.id ?? null;
  // Store form data in a closure - simplified approach
  requestHostUpdate();
}

function renderEditDialog(state: McpManagerState): TemplateResult {
  // For now, a simplified inline form
  const nameRef: Ref<HTMLInputElement> = createRef();
  const urlRef: Ref<HTMLInputElement> = createRef();
  const transportRef: Ref<HTMLSelectElement> = createRef();
  const authRef: Ref<HTMLSelectElement> = createRef();

  const handleSave = async () => {
    const data = {
      name: nameRef.value?.value ?? "",
      transport: transportRef.value?.value ?? "sse",
      url: urlRef.value?.value ?? "",
      auth_type: authRef.value?.value ?? "none",
    };
    if (!data.name || !data.url) return;
    // Load create/update functions from controller
    const { createRemoteConfig, updateRemoteConfig } = await import("../controllers/mcp-manager.ts");
    try {
      if (state.editingId) {
        await updateRemoteConfig(state.editingId, data);
      } else {
        await createRemoteConfig(data);
      }
      state.editOpen = false;
      await loadRemote(state);
    } catch (e: unknown) {
      state.remoteError = (e as Error).message;
      requestHostUpdate();
    }
  };

  return html`
    <div class="modal-overlay" @click=${() => { state.editOpen = false; requestHostUpdate(); }}>
      <div class="modal" @click=${(e: Event) => e.stopPropagation()}>
        <h4>${state.editingId ? "编辑服务器" : "添加服务器"}</h4>
        <label>名称:</label>
        <input ${ref(nameRef)} class="form-input" placeholder="例如: prod-monitor" />
        <label>URL:</label>
        <input ${ref(urlRef)} class="form-input" placeholder="例如: http://localhost:9000/mcp" />
        <label>传输类型:</label>
        <select ${ref(transportRef)} class="form-input">
          <option value="sse">SSE</option>
          <option value="streamable-http">Streamable HTTP</option>
        </select>
        <label>认证方式:</label>
        <select ${ref(authRef)} class="form-input">
          <option value="none">无</option>
          <option value="bearer">Bearer Token</option>
          <option value="basic">Basic Auth</option>
        </select>
        <div class="modal-actions">
          <button class="btn" @click=${() => { state.editOpen = false; requestHostUpdate(); }}>取消</button>
          <button class="btn primary" @click=${handleSave}>保存</button>
        </div>
      </div>
    </div>
  `;
}

async function loadRemote(state: McpManagerState) {
  state.remoteLoading = true;
  state.remoteError = null;
  requestHostUpdate();
  try {
    state.remoteConfigs = await listRemoteConfigs();
  } catch (e: unknown) {
    state.remoteError = (e as Error).message;
  } finally {
    state.remoteLoading = false;
    requestHostUpdate();
  }
}

async function testRemoteAction(state: McpManagerState, c: RemoteMcpConfig) {
  state.testing.add(c.id);
  state.testResults.delete(c.id);
  requestHostUpdate();
  try {
    const result = await testRemoteConfig(c.id);
    state.testResults.set(c.id, result);
  } catch (e: unknown) {
    state.testResults.set(c.id, { ok: false, message: (e as Error).message });
  } finally {
    state.testing.delete(c.id);
    requestHostUpdate();
  }
}

async function toggleRemoteAction(state: McpManagerState, c: RemoteMcpConfig) {
  try {
    await toggleRemoteConfig(c.id);
    await loadRemote(state);
  } catch (e: unknown) {
    state.remoteError = (e as Error).message;
    requestHostUpdate();
  }
}

async function deleteRemoteAction(state: McpManagerState, c: RemoteMcpConfig) {
  if (!confirm(`确定要删除 "${c.name}" 吗?`)) return;
  try {
    await deleteRemoteConfig(c.id);
    await loadRemote(state);
  } catch (e: unknown) {
    state.remoteError = (e as Error).message;
    requestHostUpdate();
  }
}
```

---

### Task 5: Implement Builtin Tools Panel

**Files:**
- Modify: `webui/src/ui/views/mcp-manager.ts`

**Step 1: Implement builtin tools panel**

```typescript
function renderToolsPanel(state: McpManagerState): TemplateResult {
  if (state.activeTab !== "tools") return html``;

  const grouped = groupByCategory(state.builtinTools);
  const enabledCount = state.builtinTools.filter((t) => t.enabled).length;
  const totalCount = state.builtinTools.length;

  return html`
    <div class="mcp-panel">
      <div class="mcp-panel__header">
        <h3>内置工具 (${enabledCount}/${totalCount})</h3>
        <div class="mcp-panel__header-actions">
          <button class="btn btn--sm" @click=${() => toggleAllTools(state, true)}>全部启用</button>
          <button class="btn btn--sm" @click=${() => toggleAllTools(state, false)}>全部禁用</button>
        </div>
      </div>

      ${state.builtinError
        ? html`<div class="error-banner">${state.builtinError}</div>`
        : nothing}
      ${state.builtinLoading
        ? html`<div class="loading-spinner">加载中...</div>`
        : html`
            ${Array.from(grouped.entries()).map(([category, tools]) =>
              renderCategoryGroup(state, category, tools),
            )}
          `}
    </div>
  `;
}

function groupByCategory(tools: BuiltinTool[]): Map<string, BuiltinTool[]> {
  const map = new Map<string, BuiltinTool[]>();
  for (const t of tools) {
    const cat = t.category || "其他";
    if (!map.has(cat)) map.set(cat, []);
    map.get(cat)!.push(t);
  }
  return map;
}

function renderCategoryGroup(
  state: McpManagerState,
  category: string,
  tools: BuiltinTool[],
): TemplateResult {
  const isExpanded = state.categoriesExpanded.has(category);
  const categoryEnabled = tools.filter((t) => t.enabled).length;

  return html`
    <details
      class="tool-category"
      ?open=${isExpanded}
      @toggle=${(e: Event) => {
        const el = e.target as HTMLDetailsElement;
        if (el.open) state.categoriesExpanded.add(category);
        else state.categoriesExpanded.delete(category);
      }}
    >
      <summary class="tool-category__header">
        <span>${category}</span>
        <span class="tool-category__count">${categoryEnabled}/${tools.length}</span>
      </summary>
      <div class="tool-category__list">
        ${tools.map((t) =>
          renderToolRow(state, t),
        )}
      </div>
    </details>
  `;
}

function renderToolRow(state: McpManagerState, t: BuiltinTool): TemplateResult {
  return html`
    <div class="tool-row">
      <div class="tool-row__info">
        <div class="tool-name">${t.name}</div>
        <div class="tool-desc">${t.description || ""}</div>
      </div>
      <div class="tool-row__actions">
        <span class="pill ${t.enabled ? "pill--ok" : "pill--warn"}">
          ${t.enabled ? "启用" : "禁用"}
        </span>
        <button class="btn btn--sm" @click=${() => toggleToolAction(state, t)}>
          ${t.enabled ? "禁用" : "启用"}
        </button>
      </div>
    </div>
  `;
}

async function loadBuiltinTools(state: McpManagerState) {
  state.builtinLoading = true;
  state.builtinError = null;
  requestHostUpdate();
  try {
    state.builtinTools = await listBuiltinTools();
  } catch (e: unknown) {
    state.builtinError = (e as Error).message;
  } finally {
    state.builtinLoading = false;
    requestHostUpdate();
  }
}

async function toggleToolAction(state: McpManagerState, t: BuiltinTool) {
  try {
    await toggleBuiltinTool(t.name);
    await loadBuiltinTools(state);
  } catch (e: unknown) {
    state.builtinError = (e as Error).message;
    requestHostUpdate();
  }
}

async function toggleAllTools(state: McpManagerState, enable: boolean) {
  for (const t of state.builtinTools) {
    if (t.enabled !== enable) {
      try {
        await toggleBuiltinTool(t.name);
      } catch {
        // continue with remaining tools
      }
    }
  }
  await loadBuiltinTools(state);
}
```

---

### Task 6: Utility Functions and re-render trigger

**Files:**
- Modify: `webui/src/ui/views/mcp-manager.ts`

**Step 1: Add utilities and the re-render bridge**

```typescript
// --- Utilities ---

function formatDate(dateStr: string): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// --- Re-render trigger ---

// The MCP Manager view uses module-level state. To trigger re-renders,
// we need to call the host component's requestUpdate(). We store a reference
// to the host's requestUpdate function.

let requestHostUpdate: () => void = () => {};

export function setMcpManagerUpdateTrigger(fn: () => void) {
  requestHostUpdate = fn;
}
```

---

### Task 7: Integrate lazy loading of MCP data

**Files:**
- Modify: `webui/src/ui/views/mcp-manager.ts`

**Step 1: Add lazy loading trigger to renderMcpManager**

When the view first renders, kick off data loading for the active tab and all tabs in background.

```typescript
// Track whether we've initialized data loading
let dataInitialized = false;

export function renderMcpManager(): TemplateResult {
  // Initialize state once
  if (!_state) {
    _state = createInitialState();
  }

  // Trigger initial data load
  if (!dataInitialized) {
    dataInitialized = true;
    queueMicrotask(() => {
      loadPlugins(_state!);
      loadRemote(_state!);
      loadBuiltinTools(_state!);
    });
  }

  return html`...`;  // rest as before
}
```

---

### Task 8: Integrate into app-render.ts

**Files:**
- Modify: `webui/src/ui/app-render.ts`

**Step 1: Replace the import**

Change:
```typescript
import { renderMcp } from "./views/mcp.ts";
```
To:
```typescript
import { renderMcpManager, setMcpManagerUpdateTrigger } from "./views/mcp-manager.ts";
```

**Step 2: Replace the `case "mcp"` handler**

Find the `case "mcp":` block (approx lines 1984-2014) and replace with:

```typescript
case "mcp": {
  setMcpManagerUpdateTrigger(() => state.requestUpdate());
  return renderMcpManager();
}
```

**Step 3: Remove the old import remnants**

If there are other references to `renderMcp` or `updateMcpServerEnabled`, remove them. Check if `McpViewProps` type is used elsewhere - if not, no additional cleanup needed.

---

### Task 9: Verify the build

**Files:**
- None (verification only)

**Step 1: Run TypeScript check**

```bash
cd webui && npx tsc --noEmit
```
Expected: No type errors.

**Step 2: Run Vite build**

```bash
cd webui && npx vite build
```
Expected: Build succeeds without errors.

**Step 3: Commit**

```bash
git add webui/src/ui/controllers/mcp-manager.ts
git add webui/src/ui/views/mcp-manager.ts
git add webui/src/ui/app-render.ts
git commit -m "feat: replace /mcp page with full MCP management UI"
```
