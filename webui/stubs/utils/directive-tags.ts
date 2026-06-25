export function parseInlineDirectives(text: string, _opts?: {
  stripAudioTag?: boolean;
  stripReplyTags?: boolean;
}): {
  text: string;
  audioAsVoice?: boolean;
  replyToExplicitId?: string;
  replyToCurrent?: boolean;
} {
  return { text };
}
