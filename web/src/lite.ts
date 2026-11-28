// Light mode, as on the Fire TV app: for slow devices (a TV's own browser,
// Cue's LG app, a cheap phone or Chromebook). No blur, no endless motion,
// short transitions; the page looks the same, it just draws far less. It's
// this device's own setting (kept in the browser, not the account): on by
// itself on a TV's browser or a device with little memory, and switched
// either way in Watch › Settings.

export type LiteChoice = 'auto' | 'on' | 'off'

const KEY = 'cue-lite'

export function liteChoice(): LiteChoice {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'on' || v === 'off' ? v : 'auto'
  } catch {
    return 'auto'
  }
}

// liteAuto: what Automatic picks here. A TV's browser (LG webOS, Samsung
// Tizen, a Fire TV or Android TV web view, Cue's LG app), or a device that
// reports 2 GB of memory or less, or two processor cores or fewer.
export function liteAuto(): boolean {
  const ua = navigator.userAgent
  let lgApp = false
  try {
    lgApp = sessionStorage.getItem('cue-device') === 'webos' || new URLSearchParams(location.search).get('device') === 'webos'
  } catch {
    // No storage: the browser's name says it.
  }
  const tv = lgApp || /Web0S|webOS|Tizen|SMART-TV|SmartTV|HbbTV|BRAVIA|AFT[A-Z]|Android TV|GoogleTV|CrKey/i.test(ua)
  const memory = (navigator as Navigator & { deviceMemory?: number }).deviceMemory
  const cores = navigator.hardwareConcurrency
  return tv || (memory !== undefined && memory <= 2) || (cores !== undefined && cores > 0 && cores <= 2)
}

export function liteOn(): boolean {
  const c = liteChoice()
  return c === 'on' || (c === 'auto' && liteAuto())
}

// applyLite marks the page (html.lite) for the styles that draw less.
export function applyLite(): void {
  document.documentElement.classList.toggle('lite', liteOn())
}

export function setLite(c: LiteChoice): void {
  try {
    if (c === 'auto') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, c)
  } catch {
    // Private window: it lasts for this visit.
  }
  applyLite()
}
