// Cue on a TV: the Fire TV / Google TV app shows Watch in a web view and
// adds window.CueTV. There, the remote's arrows move between buttons and
// cards (spatial navigation), OK presses, and Play hands the video to the
// app's own player, which plays every format (MKV, HEVC, Dolby, DTS).

export interface CueTVBridge {
  play(json: string): void
  changeServer(): void
}

declare global {
  interface Window {
    CueTV?: CueTVBridge
    cueTvPlayerDone?: (r: TVPlayerResult) => void
  }
}

export interface TVPlayerResult {
  reason: 'ended' | 'next' | 'error' | 'back' | 'channel-up' | 'channel-down'
  position: number // seconds
  duration: number // seconds
  message?: string
}

export function onTV(): boolean {
  return typeof window !== 'undefined' && !!window.CueTV
}

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea, [tabindex]:not([tabindex="-1"])'

function visible(el: HTMLElement): boolean {
  const r = el.getBoundingClientRect()
  if (r.width === 0 || r.height === 0) return false
  const st = getComputedStyle(el)
  return st.visibility !== 'hidden' && st.display !== 'none' && el.closest('[aria-hidden="true"]') === null
}

type Dir = 'up' | 'down' | 'left' | 'right'

// best finds the element to move to from `from` in direction dir: the
// nearest one that lies that way, preferring ones in line with it.
function best(from: HTMLElement, dir: Dir): HTMLElement | null {
  const a = from.getBoundingClientRect()
  const ax = a.left + a.width / 2
  const ay = a.top + a.height / 2
  let pick: HTMLElement | null = null
  let pickScore = Infinity
  for (const el of Array.from(document.querySelectorAll<HTMLElement>(FOCUSABLE))) {
    if (el === from || !visible(el)) continue
    const b = el.getBoundingClientRect()
    const bx = b.left + b.width / 2
    const by = b.top + b.height / 2
    let main: number
    let cross: number
    switch (dir) {
      case 'up':
        main = a.top - b.bottom
        cross = Math.abs(bx - ax)
        if (by >= ay - 1) continue
        break
      case 'down':
        main = b.top - a.bottom
        cross = Math.abs(bx - ax)
        if (by <= ay + 1) continue
        break
      case 'left':
        main = a.left - b.right
        cross = Math.abs(by - ay)
        if (bx >= ax - 1) continue
        break
      default:
        main = b.left - a.right
        cross = Math.abs(by - ay)
        if (bx <= ax + 1) continue
    }
    // Overlapping in the direction of travel counts as right next to it.
    const score = Math.max(main, 0) + cross * 2
    if (score < pickScore) {
      pickScore = score
      pick = el
    }
  }
  return pick
}

function firstFocusable(): HTMLElement | null {
  const main = document.querySelector<HTMLElement>('.wx-hero .wx-btn.play, .wx-who-tile, .wx-card, .wx-btn')
  if (main && visible(main)) return main
  return Array.from(document.querySelectorAll<HTMLElement>(FOCUSABLE)).find(visible) ?? null
}

function focus(el: HTMLElement) {
  el.focus({ preventScroll: true })
  // Inside the TV guide (its own scroll box): just bring the show into view.
  if (el.closest('.wx-guide')) {
    el.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'smooth' })
    return
  }
  // In a row of posters: the row slides so the focused one sits near the
  // left, the way TV apps do, rather than jumping to the middle.
  const row = el.closest<HTMLElement>('.wx-row-scroll, .wx-chips')
  if (row) {
    const rr = row.getBoundingClientRect()
    const er = el.getBoundingClientRect()
    const pad = 48
    if (er.left < rr.left + pad || er.right > rr.right - pad) {
      row.scrollBy({ left: er.left - rr.left - pad, behavior: 'smooth' })
    }
  }
  // The page: the top bar and the banner show the top; anything else sits a
  // third of the way down the screen.
  const r = el.getBoundingClientRect()
  if (el.closest('.wx-nav, .wx-hero')) {
    window.scrollTo({ top: 0, behavior: 'smooth' })
  } else if (r.top < 110 || r.bottom > window.innerHeight - 40) {
    window.scrollBy({ top: r.top - window.innerHeight * 0.32, behavior: 'smooth' })
  }
}

const KEYS: Record<string, Dir> = { ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right' }

// installTVNavigation turns the remote's arrows into focus moves. It does
// nothing outside the TV app.
export function installTVNavigation(): () => void {
  if (!onTV()) return () => undefined
  document.documentElement.classList.add('tv-mode')
  // TVs give a web view a 960-pixel-wide page, which makes everything huge.
  // Laid out 1280 wide (a 720p screen) and scaled to fit, Cue looks the
  // size a TV app should.
  const meta = document.querySelector<HTMLMetaElement>('meta[name="viewport"]')
  if (meta) meta.content = 'width=1280, user-scalable=no'
  const onKey = (e: KeyboardEvent) => {
    const dir = KEYS[e.key]
    if (!dir) return
    const active = document.activeElement as HTMLElement | null
    // Typing in a box: left and right move the cursor, up and down leave it.
    if (active && (active.tagName === 'INPUT' || active.tagName === 'TEXTAREA') && (dir === 'left' || dir === 'right')) return
    if (active && active.tagName === 'SELECT') return
    e.preventDefault()
    if (!active || active === document.body || !visible(active)) {
      const first = firstFocusable()
      if (first) focus(first)
      return
    }
    const next = best(active, dir)
    if (next) focus(next)
  }
  window.addEventListener('keydown', onKey, true)
  // Focus something to start from once the page has drawn.
  const t = window.setTimeout(() => {
    if (!document.activeElement || document.activeElement === document.body) {
      const first = firstFocusable()
      if (first) focus(first)
    }
  }, 600)
  return () => {
    window.removeEventListener('keydown', onKey, true)
    window.clearTimeout(t)
  }
}

// refocusSoon puts the focus on the page's main button after a page change,
// when nothing on the new page has it.
export function refocusSoon() {
  if (!onTV()) return
  window.setTimeout(() => {
    const a = document.activeElement as HTMLElement | null
    if (!a || a === document.body || !document.body.contains(a) || !visible(a)) {
      const first = firstFocusable()
      if (first) focus(first)
    }
  }, 700)
}
