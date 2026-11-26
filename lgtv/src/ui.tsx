import { useEffect, useRef, useState, type ReactNode } from 'react'
import { poster, type Card } from './api'
import { focusIn } from './focus'

// A button the remote can reach: a pill (white when highlighted), or round
// with just an icon (its name read out and shown under it when highlighted).
export function Btn(p: {
  children?: ReactNode
  icon?: ReactNode
  label?: string
  primary?: boolean
  round?: boolean
  first?: boolean
  k?: string
  disabled?: boolean
  onClick: () => void
  onFocus?: () => void
}) {
  return (
    <button
      className={`btn${p.primary ? ' primary' : ''}${p.round ? ' round' : ''}`}
      data-f=""
      data-k={p.k}
      data-first={p.first ? '' : undefined}
      aria-label={p.label}
      disabled={p.disabled}
      onClick={p.onClick}
      onFocus={p.onFocus}
    >
      {p.icon}
      {p.children && <span>{p.children}</span>}
      {p.round && p.label && <em className="btn-tip">{p.label}</em>}
    </button>
  )
}

export const cardKey = (c: Card) => `${c.kind}-${c.tmdbId}`

// A poster: lifts and takes a white edge when highlighted; a bar for how far
// in on Continue watching; its name under it when there's no picture.
export function Poster({ c, onPick, onFocus, n }: { c: Card; onPick: (c: Card) => void; onFocus?: (c: Card) => void; n?: number }) {
  const [broken, setBroken] = useState(false)
  const src = poster(c.posterUrl)
  return (
    <button className={`poster${n ? ' ranked' : ''}`} data-f="" data-k={cardKey(c)} onClick={() => onPick(c)} onFocus={() => onFocus?.(c)} aria-label={c.title}>
      {n ? <b className="rank">{n}</b> : null}
      <span className="poster-art">
        {src && !broken ? <img src={src} alt="" loading="lazy" onError={() => setBroken(true)} /> : <span className="poster-name">{c.title}</span>}
        {c.progress ? (
          <i className="poster-bar">
            <i style={{ width: `${Math.round(Math.min(1, c.progress) * 100)}%` }} />
          </i>
        ) : null}
        {c.season && c.episode ? <small className="poster-ep">S{c.season}:E{c.episode}</small> : null}
        {c.new ? <small className="poster-new">New episode</small> : null}
      </span>
    </button>
  )
}

// A row of posters with its name; it slides sideways with the highlight.
export function PosterRow(p: { id: string; title: string; items: Card[]; ranked?: boolean; onPick: (c: Card) => void; onFocus?: (c: Card) => void }) {
  return (
    <section className="row" data-row="" data-g={p.id}>
      <h3>{p.title}</h3>
      <div className="row-track" data-hs="0">
        <div className="row-items">
          {p.items.map((c, i) => (
            <Poster key={cardKey(c)} c={c} n={p.ranked ? i + 1 : undefined} onPick={p.onPick} onFocus={p.onFocus} />
          ))}
        </div>
      </div>
    </section>
  )
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="spinner">
      <i />
      {label && <p>{label}</p>}
    </div>
  )
}

// useFocusOnShow puts the highlight in a screen once it's drawn (and again
// each time `when` changes).
export function useFocusOnShow(when: unknown = 0) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const t = window.setTimeout(() => focusIn(ref.current), 30)
    return () => window.clearTimeout(t)
  }, [when])
  return ref
}

export const AVATAR_COLORS: Record<string, string> = {
  teal: '#0f766e',
  red: '#dc2626',
  blue: '#2563eb',
  purple: '#7c3aed',
  orange: '#c2410c',
  green: '#15803d',
  pink: '#be185d',
  yellow: '#a16207',
}
const ICONS: Record<string, string> = {
  fox: '🦊', cat: '🐱', dog: '🐶', panda: '🐼', koala: '🐨', lion: '🦁', frog: '🐸', monkey: '🐵', unicorn: '🦄',
  octopus: '🐙', penguin: '🐧', owl: '🦉', rocket: '🚀', star: '⭐', game: '🎮', ball: '⚽', pizza: '🍕', alien: '👾',
}

// A profile's face: its colour, and its picture or first letter.
export function Avatar({ avatar, name, size }: { avatar: string; name: string; size: number }) {
  const [color, icon = ''] = (avatar || 'teal').split('/')
  return (
    <span className="avatar" style={{ width: `${size}rem`, height: `${size}rem`, background: AVATAR_COLORS[color] ?? AVATAR_COLORS.teal, fontSize: `${size * 0.45}rem` }}>
      {ICONS[icon] ?? (name.trim()[0] ?? '?').toUpperCase()}
    </span>
  )
}

// Icons, drawn as lines (they look the same on every TV).
export function Icon({ name }: { name: 'search' | 'home' | 'film' | 'tv' | 'play' | 'plus' | 'check' | 'back' | 'pause' | 'replay' | 'next' }) {
  const paths: Record<string, ReactNode> = {
    search: (
      <>
        <circle cx="11" cy="11" r="7" />
        <path d="M20 20l-3.5-3.5" />
      </>
    ),
    home: <path d="M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z" />,
    film: (
      <>
        <rect x="3" y="4" width="18" height="16" rx="2" />
        <path d="M7 4v16M17 4v16M3 9h4M3 15h4M17 9h4M17 15h4" />
      </>
    ),
    tv: (
      <>
        <rect x="3" y="7" width="18" height="13" rx="2" />
        <path d="M8 3l4 4 4-4" />
      </>
    ),
    play: <path d="M7 4l13 8-13 8z" fill="currentColor" />,
    pause: <path d="M7 4h4v16H7zM13 4h4v16h-4z" fill="currentColor" />,
    plus: <path d="M12 5v14M5 12h14" />,
    check: <path d="M5 12l5 5 9-10" />,
    back: <path d="M15 5l-7 7 7 7" />,
    replay: <path d="M4 12a8 8 0 1 0 2.5-5.8M4 4v5h5" />,
    next: <path d="M5 4l10 8-10 8zM18 4v16" />,
  }
  return (
    <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths[name]}
    </svg>
  )
}
