import { useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { api, type WatchCard } from '../api'
import { epCode, TitleMark, useExtras, useTint } from './parts'

// Screensaver: left alone on Watch for a few minutes, Cue drifts through the
// household's own titles full screen, each picture slowly moving, with its
// logo, a line about it (a new episode, where you stopped, what's on your
// list) and a clock. Any key, click or move brings Watch back where it was.
// Never while something plays, and not on phones (they lock themselves).

const IDLE_MS = 3 * 60_000
const SLIDE_MS = 9000

interface Slide {
  card: WatchCard
  line: string
  detail: string
}

// Yours (where you stopped, new episodes, your list) and ones you haven't
// watched (trending, "because you watched", the rest of Home), taken in
// turn, a new one first; the new ones shuffled each time.
function slidesFrom(rows: { key: string; title: string; items: WatchCard[] }[]): Slide[] {
  const mine: Slide[] = []
  const fresh: Slide[] = []
  const seen = new Set<string>()
  const add = (to: Slide[], c: WatchCard, line: string, detail: string) => {
    const k = `${c.kind}-${c.tmdbId}`
    if (!c.backdropUrl || seen.has(k)) return
    seen.add(k)
    to.push({ card: c, line, detail })
  }
  const yours = new Set(['continue', 'new-episodes', 'my-list'])
  const facts = (c: WatchCard) => [c.year, c.kind === 'tv' ? 'Series' : 'Movie'].filter(Boolean).join(' · ')
  for (const r of rows.filter((r) => yours.has(r.key))) {
    for (const c of r.items.slice(0, 8)) {
      if (r.key === 'new-episodes') add(mine, c, 'New episode', [epCode(c.season, c.episode), c.episodeTitle].filter(Boolean).join(' · '))
      else if (r.key === 'continue')
        add(mine, c, 'You stopped here', c.episode ? [epCode(c.season, c.episode), c.episodeTitle].filter(Boolean).join(' · ') : `${Math.round((c.progress ?? 0) * 100)}% watched`)
      else add(mine, c, 'On your list', facts(c))
    }
  }
  for (const r of rows.filter((r) => !yours.has(r.key))) {
    for (const c of r.items.slice(0, 10)) add(fresh, c, r.key.startsWith('trending') ? 'Trending this week' : r.title, facts(c))
  }
  for (let i = fresh.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[fresh[i], fresh[j]] = [fresh[j], fresh[i]]
  }
  const out: Slide[] = []
  for (let i = 0; out.length < 20 && (i < fresh.length || i < mine.length); i++) {
    if (i < fresh.length) out.push(fresh[i])
    if (i < mine.length && out.length < 20) out.push(mine[i])
  }
  return out
}

function allowedHere(path: string): boolean {
  if (/\/(play|recording)\//.test(path) || path.includes('/live/play/')) return false
  return matchMedia('(hover: hover)').matches
}

export default function Screensaver() {
  const { pathname } = useLocation()
  const [on, setOn] = useState(false)
  const [slides, setSlides] = useState<Slide[]>([])
  const [index, setIndex] = useState(0)
  const timer = useRef(0)

  // Watching for the household to leave the screen alone.
  useEffect(() => {
    if (!allowedHere(pathname)) return
    const arm = () => {
      window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => {
        if (document.hidden || document.fullscreenElement) return arm()
        api
          .watchHome()
          .then((h) => {
            const s = slidesFrom(h.rows)
            if (s.length < 2) return
            setSlides(s)
            setIndex(0)
            setOn(true)
          })
          .catch(() => undefined)
      }, IDLE_MS)
    }
    const events = ['pointermove', 'pointerdown', 'keydown', 'wheel', 'touchstart', 'scroll'] as const
    events.forEach((e) => window.addEventListener(e, arm, { passive: true }))
    arm()
    return () => {
      window.clearTimeout(timer.current)
      events.forEach((e) => window.removeEventListener(e, arm))
    }
  }, [pathname])

  // Any key, click or move ends it (a short grace period, so the move that
  // was already happening as it started doesn't end it at once).
  useEffect(() => {
    if (!on) return
    const started = Date.now()
    const wake = () => {
      if (Date.now() - started > 800) setOn(false)
    }
    const events = ['pointermove', 'pointerdown', 'keydown', 'wheel', 'touchstart'] as const
    events.forEach((e) => window.addEventListener(e, wake, { passive: true }))
    const next = window.setInterval(() => setIndex((i) => i + 1), SLIDE_MS)
    return () => {
      events.forEach((e) => window.removeEventListener(e, wake))
      window.clearInterval(next)
    }
  }, [on])

  if (!on || slides.length === 0) return null
  return <Show slides={slides} index={index} />
}

function Show({ slides, index }: { slides: Slide[]; index: number }) {
  const at = index % slides.length
  const now = slides[at]
  const ex = useExtras(now.card)
  useTint(ex?.colour)
  // The next picture loads while this one shows.
  const after = slides[(at + 1) % slides.length]
  const [clock, setClock] = useState(() => new Date())
  useEffect(() => {
    const t = window.setInterval(() => setClock(new Date()), 15_000)
    return () => window.clearInterval(t)
  }, [])
  return (
    <div className="wx-saver" role="dialog" aria-label="Screensaver: press any key to come back">
      {slides.map((s, i) => (
        <div
          key={`${s.card.kind}-${s.card.tmdbId}`}
          className={`wx-saver-pic${i === at ? ' show' : ''}`}
          style={{ backgroundImage: i === at || s === after ? `url(${s.card.backdropUrl})` : undefined }}
        />
      ))}
      <div className="wx-saver-shade" />
      <div className="wx-saver-clock">
        <b>{clock.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}</b>
        <span>{clock.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })}</span>
      </div>
      <div className="wx-saver-caption" key={at}>
        <TitleMark title={now.card.title} logo={ex === undefined ? undefined : (ex?.logo ?? '')} />
        <p>
          <i />
          <b>{now.line}</b>
          {now.detail && <span>{now.detail}</span>}
        </p>
      </div>
      <p className="wx-saver-hint">Move the mouse or press a key to come back</p>
    </div>
  )
}
