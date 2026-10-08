import { useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import type { WatchCard, WatchRatings } from '../api'
import Icon from '../components/Icon'

// Building blocks shared by the Watch pages.

export function titleHref(c: { kind: string; tmdbId: number }): string {
  return `/watch/${c.kind}/${c.tmdbId}`
}

export function playHref(kind: string, tmdbId: number, season?: number, episode?: number): string {
  return kind === 'tv' ? `/watch/play/tv/${tmdbId}/${season ?? 1}/${episode ?? 1}` : `/watch/play/movie/${tmdbId}`
}

export function epCode(season?: number, episode?: number): string {
  return `S${season ?? 0}:E${episode ?? 0}`
}

// Card is a poster, or (wide) a landscape image with a caption and a
// progress bar, as on Continue Watching.
export function Card({ card, wide }: { card: WatchCard; wide?: boolean }) {
  const navigate = useNavigate()
  const img = wide ? card.backdropUrl || card.posterUrl : card.posterUrl
  // Continue Watching plays straight away; anything else opens its page.
  const open = () =>
    navigate(card.progress !== undefined || card.episode ? playHref(card.kind, card.tmdbId, card.season, card.episode) : titleHref(card))
  return (
    <button className={`wx-card${wide ? ' wide' : ''}`} onClick={open} title={card.title}>
      {img ? <img src={img} alt={card.title} loading="lazy" /> : <div className="wx-card-fallback">{card.title}</div>}
      {wide && (
        <div className="wx-card-caption">
          {card.title}
          {card.episode ? (
            <small>
              {epCode(card.season, card.episode)} {card.episodeTitle}
            </small>
          ) : null}
        </div>
      )}
      {card.new && <span className="wx-card-new">New episode</span>}
      {card.progress !== undefined && card.progress > 0 && (
        <div className="wx-card-progress">
          <div style={{ width: `${Math.round(card.progress * 100)}%` }} />
        </div>
      )}
    </button>
  )
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
