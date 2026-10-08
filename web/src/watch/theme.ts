import type { WatchTheme } from '../api'

// Color themes: each profile picks an accent (buttons, focus, the logo), how
// much it glows, and a background. They're CSS variables Watch's styles
// use, set here; the last one used is kept on the device, so Watch opens
// in its colors straight away.

export const DEFAULT_THEME: WatchTheme = { accent: '#34d1bf', glow: 'soft', background: 'midnight' }

export const ACCENTS: { name: string; color: string }[] = [
  { name: 'Cue cyan', color: '#34d1bf' },
  { name: 'Ocean', color: '#3b82f6' },
  { name: 'Sky', color: '#38bdf8' },
  { name: 'Violet', color: '#8b5cf6' },
  { name: 'Pink', color: '#ec4899' },
  { name: 'Red', color: '#ef4444' },
  { name: 'Orange', color: '#f97316' },
  { name: 'Gold', color: '#eab308' },
  { name: 'Lime', color: '#84cc16' },
  { name: 'Emerald', color: '#10b981' },
  { name: 'Silver', color: '#cbd5e1' },
]

export const GLOWS: { value: WatchTheme['glow']; label: string }[] = [
  { value: 'off', label: 'Off' },
  { value: 'soft', label: 'Soft' },
  { value: 'strong', label: 'Strong' },
]

export const BACKGROUNDS: { value: WatchTheme['background']; label: string; hint: string }[] = [
  { value: 'midnight', label: 'Midnight', hint: 'Cue’s own dark gray' },
  { value: 'black', label: 'Black', hint: 'True black, best on OLED' },
  { value: 'slate', label: 'Slate', hint: 'Deep blue-gray' },
  { value: 'tinted', label: 'Tinted', hint: 'A hint of your color' },
]

type RGB = [number, number, number]

function rgb(hex: string): RGB {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
  const n = parseInt(m ? m[1] : DEFAULT_THEME.accent.slice(1), 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

function hex([r, g, b]: RGB): string {
  return '#' + [r, g, b].map((v) => Math.round(Math.min(255, Math.max(0, v))).toString(16).padStart(2, '0')).join('')
}

// mix is a, moved toward b by t (0 to 1).
function mix(a: RGB, b: RGB, t: number): RGB {
  return [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t]
}

function luminance([r, g, b]: RGB): number {
  const f = (v: number) => {
    v /= 255
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
  }
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)
}

// themeVars is the CSS variables for t.
export function themeVars(t: WatchTheme): Record<string, string> {
  const a = rgb(t.accent)
  const rgbList = (c: RGB) => c.map((v) => Math.round(v)).join(', ')
  const light = luminance(a) > 0.32
  const glow = { off: 0, soft: 1, strong: 1.8 }[t.glow] ?? 1
  const bgs: Record<WatchTheme['background'], [RGB, RGB]> = {
    midnight: [rgb('#101114'), rgb('#1b1d22')],
    black: [rgb('#000000'), rgb('#141416')],
    slate: [rgb('#0e1522'), rgb('#1a2333')],
    tinted: [mix(rgb('#0d0e11'), a, 0.07), mix(rgb('#191b20'), a, 0.1)],
  }
  const [bg, panel] = bgs[t.background] ?? bgs.midnight
  const ga = (x: number) => `rgba(${rgbList(a)}, ${Math.min(1, x * glow).toFixed(2)})`
  return {
    '--wx-accent': hex(a),
    '--wx-accent-rgb': rgbList(a),
    '--wx-accent-text': light ? hex(mix(a, [0, 0, 0], 0.85)) : '#ffffff',
    '--wx-accent-hi': hex(mix(a, [255, 255, 255], 0.3)),
    '--wx-accent-lo': hex(mix(a, [0, 0, 0], 0.2)),
    '--wx-bg': hex(bg),
    '--wx-bg-rgb': rgbList(bg),
    '--wx-panel': hex(panel),
    // The glows: around a focused card or button, the logo, its letters.
    '--wx-glow': glow ? `0 0 ${Math.round(24 * glow)}px ${ga(0.45)}` : '0 0 0 transparent',
    '--wx-glow-filter': glow ? `drop-shadow(0 0 ${Math.round(10 * glow)}px ${ga(0.45)})` : 'none',
    '--wx-glow-text': glow ? `0 1px ${Math.round(12 * glow)}px ${ga(0.25)}` : 'none',
  }
}

const KEY = 'cue-theme'

export function savedTheme(): WatchTheme {
  try {
    const t = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    if (t && typeof t.accent === 'string') return { ...DEFAULT_THEME, ...t }
  } catch {
    // Private window: Cue's own colors until the profile loads.
  }
  return DEFAULT_THEME
}

// applyTheme sets t's colors for the whole page.
export function applyTheme(t: WatchTheme | undefined) {
  const theme = { ...DEFAULT_THEME, ...(t ?? {}) }
  const vars = Object.entries(themeVars(theme))
    .map(([k, v]) => `${k}: ${v};`)
    .join(' ')
  let el = document.getElementById('cue-theme') as HTMLStyleElement | null
  if (!el) {
    el = document.createElement('style')
    el.id = 'cue-theme'
    document.head.appendChild(el)
  }
  // html .wx outranks the defaults on .wx in watch.css.
  el.textContent = `:root, html .wx { ${vars} }`
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', themeVars(theme)['--wx-bg'])
  try {
    localStorage.setItem(KEY, JSON.stringify(theme))
  } catch {
    // Not kept: fine.
  }
}
