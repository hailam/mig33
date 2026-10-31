/** Relative luminance of an opaque six-digit sRGB surface/text color. */
function luminance(color: string): number {
  const rgb = Number.parseInt(color.slice(1), 16);
  const channels = [rgb >>> 16, (rgb >>> 8) & 255, rgb & 255].map((channel) => {
    const value = channel / 255;
    return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4;
  });
  return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722;
}

/** Modern-only readability adaptation; never changes the packet or Classic. */
export function readableMessageColor(value: number | undefined, fallback: string, background: string): string {
  if (value === undefined || !Number.isFinite(value) || value < 0) return fallback;
  const candidate = `#${(value & 0xffffff).toString(16).padStart(6, '0')}`;
  const foreground = luminance(candidate), surface = luminance(background);
  const contrast = (Math.max(foreground, surface) + .05) / (Math.min(foreground, surface) + .05);
  return contrast >= 4.5 ? candidate : fallback;
}
