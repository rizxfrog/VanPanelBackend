export function coerceIdentityValue(value: unknown): string {
  return typeof value === "string" ? value : "unknown";
}
