// MCP Manager View — self-contained Lit view with three tabbed sub-panels.
import { html, nothing, type TemplateResult } from "lit";
import {
  listPlugins,
  uploadPlugin,
  installPlugin,
  uninstallPlugin,
  togglePlugin,
  listRemoteConfigs,
  createRemoteConfig,
  updateRemoteConfig,
  deleteRemoteConfig,
  toggleRemoteConfig,
  testRemoteConfig,
  listBuiltinTools,
  toggleBuiltinTool,
  type McpPlugin,
  type RemoteMcpConfig,
  type BuiltinTool,
} from "../controllers/mcp-manager.ts";

// --------------- types ---------------

interface McpManagerState {
  activeTab: "plugins" | "remote" | "tools";
  loading: boolean;
  error: string | null;
  // Plugin panel
  plugins: McpPlugin[];
  pluginsLoading: boolean;
  pluginsError: string | null;
  uploadOpen: boolean;
  uploading: boolean;
  // Remote panel
  remoteConfigs: RemoteMcpConfig[];
  remoteLoading: boolean;
  remoteError: string | null;
  editOpen: boolean;
  editingId: number | null;
  testing: Set<number>;
  testResults: Map<number, { ok: boolean; message: string }>;
  // Builtin panel
  builtinTools: BuiltinTool[];
  builtinLoading: boolean;
  builtinError: string | null;
  categoriesExpanded: Set<string>;
}

// --------------- module-level state + trigger ---------------

let requestHostUpdate: () => void = () => {};
export function setMcpManagerUpdateTrigger(fn: () => void) {
  requestHostUpdate = fn;
}

let _state: McpManagerState | null = null;
let _dataInitialized = false;

