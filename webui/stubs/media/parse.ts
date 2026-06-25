export function splitMediaFromOutput(text: string): {
  text: string;
  segments?: Array<{ type: string; text?: string; url?: string }>;
  audioAsVoice?: boolean;
  mediaUrls?: string[];
} {
  return { text };
}
