import { createContext, useContext, useEffect, useRef, useState, type PointerEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchCard, type WatchExtras, type WatchRatings } from '../api'
import Icon from '../components/Icon'
import { CardPreview } from './living'
import { onTV } from './tv'

// Building blocks shared by the Watch pages.

export function titleHref(c: { kind: string; tmdbId: number }): string {
  return `/watch/${c.kind}/${c.tmdbId}`
}

export function playHref(kind: string, tmdbId: number, season?: number, episode?: number): string {
  return kind === 'tv' ? `/watch/play/tv/${tmdbId}/${season ?? 1}/${episode ?? 1}` : `/watch/play/movie/${tmdbId}`
}

// warmed: titles whose stream was looked up ahead already (this visit).
const warmed = new Set<string>()

// warm looks a Continue Watching card's stream up when it's pointed at
// (hovered, or the TV's highlight lands on it), so Play starts at once.
function warm(card: WatchCard) {
  const plays = card.progress !== undefined || !!card.episode
  if (!plays) return
  const key = `${card.kind}:${card.tmdbId}:${card.season ?? 0}:${card.episode ?? 0}`
  if (warmed.has(key)) return
  warmed.add(key)
  void api.prefetchPlay(card.kind, card.tmdbId, card.season ?? 0, card.episode ?? 0).catch(() => undefined)
}

export function epCode(season?: number, episode?: number): string {
  return `S${season ?? 0}:E${episode ?? 0}`
}

// SpotlightContext: on the TV's Home, a card the highlight lands on is
// shown big in the banner (see Home).
export const SpotlightContext = createContext<((c: WatchCard) => void) | null>(null)

// Card is a poster, or (wide) a landscape picture with the episode's code
// on it, as on Continue Watching; its name sits underneath, as in the TV
// apps. Highlighted, the picture lifts with a shadow and a light sweeps
// across it once (watch.css).
export function Card({ card, wide }: { card: WatchCard; wide?: boolean }) {
  const navigate = useNavigate()
  const spotlight = useContext(SpotlightContext)
  let img = wide ? card.backdropUrl || card.posterUrl : card.posterUrl
  // A TV's posters are small: a smaller picture loads and draws faster.
  if (img && onTV()) img = img.replace('/t/p/w500/', '/t/p/w342/').replace('/t/p/w1280/', '/t/p/w780/')
  // A card opens its page, Continue Watching and new episodes too: Resume
  // (or Play) is there, with the episodes, so nothing starts by surprise.
  const open = () => navigate(titleHref(card))
  const code = card.episode ? epCode(card.season, card.episode) : ''
  // Pointed at, a wide card's logo floats in front of its picture, and
  // once the mouse rests a moment the card plays the title's trailer.
  const [hot, setHot] = useState(false)
  const [resting, setResting] = useState(false)
  const restTimer = useRef(0)
  const ex = useExtras(hot ? card : null)
  useEffect(() => () => window.clearTimeout(restTimer.current), [])
  // The picture tilts toward the pointer, its glare following it (a mouse
  // only: touch screens have no pointer resting on a card).
  const tilt = (e: PointerEvent<HTMLButtonElement>) => {
    if (e.pointerType !== 'mouse') return
    const r = e.currentTarget.querySelector('.wx-card-art')?.getBoundingClientRect()
    if (!r) return
    const x = (e.clientX - r.left) / r.width - 0.5
    const y = (e.clientY - r.top) / r.height - 0.5
    e.currentTarget.style.setProperty('--px', x.toFixed(3))
    e.currentTarget.style.setProperty('--py', y.toFixed(3))
  }
  return (
    <button
      className={`wx-card${wide ? ' wide' : ''}`}
      onClick={open}
      title={card.title}
      onMouseEnter={() => {
        warm(card)
        setHot(true)
        window.clearTimeout(restTimer.current)
        restTimer.current = window.setTimeout(() => setResting(true), 1300)
      }}
      onPointerMove={tilt}
      onPointerLeave={(e) => {
        e.currentTarget.style.removeProperty('--px')
        e.currentTarget.style.removeProperty('--py')
        window.clearTimeout(restTimer.current)
        setResting(false)
      }}
      onFocus={() => {
        warm(card)
        setHot(true)
        spotlight?.(card)
      }}
    >
      <span className="wx-card-art">
        {img ? <img src={img} alt="" loading="lazy" /> : <span className="wx-card-fallback">{card.title}</span>}
        {resting && wide && <CardPreview of={card} />}
        {wide && ex?.logo && <img className="wx-card-logo" src={ex.logo} alt="" />}
        {wide && code && <span className="wx-chip">{code}</span>}
        {card.new && <span className="wx-card-new">New episode</span>}
        {card.progress !== undefined && card.progress > 0 && (
          <span className="wx-card-progress">
            <span style={{ width: `${Math.round(card.progress * 100)}%` }} />
          </span>
        )}
      </span>
      <span className="wx-card-name">{card.title}</span>
      {wide && card.episodeTitle && <span className="wx-card-sub">{card.episodeTitle}</span>}
    </button>
  )
}

