// MCP Manager Controller — manages plugin, remote config, and builtin tool state
// via the backend /api/system/agent endpoints.

// --------------- private helpers ---------------

const API_BASE = "/api/system/agent";

interface ApiResponse<T> {
  code: number;
  data: T;
  message: string;
}

// Backend uses `json:"items"` for paginated lists (model.ListResp)
interface ListResponse<T> {
  items: T[];
  total: number;
}

// Backend returns this for test-connect results
interface TestRemoteMCPResult {
  reachable: boolean;
  tools: string[];
  error: string;
}

async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { credentials: "same-origin" });
  if (!res.ok) {
    throw new Error(`GET ${path} failed: ${res.status} ${res.statusText}`);
  }
  const body: ApiResponse<T> = await res.json();
  if (body.code !== 0) {
    throw new Error(body.message || `GET ${path} returned code ${body.code}`);
  }
  return body.data;
}

async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: body != null ? { "Content-Type": "application/json" } : undefined,
    body: body != null ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });
  if (!res.ok) {
    throw new Error(`POST ${path} failed: ${res.status} ${res.statusText}`);
  }
  const json: ApiResponse<T> = await res.json();
  if (json.code !== 0) {
    throw new Error(json.message || `POST ${path} returned code ${json.code}`);
  }
  return json.data;
}

async function apiPut(path: string, body?: unknown): Promise<void> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "PUT",
    headers: body != null ? { "Content-Type": "application/json" } : undefined,
    body: body != null ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });
  if (!res.ok) {
    throw new Error(`PUT ${path} failed: ${res.status} ${res.statusText}`);
  }
  const json: ApiResponse<unknown> = await res.json();
  if (json.code !== 0) {
    throw new Error(json.message || `PUT ${path} returned code ${json.code}`);
  }
}

async function apiDelete(path: string): Promise<void> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "DELETE",
    credentials: "same-origin",
  });
  if (!res.ok) {
    throw new Error(`DELETE ${path} failed: ${res.status} ${res.statusText}`);
  }
  const json: ApiResponse<unknown> = await res.json();
  if (json.code !== 0) {
    throw new Error(json.message || `DELETE ${path} returned code ${json.code}`);
  }
}

async function apiPostFormData<T>(path: string, formData: FormData): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    body: formData,
    credentials: "same-origin",
  });
  if (!res.ok) {
    throw new Error(`POST ${path} failed: ${res.status} ${res.statusText}`);
  }
  const json: ApiResponse<T> = await res.json();
  if (json.code !== 0) {
    throw new Error(json.message || `POST ${path} returned code ${json.code}`);
  }
  return json.data;
}

// --------------- types ---------------

export interface McpPlugin {
  id: number;
  name: string;
  display_name: string;
  description: string;
  version: string;
  author: string;
  category: string;
  icon_url: string;
  status: string; // "active" | "disabled" | "error"
  binary_path: string;
  created_at: string;
  updated_at: string;
}

export interface RemoteMcpConfig {
  id: number;
  user_id: number;
  name: string;
  description: string;
  transport: string; // "sse" | "streamable-http"
  url: string;
  auth_type: string; // "none" | "bearer" | "basic"
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

// --------------- plugin operations ---------------

export async function listPlugins(): Promise<McpPlugin[]> {
  const data = await apiGet<ListResponse<McpPlugin>>("/hub/plugins/list");
  return data.items ?? [];
}

export async function uploadPlugin(manifest: string, binary: File): Promise<McpPlugin> {
  const formData = new FormData();
  formData.append("manifest", manifest);
  formData.append("binary", binary);
  return apiPostFormData<McpPlugin>("/hub/plugins/upload", formData);
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

// --------------- remote MCP config operations ---------------

export async function listRemoteConfigs(): Promise<RemoteMcpConfig[]> {
  const data = await apiGet<ListResponse<RemoteMcpConfig>>("/remote-mcps/list");
  return data.items ?? [];
}

export async function createRemoteConfig(config: Partial<RemoteMcpConfig>): Promise<void> {
  await apiPost("/remote-mcps/create", config);
}

export async function updateRemoteConfig(id: number, config: Partial<RemoteMcpConfig>): Promise<void> {
  await apiPut(`/remote-mcps/${id}/update`, config);
}

export async function deleteRemoteConfig(id: number): Promise<void> {
  await apiDelete(`/remote-mcps/${id}/delete`);
}

export async function toggleRemoteConfig(id: number): Promise<void> {
  await apiPut(`/remote-mcps/${id}/toggle`);
}

export async function testRemoteConfig(id: number): Promise<{ ok: boolean; message: string }> {
  const result = await apiPost<TestRemoteMCPResult>(`/remote-mcps/${id}/test`);
  if (!result.reachable) {
    return { ok: false, message: result.error || "Connection failed" };
  }
  return {
    ok: true,
    message: `Connected, ${result.tools?.length ?? 0} tools available`,
  };
}

// --------------- builtin tool operations ---------------

// Backend returns raw []*BuiltinTool (no ListResp wrapper)
export async function listBuiltinTools(): Promise<BuiltinTool[]> {
  const data = await apiGet<BuiltinTool[]>("/builtin-tools/list");
  return Array.isArray(data) ? data : [];
}

export async function toggleBuiltinTool(name: string): Promise<void> {
  await apiPut(`/builtin-tools/${encodeURIComponent(name)}/toggle`);
}
