/** Relative luminance of an opaque six-digit sRGB surface/text color. */
function luminance(color: string): number {
  const rgb = Number.parseInt(color.slice(1), 16);
  const channels = [rgb >>> 16, (rgb >>> 8) & 255, rgb & 255].map((channel) => {
    const value = channel / 255;
    return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4;
  });
  return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722;
}

function contrast(foreground: string, surface: number): number {
  const value = luminance(foreground);
  return (Math.max(value, surface) + .05) / (Math.min(value, surface) + .05);
}

/** Modern-only readability adaptation; never changes the packet or Classic.
 * Retain the chosen hue by mixing toward white/black only as far as needed.
 * Replacing every low-contrast choice with the default erased eight of the
 * server's nine colors on the dark outgoing bubble.
 */
export function readableMessageColor(value: number | undefined, fallback: string, background: string): string {
  if (value === undefined || !Number.isInteger(value) || value < 0 || value > 0xffffff
    || !/^#[\da-f]{6}$/i.test(background)) return fallback;
  const candidate = `#${value.toString(16).padStart(6, '0')}`;
  const surface = luminance(background);
  if (contrast(candidate, surface) >= 4.5) return candidate;
  const channels = [value >>> 16, (value >>> 8) & 255, value & 255];
  let best: { color: string; amount: number } | undefined;
  for (const endpoint of [0, 255]) {
    const mix = (amount: number) => '#' + channels.map((channel) =>
      Math.round(channel + (endpoint - channel) * amount).toString(16).padStart(2, '0')).join('');
    let color = mix(1);
    if (contrast(color, surface) < 4.5) continue;
    let low = 0, high = 1;
    // Test the quantized RGB itself, so rounding cannot lower the returned
    // color below the normal-text threshold.
    for (let step = 0; step < 24; step++) {
      const amount = (low + high) / 2;
      const next = mix(amount);
      if (contrast(next, surface) >= 4.5) { high = amount; color = next; }
      else low = amount;
    }
    if (!best || high < best.amount) best = { color, amount: high };
  }
  return best?.color ?? fallback;
}
