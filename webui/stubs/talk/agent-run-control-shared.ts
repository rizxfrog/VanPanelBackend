export const REALTIME_VOICE_AGENT_CONSULT_TOOL_NAME = "realtime_voice_agent_consult";

export function buildRealtimeVoiceAgentCancelProviderResult(): Record<string, unknown> {
  return {};
}

export function buildRealtimeVoiceAgentControlSpeechMessage(_text: string): Record<string, unknown> {
  return {};
}

export function parseRealtimeVoiceAgentControlToolArgs(_args: unknown): Record<string, unknown> {
  return {};
}

export const REALTIME_VOICE_AGENT_CONTROL_TOOL_NAME = "realtime_voice_agent_control";

export function shouldAutoControlRealtimeVoiceAgentText(_text: string): boolean {
  return false;
}

export type RealtimeVoiceAgentControlMode = "auto" | "manual";
