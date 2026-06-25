export const GATEWAY_EVENT_UPDATE_AVAILABLE = "update-available";
export type GatewayUpdateAvailableEventPayload = {
  version: string;
  downloadUrl?: string;
};
