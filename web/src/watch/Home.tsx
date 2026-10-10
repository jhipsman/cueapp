import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchCard, type WatchHome, type WatchKind } from '../api'
import Icon from '../components/Icon'
import SetupCard from './SetupCard'
import { useDocumentTitle } from '../documentTitle'
import { epCode, playHref, Row, SpotlightContext, titleHref } from './parts'
import { onTV } from './tv'

// Home: a banner, then rows to scroll. With only set, just movies or just
// shows (the Movies and Shows tabs).
//
// On a TV it's laid out like the TV's own apps: the banner stays at the top
// and shows whatever poster the highlight is on (its artwork, name and
// story), and the rows slide up beneath it, the highlighted row first.
export default function Home({ only }: { only?: WatchKind }) {
  const [home, setHome] = useState<WatchHome | null>(null)
  const [error, setError] = useState('')
  const [spot, setSpot] = useState<WatchCard | null>(null)
  const spotTimer = useRef(0)
  const tv = onTV()
  useDocumentTitle(only === 'movie' ? 'Movies' : only === 'tv' ? 'Shows' : 'Watch')

  useEffect(() => {
    let live = true
    api
      .watchHome()
      .then((h) => live && setHome(h))
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [])

  // The first title on Continue Watching is the likeliest to be played next:
  // its stream is looked up now, so Play starts at once.
  const firstContinue = home?.rows.find((r) => r.key === 'continue')?.items[0]
  useEffect(() => {
    if (firstContinue) void api.prefetchPlay(firstContinue.kind, firstContinue.tmdbId, firstContinue.season ?? 0, firstContinue.episode ?? 0)
  }, [firstContinue])

  // The banner follows the highlight, a moment after it settles (moving
  // quickly along a row doesn't redraw it for every poster passed).
  const onSpot = useCallback((c: WatchCard) => {
    window.clearTimeout(spotTimer.current)
    spotTimer.current = window.setTimeout(() => setSpot(c), 140)
  }, [])
  useEffect(() => () => window.clearTimeout(spotTimer.current), [])

  if (error) {
    return (
      <div className="wx-page">
        <p className="wx-error">{error}</p>
      </div>
    )
  }
  if (!home) {
    return (
      <div>
        <div className="wx-hero wx-skeleton" style={{ borderRadius: 0 }} />
      </div>
    )
  }

  const rows = home.rows
    .map((r) => ({ ...r, items: only ? r.items.filter((c) => c.kind === only) : r.items }))
    .filter((r) => r.items.length > 0)
  let hero = home.hero
  if (only && hero?.kind !== only) {
    hero = rows.flatMap((r) => r.items).find((c) => c.backdropUrl && c.overview && c.progress === undefined && !c.episode)
  }
  const shown = (tv && spot) || hero

  return (
    <div className={tv ? 'wx-tvhome' : undefined}>
      {shown && <Banner card={shown} featured={shown === hero} />}
      <div className="wx-rows" style={shown ? undefined : { marginTop: 90 }}>
        {!only && <SetupCard />}
        <SpotlightContext.Provider value={tv ? onSpot : null}>
          {rows.map((r) => (
            <Row key={r.key} title={r.title} items={r.items} wide={r.key === 'continue' || r.key === 'new-episodes'} />
          ))}
        </SpotlightContext.Provider>
        {rows.length === 0 && (
          <p className="wx-dim" style={{ padding: '0 var(--wx-gutter)' }}>
            Nothing to show yet. Cue needs its movie info key: the owner can add it in Settings.
          </p>
        )}
      </div>
    </div>
  )
}

// Banner: a title big, with Play and More info. featured is Home's own
// pick; otherwise it's the poster highlighted on a TV.
function Banner({ card, featured }: { card: WatchCard; featured: boolean }) {
  const navigate = useNavigate()
  // A title being watched carries on where it was; anything else starts at
  // the beginning (the first episode of a show).
  const resumes = !featured && (card.progress !== undefined || !!card.episode)
  const play = resumes ? playHref(card.kind, card.tmdbId, card.season, card.episode) : playHref(card.kind, card.tmdbId, 1, 1)
  return (
    <header className="wx-hero" style={{ backgroundImage: card.backdropUrl ? `url(${card.backdropUrl})` : undefined }}>
      <div className="wx-hero-body" key={`${card.kind}-${card.tmdbId}-${card.episode ?? 0}`}>
        <h1>{card.title}</h1>
        <div className="wx-meta">
          {card.episode ? (
            <span>
              {epCode(card.season, card.episode)}
              {card.episodeTitle ? ` · ${card.episodeTitle}` : ''}
            </span>
          ) : null}
          {card.year ? <span>{card.year}</span> : null}
          {card.rating ? (
            <span className="wx-score">
              <span className="wx-badge-tmdb">TMDB</span> {card.rating.toFixed(1)}
            </span>
          ) : null}
          <span>{card.kind === 'tv' ? 'Series' : 'Movie'}</span>
          {card.new && <span className="wx-card-new inline">New episode</span>}
        </div>
        {card.overview && <p className="wx-overview clamp">{card.overview}</p>}
        <div className="wx-actions">
          <button className="wx-btn play" onClick={() => navigate(play)}>
            <Icon name="play" size={22} /> {resumes && (card.progress ?? 0) > 0 ? 'Resume' : 'Play'}
          </button>
          <button className="wx-btn" onClick={() => navigate(titleHref(card))}>
            <Icon name="info" size={20} /> More info
          </button>
        </div>
      </div>
    </header>
  )
}
