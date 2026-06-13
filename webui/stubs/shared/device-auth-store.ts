export type DeviceAuthEntry = {
  deviceId: string;
  token: string;
};

export function clearDeviceAuthTokenFromStore(_store: unknown): void {}

export function loadDeviceAuthTokenFromStore(_store: unknown): DeviceAuthEntry | null {
  return null;
}

export function storeDeviceAuthTokenInStore(_store: unknown, _entry: DeviceAuthEntry): void {}
