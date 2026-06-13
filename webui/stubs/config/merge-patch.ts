export function applyMergePatch<T extends Record<string, unknown>>(target: T, patch: Partial<T>): T {
  return { ...target, ...patch };
}
