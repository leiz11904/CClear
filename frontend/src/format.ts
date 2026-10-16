export function formatBytes(n: number): string {
  if (n < 0) return "由系统计算";
  if (n < 1024) return `${Math.round(n)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = n;
  let unit = -1;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  if (value >= 100 || Math.abs(value - Math.trunc(value)) < 0.05) {
    return `${value.toFixed(0)} ${units[unit]}`;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}

export function formatPercent(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0%";
  if (n < 0.1) return "<0.1%";
  if (n >= 10) return `${n.toFixed(0)}%`;
  return `${n.toFixed(1)}%`;
}

// Eight steps from the four directory hues. Index i and i+4 share a hue
// but not a shade, so the first eight siblings stay distinguishable.
// The same index is used in the rank, bubbles, and treemap.
export const lightPalette = [
  "#3178E8", "#35A99B", "#8877C8", "#DBA258",
  "#1C5FBE", "#1C7C72", "#5C4C96", "#A56E28",
];
export const darkPalette = [
  "#6B9BF5", "#5FCBBC", "#B0A4E4", "#E8C48A",
  "#9BBDFA", "#8EDDD2", "#C8BFF0", "#F0D7B0",
];

export function paletteFor(theme: "light" | "dark"): string[] {
  return theme === "dark" ? darkPalette : lightPalette;
}

function channel(hex: string, shift: number): number {
  const n = parseInt(hex.slice(1 + shift, 3 + shift), 16) / 255;
  return n <= 0.03928 ? n / 12.92 : ((n + 0.055) / 1.055) ** 2.4;
}

function luminance(hex: string): number {
  return 0.2126 * channel(hex, 0) + 0.7152 * channel(hex, 2) + 0.0722 * channel(hex, 4);
}

function contrast(a: number, b: number): number {
  const lighter = Math.max(a, b);
  const darker = Math.min(a, b);
  return (lighter + 0.05) / (darker + 0.05);
}

export function inkOn(fill: string): string {
  const base = luminance(fill);
  const dark = "#202B3B";
  const light = "#FFFFFF";
  return contrast(base, luminance(dark)) >= contrast(base, luminance(light)) ? dark : light;
}
