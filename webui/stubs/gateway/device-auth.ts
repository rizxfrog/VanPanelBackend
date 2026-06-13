export function buildDeviceAuthPayload(deviceId: string, _nonce?: string): Record<string, unknown> {
  return { deviceId };
}
