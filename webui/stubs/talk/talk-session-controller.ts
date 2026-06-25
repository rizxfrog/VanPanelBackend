export function normalizeTalkTransport(transport: unknown): unknown {
  return transport ?? "websocket";
}