// Ambient is a picture drawn huge, blurred and dark behind a page, so the
// page takes on its colours (as Apple's TV app does). It cross-fades when
// the picture changes.
export function Ambient({ url }: { url: string }) {
  const small = url.replace(/\/t\/p\/w\d+\//, '/t/p/w300/')
  return (
    <div className="wx-ambient" aria-hidden="true">
      <div key={small} style={{ backgroundImage: `url(${small})` }} />
    </div>
  )
}

// A title's logo, colour and trailer (WatchExtras), looked up once per title
// and shared by every page.
const extrasCache = new Map<string, WatchExtras | null>()
const extrasWaiting = new Map<string, Promise<WatchExtras | null>>()

function loadExtras(kind: WatchCard['kind'], tmdbId: number): Promise<WatchExtras | null> {
  const key = `${kind}-${tmdbId}`
  const had = extrasWaiting.get(key)
  if (had) return had
  const p = api
    .watchExtras(kind, tmdbId)
    .then((x) => x)
    .catch(() => null)
    .then((x) => {
      extrasCache.set(key, x)
      return x
    })
  extrasWaiting.set(key, p)
  return p
}

// useExtras gives a title's extras once they're known (undefined until
// then, null when they couldn't be had). Pass null to ask for nothing.
export function useExtras(card: { kind: WatchCard['kind']; tmdbId: number } | null | undefined): WatchExtras | null | undefined {
  const key = card ? `${card.kind}-${card.tmdbId}` : ''
  const [, bump] = useState(0)
  useEffect(() => {
    if (!card || extrasCache.has(key)) return
    let live = true
    void loadExtras(card.kind, card.tmdbId).then(() => live && bump((n) => n + 1))
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])
  return card ? extrasCache.get(key) : undefined
}

export function useLogo(card: { kind: WatchCard['kind']; tmdbId: number; logoUrl?: string } | null | undefined): string | undefined {
  const known = card?.logoUrl
  const ex = useExtras(known !== undefined ? null : card)
  if (!card) return undefined
  if (known !== undefined) return known
  return ex === undefined ? undefined : (ex?.logo ?? '')
}

// useTint colours Watch after a picture: the glow behind the page, the
// highlight's halo and the progress bars take its main colour.
export function useTint(colour: string | undefined) {
  useEffect(() => {
    if (!colour) return
    const root = document.querySelector<HTMLElement>('.wx')
    root?.style.setProperty('--wx-tint', colour)
  }, [colour])
}

// TitleMark is a title's name as its logo artwork, or in big letters when
// it has none. Until the lookup answers, a quiet space of the same height,
// so the name doesn't flash before the logo replaces it.
export function TitleMark({ title, logo }: { title: string; logo: string | undefined }) {
  if (logo === undefined) return <div className="wx-logo-wait" aria-label={title} />
  if (logo) return <img className="wx-logo" src={logo} alt={title} />
  return <h1>{title}</h1>
}

// Row is a titled strip that scrolls sideways, with arrows on hover.
export function Row({ title, items, wide }: { title: string; items: WatchCard[]; wide?: boolean }) {
  const strip = useRef<HTMLDivElement>(null)
  const scroll = (dir: number) => {
    const el = strip.current
    if (el) el.scrollBy({ left: dir * el.clientWidth * 0.85, behavior: 'smooth' })
  }
  if (items.length === 0) return null
  return (
    <section className="wx-row">
      <h2>{title}</h2>
      <div className="wx-row-strip">
        <button className="wx-row-arrow left" onClick={() => scroll(-1)} aria-label={`Scroll ${title} left`}>
          ‹
        </button>
        <div className="wx-row-scroll" ref={strip}>
          {items.map((c) => (
            <Card key={`${c.kind}-${c.tmdbId}-${c.season ?? 0}-${c.episode ?? 0}`} card={c} wide={wide} />
          ))}
        </div>
        <button className="wx-row-arrow right" onClick={() => scroll(1)} aria-label={`Scroll ${title} right`}>
          ›
        </button>
      </div>
    </section>
  )
}

// Scores is the line of ratings: IMDb, Rotten Tomatoes and TMDB.
export function Scores({ ratings, tmdb }: { ratings?: WatchRatings; tmdb?: number }) {
  return (
    <>
      {ratings?.imdb && (
        <span className="wx-score" title={ratings.imdbVotes ? `${ratings.imdbVotes} votes on IMDb` : 'IMDb'}>
          <span className="wx-badge-imdb">IMDb</span> {ratings.imdb}
        </span>
      )}
      {ratings?.rottenTomatoes && (
        <span className="wx-score" title="Rotten Tomatoes critics">
          <span className="wx-badge-rt">RT</span> {ratings.rottenTomatoes}
        </span>
      )}
      {ratings?.metacritic && (
        <span className="wx-score" title="Metacritic critics">
          <span className="wx-badge-mc">MC</span> {ratings.metacritic}
        </span>
      )}
      {!ratings?.imdb && tmdb ? (
        <span className="wx-score" title="TMDB users' score">
          <span className="wx-badge-tmdb">TMDB</span> {tmdb.toFixed(1)}
        </span>
      ) : null}
    </>
  )
}

export function minutes(n?: number): string {
  if (!n) return ''
  const h = Math.floor(n / 60)
  const m = n % 60
  return h ? `${h}h ${m}m` : `${m}m`
}

export function Spinner({ label }: { label: string }) {
  return (
    <div className="wx-player-center">
      <div className="wx-spinner" />
      <div>{label}</div>
    </div>
  )
}

export function PlayIcon() {
  return <Icon name="play" size={20} />
}
