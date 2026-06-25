export function resolvePendingDeviceApprovalState(): string { return "none"; }
export type DevicePairingAccessSummary = { deviceId: string; approved: boolean };
export type PendingDeviceApprovalKind = "role" | "scope";
