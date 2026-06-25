export function mediaKindFromMime(mimeType: string | undefined): "image" | "audio" | "video" | undefined {
  if (!mimeType) {
    return undefined;
  }
  const type = mimeType.trim().toLowerCase();
  if (type.startsWith("image/")) return "image";
  if (type.startsWith("audio/")) return "audio";
  if (type.startsWith("video/")) return "video";
  return undefined;
}
