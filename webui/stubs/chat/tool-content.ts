export function isToolCallContentType(type: unknown): boolean {
  return type === "tool_call" || type === "tool_use" || type === "toolcall" || type === "tooluse";
}

export function isToolResultContentType(type: unknown): boolean {
  return type === "tool_result" || type === "toolresult";
}

export function resolveToolUseId(item: unknown): string {
  if (!item || typeof item !== "object") return "";
  const obj = item as Record<string, unknown>;
  return (typeof obj.id === "string" && obj.id) || "";
}

export function resolveToolBlockArgs(item: unknown): Record<string, unknown> {
  if (!item || typeof item !== "object") return {};
  const obj = item as Record<string, unknown>;
  if (obj.input && typeof obj.input === "object" && !Array.isArray(obj.input)) {
    return obj.input as Record<string, unknown>;
  }
  if (obj.arguments && typeof obj.arguments === "object" && !Array.isArray(obj.arguments)) {
    return obj.arguments as Record<string, unknown>;
  }
  if (obj.args && typeof obj.args === "object" && !Array.isArray(obj.args)) {
    return obj.args as Record<string, unknown>;
  }
  return {};
}