function getState(): McpManagerState {
  if (!_state) {
    _state = {
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
  }
  return _state;
}

// --------------- data loaders ---------------

async function loadPlugins(state: McpManagerState) {
  try {
    state.pluginsLoading = true;
    state.pluginsError = null;
    requestHostUpdate();
    state.plugins = await listPlugins();
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
  } finally {
    state.pluginsLoading = false;
    requestHostUpdate();
  }
}

async function loadRemote(state: McpManagerState) {
  try {
    state.remoteLoading = true;
    state.remoteError = null;
    requestHostUpdate();
    state.remoteConfigs = await listRemoteConfigs();
  } catch (e: unknown) {
    state.remoteError = (e as Error).message;
  } finally {
    state.remoteLoading = false;
    requestHostUpdate();
  }
}

async function loadBuiltinTools(state: McpManagerState) {
  try {
    state.builtinLoading = true;
    state.builtinError = null;
    requestHostUpdate();
    state.builtinTools = await listBuiltinTools();
  } catch (e: unknown) {
    state.builtinError = (e as Error).message;
  } finally {
    state.builtinLoading = false;
    requestHostUpdate();
  }
}

// --------------- plugin action handlers ---------------

async function handleTogglePlugin(state: McpManagerState, pluginId: number) {
  try {
    await togglePlugin(pluginId);
    // Refresh list to show updated status
    await loadPlugins(state);
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
    requestHostUpdate();
  }
}

async function handleUninstallPlugin(state: McpManagerState, pluginId: number) {
  if (!confirm("确定要卸载此插件吗？")) return;
  try {
    state.pluginsLoading = true;
    requestHostUpdate();
    await uninstallPlugin(pluginId);
    await loadPlugins(state);
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
    requestHostUpdate();
  } finally {
    state.pluginsLoading = false;
    requestHostUpdate();
  }
}

async function handleInstallPlugin(state: McpManagerState, pluginId: number) {
  try {
    state.pluginsLoading = true;
    requestHostUpdate();
    await installPlugin(pluginId);
    await loadPlugins(state);
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
    requestHostUpdate();
  } finally {
    state.pluginsLoading = false;
    requestHostUpdate();
  }
}

async function handleUploadPlugin(state: McpManagerState, manifest: string, binary: File) {
  try {
    state.uploading = true;
    state.pluginsError = null;
    requestHostUpdate();
    await uploadPlugin(manifest, binary);
    state.uploadOpen = false;
    await loadPlugins(state);
  } catch (e: unknown) {
    state.pluginsError = (e as Error).message;
    requestHostUpdate();
  } finally {
    state.uploading = false;
    requestHostUpdate();
  }
}

// --------------- remote config action handlers ---------------

async function handleToggleRemote(state: McpManagerState, id: number) {
  try {
    await toggleRemoteConfig(id);
    await loadRemote(state);
  } catch (e: unknown) {
    state.remoteError = (e as Error).message;
    requestHostUpdate();
  }
}

async function handleDeleteRemote(state: McpManagerState, id: number) {
  if (!confirm("确定要删除此远程 MCP 配置吗？")) return;
  try {
    state.remoteLoading = true;
    requestHostUpdate();
    await deleteRemoteConfig(id);
    await loadRemote(state);
  } catch (e: unknown) {
    state.remoteError = (e as Error).message;
    requestHostUpdate();
  } finally {
    state.remoteLoading = false;
    requestHostUpdate();
  }
}

async function handleTestRemote(state: McpManagerState, id: number) {
  state.testing.add(id);
  state.testResults.delete(id);
  requestHostUpdate();
  try {
    const result = await testRemoteConfig(id);
    state.testResults.set(id, result);
  } catch (e: unknown) {
    state.testResults.set(id, { ok: false, message: (e as Error).message });
  } finally {
    state.testing.delete(id);
    requestHostUpdate();
  }
}

async function handleSaveRemote(
  state: McpManagerState,
  id: number | null,
  form: { name: string; url: string; transport: string; auth_type: string },
) {
  try {
    state.remoteLoading = true;
    state.remoteError = null;
    requestHostUpdate();
    if (id) {
      await updateRemoteConfig(id, form);
    } else {
      await createRemoteConfig(form);
    }
    state.editOpen = false;
    state.editingId = null;
    await loadRemote(state);
  } catch (e: unknown) {
    state.remoteError = (e as Error).message;
    requestHostUpdate();
  } finally {
    state.remoteLoading = false;
    requestHostUpdate();
  }
}

// --------------- builtin tool action handlers ---------------

async function handleToggleBuiltinTool(state: McpManagerState, name: string) {
  try {
    await toggleBuiltinTool(name);
    await loadBuiltinTools(state);
  } catch (e: unknown) {
    state.builtinError = (e as Error).message;
    requestHostUpdate();
  }
}

async function handleEnableAllTools(state: McpManagerState) {
  const disabled = state.builtinTools.filter((t) => !t.enabled);
  if (disabled.length === 0) return;
  try {
    state.builtinLoading = true;
    requestHostUpdate();
    await Promise.all(disabled.map((t) => toggleBuiltinTool(t.name)));
    await loadBuiltinTools(state);
  } catch (e: unknown) {
    state.builtinError = (e as Error).message;
    requestHostUpdate();
  } finally {
    state.builtinLoading = false;
    requestHostUpdate();
  }
}

async function handleDisableAllTools(state: McpManagerState) {
  const enabled = state.builtinTools.filter((t) => t.enabled);
  if (enabled.length === 0) return;
  try {
    state.builtinLoading = true;
    requestHostUpdate();
    await Promise.all(enabled.map((t) => toggleBuiltinTool(t.name)));
    await loadBuiltinTools(state);
  } catch (e: unknown) {
    state.builtinError = (e as Error).message;
    requestHostUpdate();
  } finally {
    state.builtinLoading = false;
    requestHostUpdate();
  }
}

// --------------- utility ---------------

function formatDate(dateStr: string): string {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function statusPill(status: string): TemplateResult {
  switch (status) {
    case "active":
      return html`<span class="pill pill--ok">${status}</span>`;
    case "error":
      return html`<span class="pill pill--danger">${status}</span>`;
    case "disabled":
    default:
      return html`<span class="pill">${status}</span>`;
  }
}

function enabledPill(enabled: boolean): TemplateResult {
  return enabled
    ? html`<span class="pill pill--ok">启用</span>`
    : html`<span class="pill">禁用</span>`;
}

function transportLabel(transport: string): string {
  if (transport === "sse") return "SSE";
  if (transport === "streamable-http") return "Streamable HTTP";
  return transport;
}

function authLabel(authType: string): string {
  switch (authType) {
    case "bearer":
      return "Bearer";
    case "basic":
      return "Basic";
    case "none":
    default:
      return "无";
  }
}

// --------------- panel: plugins ---------------

function renderPluginsPanel(state: McpManagerState): TemplateResult {
  return html`
    <div class="mcp-panel">
      <div class="mcp-panel__header">
        <h3>插件市场</h3>
        <button
          class="btn btn--sm primary"
          @click=${() => {
            state.uploadOpen = true;
            requestHostUpdate();
          }}
        >
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
                  ${state.plugins.map(
                    (p) => html`
                      <tr>
                        <td>${p.display_name || p.name}</td>
                        <td>${p.version}</td>
                        <td>${statusPill(p.status)}</td>
                        <td>${formatDate(p.created_at)}</td>
                        <td>
                          <button
                            class="btn btn--sm"
                            @click=${() => handleTogglePlugin(state, p.id)}
                          >
                            ${p.status === "active" ? "禁用" : "启用"}
                          </button>
                          <button
                            class="btn btn--sm danger"
                            @click=${() => handleUninstallPlugin(state, p.id)}
                          >
                            卸载
                          </button>
                        </td>
                      </tr>
                    `,
                  )}
                </tbody>
              </table>
            `}

      ${state.uploadOpen ? renderUploadDialog(state) : nothing}
    </div>
  `;
}

function renderUploadDialog(state: McpManagerState): TemplateResult {
  let manifestText = "";
  let binaryFile: File | null = null;

  const close = () => {
    state.uploadOpen = false;
    requestHostUpdate();
  };

  const doUpload = () => {
    if (!manifestText || !binaryFile) return;
    handleUploadPlugin(state, manifestText, binaryFile);
  };

  return html`
    <div class="modal-overlay" @click=${close}>
      <div class="modal" @click=${(e: Event) => e.stopPropagation()}>
        <h4>上传插件</h4>
        <label>
          插件清单 (JSON)
          <textarea
            class="form-input"
            rows="8"
            @input=${(e: Event) => {
              manifestText = (e.target as HTMLTextAreaElement).value;
            }}
            placeholder='{"name": "my-plugin", "version": "1.0.0", ...}'
          ></textarea>
        </label>
        <label style="margin-top: 12px;">
          插件二进制文件
          <input
            class="form-input"
            type="file"
            @change=${(e: Event) => {
              const input = e.target as HTMLInputElement;
              binaryFile = input.files?.[0] ?? null;
            }}
          />
        </label>
        <div class="modal-actions">
          <button class="btn" @click=${close} ?disabled=${state.uploading}>取消</button>
          <button class="btn primary" @click=${doUpload} ?disabled=${state.uploading}>
            ${state.uploading ? "上传中..." : "上传"}
          </button>
        </div>
      </div>
    </div>
  `;
}

// --------------- panel: remote ---------------

function renderRemotePanel(state: McpManagerState): TemplateResult {
  return html`
    <div class="mcp-panel">
      <div class="mcp-panel__header">
        <h3>远程 MCP 服务器</h3>
        <button
          class="btn btn--sm primary"
          @click=${() => {
            state.editOpen = true;
            state.editingId = null;
            requestHostUpdate();
          }}
        >
          + 添加服务器
        </button>
      </div>

      ${state.remoteError
        ? html`<div class="error-banner">${state.remoteError}</div>`
        : nothing}
      ${state.remoteLoading
        ? html`<div class="loading-spinner">加载中...</div>`
        : state.remoteConfigs.length === 0
          ? html`<div class="empty-state">暂无远程 MCP 配置</div>`
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
                  ${state.remoteConfigs.map(
                    (r) => html`
                      <tr>
                        <td>${r.name}</td>
                        <td>${r.url}</td>
                        <td>${transportLabel(r.transport)}</td>
                        <td>${authLabel(r.auth_type)}</td>
                        <td>${enabledPill(r.enabled)}</td>
                        <td>
                          <button
                            class="btn btn--sm"
                            ?disabled=${state.testing.has(r.id)}
                            @click=${() => handleTestRemote(state, r.id)}
                          >
                            ${state.testing.has(r.id)
                              ? "测试中..."
                              : state.testResults.has(r.id)
                                ? state.testResults.get(r.id)!.ok
                                  ? "\u2713"
                                  : "\u2717"
                                : "测试"}
                          </button>
                          <button
                            class="btn btn--sm"
                            @click=${() => {
                              state.editOpen = true;
                              state.editingId = r.id;
                              requestHostUpdate();
                            }}
                          >
                            编辑
                          </button>
                          <button
                            class="btn btn--sm"
                            @click=${() => handleToggleRemote(state, r.id)}
                          >
                            ${r.enabled ? "禁用" : "启用"}
                          </button>
                          <button
                            class="btn btn--sm danger"
                            @click=${() => handleDeleteRemote(state, r.id)}
                          >
                            删除
                          </button>
                        </td>
                      </tr>
                    `,
                  )}
                </tbody>
              </table>
            `}

      ${state.editOpen ? renderRemoteEditDialog(state) : nothing}
    </div>
  `;
}

function renderRemoteEditDialog(state: McpManagerState): TemplateResult {
  const editing = state.editingId
    ? state.remoteConfigs.find((r) => r.id === state.editingId)
    : null;

  let name = editing?.name ?? "";
  let url = editing?.url ?? "";
  let transport = editing?.transport ?? "sse";
  let authType = editing?.auth_type ?? "none";

  const close = () => {
    state.editOpen = false;
    state.editingId = null;
    requestHostUpdate();
  };

  const save = () => {
    handleSaveRemote(state, state.editingId, { name, url, transport, auth_type: authType });
  };

  return html`
    <div class="modal-overlay" @click=${close}>
      <div class="modal" @click=${(e: Event) => e.stopPropagation()}>
        <h4>${state.editingId ? "编辑" : "添加"}远程 MCP 服务器</h4>
        <label>
          名称
          <input
            class="form-input"
            type="text"
            .value=${name}
            @input=${(e: Event) => {
              name = (e.target as HTMLInputElement).value;
            }}
          />
        </label>
        <label style="margin-top: 12px;">
          URL
          <input
            class="form-input"
            type="text"
            .value=${url}
            @input=${(e: Event) => {
              url = (e.target as HTMLInputElement).value;
            }}
          />
        </label>
        <label style="margin-top: 12px;">
          传输方式
          <select
            class="form-input"
            @change=${(e: Event) => {
              transport = (e.target as HTMLSelectElement).value;
            }}
          >
            <option value="sse" ?selected=${transport === "sse"}>SSE</option>
            <option value="streamable-http" ?selected=${transport === "streamable-http"}>
              Streamable HTTP
            </option>
          </select>
        </label>
        <label style="margin-top: 12px;">
          认证方式
          <select
            class="form-input"
            @change=${(e: Event) => {
              authType = (e.target as HTMLSelectElement).value;
            }}
          >
            <option value="none" ?selected=${authType === "none"}>无</option>
            <option value="bearer" ?selected=${authType === "bearer"}>Bearer</option>
            <option value="basic" ?selected=${authType === "basic"}>Basic</option>
          </select>
        </label>
        <div class="modal-actions">
          <button class="btn" @click=${close} ?disabled=${state.remoteLoading}>取消</button>
          <button class="btn primary" @click=${save} ?disabled=${state.remoteLoading}>
            ${state.remoteLoading ? "保存中..." : "保存"}
          </button>
        </div>
      </div>
    </div>
  `;
}

// --------------- panel: builtin tools ---------------

function renderToolsPanel(state: McpManagerState): TemplateResult {
  const enabledCount = state.builtinTools.filter((t) => t.enabled).length;
  const totalCount = state.builtinTools.length;

  // Group tools by category
  const categories = new Map<string, BuiltinTool[]>();
  for (const tool of state.builtinTools) {
    const cat = tool.category || "未分类";
    if (!categories.has(cat)) {
      categories.set(cat, []);
    }
    categories.get(cat)!.push(tool);
  }

  return html`
    <div class="mcp-panel">
      <div class="mcp-panel__header">
        <h3>内置工具 (${enabledCount}/${totalCount})</h3>
        <div>
          <button
            class="btn btn--sm primary"
            ?disabled=${state.builtinLoading || disabledCount(state, categories) === 0}
            @click=${() => handleEnableAllTools(state)}
          >
            全部启用
          </button>
          <button
            class="btn btn--sm"
            ?disabled=${state.builtinLoading || enabledCount === 0}
            @click=${() => handleDisableAllTools(state)}
          >
            全部禁用
          </button>
        </div>
      </div>

      ${state.builtinError
        ? html`<div class="error-banner">${state.builtinError}</div>`
        : nothing}
      ${state.builtinLoading
        ? html`<div class="loading-spinner">加载中...</div>`
        : state.builtinTools.length === 0
          ? html`<div class="empty-state">暂无可用的内置工具</div>`
          : html`
              <div class="mcp-tools-categories">
                ${[...categories.entries()].map(([cat, tools]) => {
                  const catEnabled = tools.filter((t) => t.enabled).length;
                  const catTotal = tools.length;
                  const expanded = state.categoriesExpanded.has(cat);
                  return html`
                    <details
                      ?open=${expanded}
                      @toggle=${(e: Event) => {
                        const details = e.target as HTMLDetailsElement;
                        if (details.open) {
                          state.categoriesExpanded.add(cat);
                        } else {
                          state.categoriesExpanded.delete(cat);
                        }
                      }}
                    >
                      <summary>${cat} (${catEnabled}/${catTotal})</summary>
                      <div class="mcp-tools-list">
                        ${tools.map(
                          (t) => html`
                            <div class="mcp-tool-row">
                              <div class="mcp-tool-row__info">
                                <div class="mcp-tool-row__name">${t.display_name || t.name}</div>
                                <div class="mcp-tool-row__desc">${t.description}</div>
                              </div>
                              <div class="mcp-tool-row__actions">
                                ${enabledPill(t.enabled)}
                                <button
                                  class="btn btn--sm"
                                  @click=${() => handleToggleBuiltinTool(state, t.name)}
                                >
                                  ${t.enabled ? "禁用" : "启用"}
                                </button>
                              </div>
                            </div>
                          `,
                        )}
                      </div>
                    </details>
                  `;
                })}
              </div>
            `}
    </div>
  `;
}

function disabledCount(
  state: McpManagerState,
  categories: Map<string, BuiltinTool[]>,
): number {
  return [...categories.values()].reduce(
    (sum, tools) => sum + tools.filter((t) => !t.enabled).length,
    0,
  );
}

// --------------- tab bar ---------------

function renderTabs(state: McpManagerState): TemplateResult {
  return html`
    <div class="mcp-manager__tabs">
      <button
        class="mcp-manager__tab ${state.activeTab === "plugins" ? "active" : ""}"
        @click=${() => {
          state.activeTab = "plugins";
          requestHostUpdate();
        }}
      >
        🔌 插件市场
      </button>
      <button
        class="mcp-manager__tab ${state.activeTab === "remote" ? "active" : ""}"
        @click=${() => {
          state.activeTab = "remote";
          requestHostUpdate();
        }}
      >
        🌐 远程服务器
      </button>
      <button
        class="mcp-manager__tab ${state.activeTab === "tools" ? "active" : ""}"
        @click=${() => {
          state.activeTab = "tools";
          requestHostUpdate();
        }}
      >
        🔧 内置工具
      </button>
    </div>
  `;
}

// --------------- main export ---------------

export function renderMcpManager(): TemplateResult {
  const state = getState();
  if (!_dataInitialized) {
    _dataInitialized = true;
    queueMicrotask(() => {
      loadPlugins(state);
      loadRemote(state);
      loadBuiltinTools(state);
    });
  }
  return html`
    <section class="mcp-manager">
      ${renderTabs(state)}
      ${state.activeTab === "plugins" ? renderPluginsPanel(state) : nothing}
      ${state.activeTab === "remote" ? renderRemotePanel(state) : nothing}
      ${state.activeTab === "tools" ? renderToolsPanel(state) : nothing}
    </section>
  `;
}
