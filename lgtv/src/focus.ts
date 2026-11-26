// The remote: arrows move the highlight to the nearest thing that way (in
// line with it first), OK presses it, Back goes back. Things that can take
// the highlight are marked data-f. A row (data-g="name") remembers where the
// highlight was in it, so moving down into a row lands where you left it, as
// on the TV's own apps. A row that scrolls sideways (data-hs) and a page that
// scrolls (data-vs) follow the highlight smoothly.

export type Dir = 'up' | 'down' | 'left' | 'right'

const SEL = '[data-f]'

// The LG remote's keys (and a keyboard's, for trying it on a computer).
export const KEY = {
  left: 37,
  up: 38,
  right: 39,
  down: 40,
  ok: 13,
  back: 461,
  escape: 27,
  backspace: 8,
  play: 415,
  pause: 19,
  playPause: 179,
  stop: 413,
  forward: 417,
  rewind: 412,
}
const DIRS: Record<number, Dir> = { 37: 'left', 38: 'up', 39: 'right', 40: 'down' }

function shown(el: HTMLElement): boolean {
  const r = el.getBoundingClientRect()
  if (r.width === 0 || r.height === 0) return false
  return !el.closest('[data-hidden]')
}

// The layer the highlight stays in: the last one open (a dialog over a page).
function scope(): ParentNode {
  const layers = document.querySelectorAll<HTMLElement>('[data-layer]')
  return layers.length ? layers[layers.length - 1] : document
}

function candidates(): HTMLElement[] {
  return Array.prototype.slice.call(scope().querySelectorAll(SEL)).filter(shown)
}

function best(from: HTMLElement, dir: Dir, list: HTMLElement[]): HTMLElement | null {
  const a = from.getBoundingClientRect()
  const ax = a.left + a.width / 2
  const ay = a.top + a.height / 2
  let pick: HTMLElement | null = null
  let pickScore = Infinity
  for (const el of list) {
    if (el === from) continue
    const b = el.getBoundingClientRect()
    const bx = b.left + b.width / 2
    const by = b.top + b.height / 2
    let main: number
    let cross: number
    if (dir === 'up') {
      if (by >= ay - 1) continue
      main = a.top - b.bottom
      cross = Math.abs(bx - ax)
    } else if (dir === 'down') {
      if (by <= ay + 1) continue
      main = b.top - a.bottom
      cross = Math.abs(bx - ax)
    } else if (dir === 'left') {
      if (bx >= ax - 1) continue
      main = a.left - b.right
      cross = Math.abs(by - ay)
    } else {
      if (bx <= ax + 1) continue
      main = b.left - a.right
      cross = Math.abs(by - ay)
    }
    const score = Math.max(main, 0) + cross * 2
    if (score < pickScore) {
      pickScore = score
      pick = el
    }
  }
  return pick
}

// Where the highlight was in each row.
const memory = new Map<string, string>()
const groupOf = (el: Element | null) => el?.closest('[data-g]')?.getAttribute('data-g') ?? ''
const keyOf = (el: Element) => el.getAttribute('data-k') ?? ''

function remembered(group: string, list: HTMLElement[]): HTMLElement | null {
  const k = memory.get(group)
  if (!k) return null
  return list.find((el) => groupOf(el) === group && keyOf(el) === k) ?? null
}

// ---- Smooth scrolling (the engine's own: older TVs have no smooth scroll) ----

const tweens = new WeakMap<HTMLElement, number>()
function glide(box: HTMLElement, prop: 'scrollLeft' | 'scrollTop', to: number) {
  const from = box[prop]
  to = Math.max(0, Math.round(to))
  if (Math.abs(to - from) < 2) return
  const prev = tweens.get(box)
  if (prev) cancelAnimationFrame(prev)
  const start = performance.now()
  const ms = 220
  const step = (t: number) => {
    const p = Math.min(1, (t - start) / ms)
    const e = 1 - Math.pow(1 - p, 3)
    box[prop] = from + (to - from) * e
    if (p < 1) tweens.set(box, requestAnimationFrame(step))
  }
  tweens.set(box, requestAnimationFrame(step))
}

