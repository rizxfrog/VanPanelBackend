export type ModelAuthExpiry = { expiresAt: number };
export type ModelAuthStatusProfile = { provider: string };
export type ModelAuthStatusProvider = { name: string; authorized: boolean };
export type ModelAuthStatusResult = { providers: ModelAuthStatusProvider[] };
