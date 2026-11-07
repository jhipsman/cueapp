import { createContext, useContext, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchCard, type WatchRatings } from '../api'
import Icon from '../components/Icon'
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
  // Continue Watching plays straight away; anything else opens its page.
  const open = () =>
    navigate(card.progress !== undefined || card.episode ? playHref(card.kind, card.tmdbId, card.season, card.episode) : titleHref(card))
  const code = card.episode ? epCode(card.season, card.episode) : ''
  return (
    <button
      className={`wx-card${wide ? ' wide' : ''}`}
      onClick={open}
      title={card.title}
      onMouseEnter={() => warm(card)}
      onFocus={() => {
        warm(card)
        spotlight?.(card)
      }}
    >
      <span className="wx-card-art">
        {img ? <img src={img} alt="" loading="lazy" /> : <span className="wx-card-fallback">{card.title}</span>}
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

// Logo artwork per title, looked up once ("" is none: the name is drawn).
const logos = new Map<string, string>()

export function useLogo(card: { kind: WatchCard['kind']; tmdbId: number; logoUrl?: string } | null | undefined): string | undefined {
  const key = card ? `${card.kind}-${card.tmdbId}` : ''
  const known = card?.logoUrl
  const [, bump] = useState(0)
  useEffect(() => {
    if (!card || known !== undefined || logos.has(key)) return
    let live = true
    api
      .watchLogo(card.kind, card.tmdbId)
      .then((r) => {
        logos.set(key, r.url)
        if (live) bump((n) => n + 1)
      })
      .catch(() => logos.set(key, ''))
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, known])
  if (!card) return undefined
  return known ?? logos.get(key)
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