// reveal brings el into view: its row slides so it sits near the left (or
// stays put while it's well in view), and the page so its row sits at the
// page's line (data-vs-at, from the top) or simply in view.
export function reveal(el: HTMLElement) {
  const hs = el.closest<HTMLElement>('[data-hs]')
  if (hs) {
    const pad = Number(hs.getAttribute('data-hs')) || 0
    const box = hs.getBoundingClientRect()
    const r = el.getBoundingClientRect()
    const left = r.left - box.left + hs.scrollLeft
    if (r.left < box.left + pad || r.right > box.right - pad) glide(hs, 'scrollLeft', left - pad)
  }
  const vs = el.closest<HTMLElement>('[data-vs]')
  if (vs) {
    const box = vs.getBoundingClientRect()
    const anchor = el.closest<HTMLElement>('[data-row]') ?? el
    const r = anchor.getBoundingClientRect()
    const at = vs.getAttribute('data-vs-at')
    const top = r.top - box.top + vs.scrollTop
    if (at !== null) glide(vs, 'scrollTop', top - Number(at))
    else if (r.top < box.top + 40) glide(vs, 'scrollTop', top - 40)
    else if (r.bottom > box.bottom - 40) glide(vs, 'scrollTop', top + r.height - box.height + 40)
  }
}

// ---- Moving ----

export function focus(el: HTMLElement | null | undefined) {
  if (!el) return
  el.focus()
  const g = groupOf(el)
  if (g && keyOf(el)) memory.set(g, keyOf(el))
  reveal(el)
}

export function move(dir: Dir): boolean {
  const list = candidates()
  const from = document.activeElement as HTMLElement | null
  if (!from || from === document.body || !from.matches(SEL) || !shown(from)) {
    focus(list[0])
    return !!list[0]
  }
  let next = best(from, dir, list)
  // Into another row: where the highlight was in it.
  const g = groupOf(next)
  if (next && g && g !== groupOf(from)) next = remembered(g, list) ?? next
  if (!next) return false
  focus(next)
  return true
}

// focusIn puts the highlight in a box: on what it last had there, else its
// data-first, else the first thing in it.
export function focusIn(box: Element | null): boolean {
  if (!box) return false
  const list: HTMLElement[] = Array.prototype.slice.call(box.querySelectorAll(SEL)).filter(shown)
  if (!list.length) return false
  const g = box.getAttribute('data-g')
  const el = (g && remembered(g, list)) || list.find((e) => e.hasAttribute('data-first')) || list[0]
  focus(el)
  return true
}

// ---- Keys ----

type Handler = (code: number) => boolean
const handlers: Handler[] = []

// onKey lets a screen take keys first (the newest first); true means used.
export function onKey(h: Handler): () => void {
  handlers.push(h)
  return () => {
    const i = handlers.lastIndexOf(h)
    if (i >= 0) handlers.splice(i, 1)
  }
}

export function isBack(code: number, target: EventTarget | null): boolean {
  if (code === KEY.back || code === KEY.escape) return true
  // Backspace is Back, except while typing.
  return code === KEY.backspace && !(target instanceof HTMLInputElement)
}

export function installRemote() {
  window.addEventListener(
    'keydown',
    (e) => {
      const code = e.keyCode
      for (let i = handlers.length - 1; i >= 0; i--) {
        if (handlers[i](isBack(code, e.target) ? KEY.back : code)) {
          e.preventDefault()
          return
        }
      }
      const dir = DIRS[code]
      if (!dir) return
      const t = e.target
      // Typing: left and right move the cursor.
      if (t instanceof HTMLInputElement && (dir === 'left' || dir === 'right')) return
      e.preventDefault()
      move(dir)
    },
    true,
  )
  // The pointer the LG remote has (wave it): whatever it's over takes the highlight.
  document.addEventListener('mouseover', (e) => {
    const el = (e.target as Element | null)?.closest<HTMLElement>(SEL)
    if (el && el !== document.activeElement) focus(el)
  })
}
