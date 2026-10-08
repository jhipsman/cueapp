import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchHome, type WatchKind } from '../api'
import Icon from '../components/Icon'
import SetupCard from './SetupCard'
import { useDocumentTitle } from '../documentTitle'
import { playHref, Row, titleHref } from './parts'

// Home: a banner, then rows to scroll. With only set, just movies or just
// shows (the Movies and Shows tabs).
export default function Home({ only }: { only?: WatchKind }) {
  const navigate = useNavigate()
  const [home, setHome] = useState<WatchHome | null>(null)
  const [error, setError] = useState('')
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

  if (error) {
    return (
      <div className="wx-page">
        <p className="wx-error">{error}</p>
      </div>
    )
  }
  // The first title on Continue Watching is the likeliest to be played next:
  // its stream is looked up now, so Play starts at once.
  const firstContinue = home?.rows.find((r) => r.key === 'continue')?.items[0]
  useEffect(() => {
    if (firstContinue) void api.prefetchPlay(firstContinue.kind, firstContinue.tmdbId, firstContinue.season ?? 0, firstContinue.episode ?? 0)
  }, [firstContinue])

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

  return (
    <div>
      {hero && (
        <header className="wx-hero" style={{ backgroundImage: `url(${hero.backdropUrl})` }}>
          <div className="wx-hero-body">
            <h1>{hero.title}</h1>
            <div className="wx-meta">
              {hero.year ? <span>{hero.year}</span> : null}
              {hero.rating ? (
                <span className="wx-score">
                  <span className="wx-badge-tmdb">TMDB</span> {hero.rating.toFixed(1)}
                </span>
              ) : null}
              <span>{hero.kind === 'tv' ? 'Series' : 'Movie'}</span>
            </div>
            {hero.overview && <p className="wx-overview clamp">{hero.overview}</p>}
            <div className="wx-actions">
              <button className="wx-btn play" onClick={() => navigate(playHref(hero.kind, hero.tmdbId, 1, 1))}>
                <Icon name="play" size={22} /> Play
              </button>
              <button className="wx-btn" onClick={() => navigate(titleHref(hero))}>
                <Icon name="info" size={20} /> More info
              </button>
            </div>
          </div>
        </header>
      )}
      <div className="wx-rows" style={hero ? undefined : { marginTop: 90 }}>
        {!only && <SetupCard />}
        {rows.map((r) => (
          <Row key={r.key} title={r.title} items={r.items} wide={r.key === 'continue' || r.key === 'new-episodes'} />
        ))}
        {rows.length === 0 && (
          <p className="wx-dim" style={{ padding: '0 var(--wx-gutter)' }}>
            Nothing to show yet. Cue needs its movie info key: the owner can add it in Settings.
          </p>
        )}
      </div>
    </div>
  )
}
