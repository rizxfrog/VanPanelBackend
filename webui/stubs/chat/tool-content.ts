export function isToolCallContentType(type: unknown): boolean {
  return type === "tool_call";
}

export function isToolResultContentType(type: unknown): boolean {
  return type === "tool_result";
}

export function resolveToolUseId(item: unknown): string {
  return "";
}

export function resolveToolBlockArgs(item: unknown): Record<string, unknown> {
  return {};
}
