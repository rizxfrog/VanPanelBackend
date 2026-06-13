export function defaultTitle(_name: string): string {
  return "";
}

export function formatToolDetailText(_text: string): string {
  return "";
}

export function normalizeToolName(name: string): string {
  return name;
}

export function resolveToolVerbAndDetailForArgs(_args: unknown): { verb: string; detail: string } {
  return { verb: "", detail: "" };
}

export type ToolDisplaySpec = Record<string, unknown>;
