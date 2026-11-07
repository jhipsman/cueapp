import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type WatchEpisode, type WatchKind, type WatchTitle } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import FreeElsewhere from './FreeElsewhere'
import { Frames, useFrames, useTrailer } from './living'
import { Ambient, Card, epCode, minutes, playHref, Row, Scores, TitleMark, useExtras, useTint } from './parts'
import { onTV } from './tv'

// A movie's or a show's page: the banner with ratings and Play, then for a
// show its seasons and episodes, then cast and "More like this".
export default function TitlePage({ kind }: { kind: WatchKind }) {
  const navigate = useNavigate()
  const tmdbId = Number(useParams().tmdbId)
  const [title, setTitle] = useState<WatchTitle | null>(null)
  const [error, setError] = useState('')
  const [inList, setInList] = useState(false)
  const [season, setSeason] = useState<number | null>(null)
  useDocumentTitle(title?.title ?? 'Watch')
  // The page takes the colour of the title's picture.
  const pageExtras = useExtras(title ? { kind, tmdbId } : null)
  useTint(pageExtras?.colour)
  // The big picture comes alive once the viewer rests here a moment: the
  // trailer, muted, or real frames from the episode or film Play would start.
  const trailer = useTrailer(pageExtras?.trailer, `${kind}-${tmdbId}`)
  const frames = useFrames(
    { kind, tmdbId, title: title?.title ?? '', season: title?.resumeSeason, episode: title?.resumeEpisode },
    !!title && pageExtras !== undefined && !pageExtras?.trailer,
  )

  useEffect(() => {
    let live = true
    setTitle(null)
    setError('')
    window.scrollTo(0, 0)
    api
      .watchTitle(kind, tmdbId)
      .then((t) => {
        if (!live) return
        setTitle(t)
        setInList(t.inList)
        // Look the stream up now, so Play starts straight away.
        void api.prefetchPlay(kind, tmdbId, t.resumeSeason ?? 1, t.resumeEpisode ?? 1)
        setSeason(t.resumeSeason ?? t.seasons?.[0]?.number ?? null)
      })
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [kind, tmdbId])

  async function toggleList() {
    const next = !inList
    setInList(next)
    try {
      if (next) await api.addToWatchList(kind, tmdbId)
      else await api.removeFromWatchList(kind, tmdbId)
    } catch {
      setInList(!next)
    }
  }

  if (error) {
    return (
      <div className="wx-page">
        <p className="wx-error">{error}</p>
      </div>
    )
  }
  if (!title) return <div className="wx-hero wx-skeleton" style={{ borderRadius: 0 }} />

  const resuming = (title.resumePosition ?? 0) > 30
  const startedShow = kind === 'tv' && (title.resumeSeason !== title.seasons?.[0]?.number || title.resumeEpisode !== 1 || resuming)
  const playLabel = resuming ? 'Resume' : startedShow ? `Play ${epCode(title.resumeSeason, title.resumeEpisode)}` : 'Play'

  return (
    <div className="wx-titlepage">
      {title.backdropUrl && <Ambient url={title.backdropUrl} />}
      <header className={`wx-hero${trailer.playing ? ' playing' : ''}${frames.length > 1 ? ' framing' : ''}`} ref={trailer.box}>
        {title.backdropUrl && <div className="wx-hero-pic settle" style={{ backgroundImage: `url(${title.backdropUrl})` }} />}
        {trailer.src && (
          <div className="wx-hero-trailer" aria-hidden="true">
            <iframe
              ref={trailer.frame}
              key={trailer.src}
              src={trailer.src}
              title="Trailer"
              tabIndex={-1}
              allow="autoplay; encrypted-media"
              referrerPolicy="strict-origin-when-cross-origin"
              onLoad={trailer.loaded}
            />
          </div>
        )}
        {frames.length > 1 && <Frames urls={frames} />}
        {trailer.waiting && <span className="wx-trailer-wait" aria-hidden="true" />}
        {trailer.playing && (
          <button className="wx-trailer-sound" onClick={trailer.toggleSound} aria-label={trailer.sound ? 'Mute the trailer' : 'Play the trailer with sound'}>
            <Icon name={trailer.sound ? 'volume' : 'volume-x'} size={18} />
          </button>
        )}
        <div className="wx-hero-body rise">
          <TitleMark title={title.title} logo={title.logoUrl ?? ''} />
          <div className="wx-meta">
            <Scores ratings={title.ratings} tmdb={title.rating} />
            {title.year ? <span>{title.year}</span> : null}
            {title.certification && <span className="wx-cert">{title.certification}</span>}
            {kind === 'movie' && title.runtime ? <span>{minutes(title.runtime)}</span> : null}
            {kind === 'tv' && title.seasons?.length ? (
              <span>
                {title.seasons.length} season{title.seasons.length > 1 ? 's' : ''}
              </span>
            ) : null}
          </div>
          {title.overview && <p className="wx-overview clamp">{title.overview}</p>}
          {title.directors && title.directors.length > 0 && (
            <p className="wx-credit">
              <span>{kind === 'tv' ? 'Created by' : 'Directed by'}</span> {title.directors.slice(0, 2).join(', ')}
            </p>
          )}
          <div className="wx-actions">
            <button className="wx-btn play" onClick={() => navigate(playHref(kind, tmdbId, title.resumeSeason, title.resumeEpisode))}>
              <Icon name="play" size={22} /> {playLabel}
            </button>
            <button className="wx-btn" onClick={() => void toggleList()} aria-pressed={inList}>
              <Icon name={inList ? 'check' : 'plus'} size={20} strokeWidth={2.5} /> My List
            </button>
          </div>
        </div>
      </header>

      <div className="wx-page" style={{ paddingTop: 10 }}>
        <div className="wx-facts">
          <div>
            {title.cast.length > 0 && (
              <p>
                <span>Cast: </span>
                {title.cast.slice(0, 6).join(', ')}
              </p>
            )}
            {title.genres.length > 0 && (
              <p>
                <span>Genres: </span>
                {title.genres.join(', ')}
              </p>
            )}
            {title.networks && title.networks.length > 0 && (
              <p>
                <span>On: </span>
                {title.networks.join(', ')}
              </p>
            )}
            <FreeElsewhere kind={kind} tmdbId={tmdbId} label="Also free on" subscriptions />
          </div>
        </div>

        {kind === 'tv' && title.seasons && title.seasons.length > 0 && season !== null && (
          <Episodes tmdbId={tmdbId} seasons={title.seasons} season={season} onSeason={setSeason} />
        )}

        {title.more.length > 0 && onTV() && <Row title="More like this" items={title.more.slice(0, 18)} />}
        {title.more.length > 0 && !onTV() && (
          <>
            <h2>More like this</h2>
            <div className="wx-grid">
              {title.more.slice(0, 18).map((c) => (
                <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  )
}

function Episodes({
  tmdbId,
  seasons,
  season,
  onSeason,
}: {
  tmdbId: number
  seasons: { number: number; name: string; episodes: number }[]
  season: number
  onSeason: (n: number) => void
}) {
  const navigate = useNavigate()
  const [loaded, setLoaded] = useState<{ season: number; eps: WatchEpisode[] } | null>(null)

  useEffect(() => {
    let live = true
    api
      .watchSeason(tmdbId, season)
      .then((eps) => live && setLoaded({ season, eps }))
      .catch(() => live && setLoaded({ season, eps: [] }))
    return () => {
      live = false
    }
  }, [tmdbId, season])

  const eps = loaded?.season === season ? loaded.eps : null
  return (
    <>
      <div className="wx-seasons">
        <h2>Episodes</h2>
        {seasons.length > 1 ? (
          // Buttons, not a drop-down: easy with a remote, one tap on a phone.
          <div className="wx-chips wx-season-chips" role="tablist" aria-label="Season">
            {seasons.map((s) => (
              <button key={s.number} role="tab" aria-selected={s.number === season} className={s.number === season ? 'active' : ''} onClick={() => onSeason(s.number)}>
                {s.name}
              </button>
            ))}
          </div>
        ) : (
          <span className="wx-dim">{seasons[0].name}</span>
        )}
      </div>
      {eps === null && <div className="wx-skeleton" style={{ height: 300 }} />}
      {eps && eps.length === 0 && <p className="wx-dim">TMDB has no episodes for this season yet.</p>}
      <div className="wx-episodes">
        {eps?.map((e) => (
          <button
            key={e.episode}
            className={`wx-ep${e.aired ? '' : ' unaired'}`}
            disabled={!e.aired}
            onClick={() => navigate(playHref('tv', tmdbId, e.season, e.episode))}
          >
            <div className="wx-ep-num">{e.episode}</div>
            <div className="wx-ep-thumb">
              {e.stillUrl && <img src={e.stillUrl} alt="" loading="lazy" />}
              {(e.progress ?? 0) > 0 && (
                <div className="wx-card-progress">
                  <div style={{ width: `${Math.round((e.finished ? 1 : e.progress!) * 100)}%` }} />
                </div>
              )}
            </div>
            <div>
              <div className="wx-ep-head">
                {e.title}
                <span>{e.aired ? minutes(e.runtime) : e.airDate ? `Coming ${e.airDate}` : 'Coming soon'}</span>
              </div>
              {e.overview && <p>{e.overview}</p>}
            </div>
          </button>
        ))}
      </div>
    </>
  )
}
