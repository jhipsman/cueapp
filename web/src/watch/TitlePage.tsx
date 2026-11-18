import { usePrefs } from './prefs'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type WatchEpisode, type WatchKind, type WatchPerson, type WatchTitle } from '../api'
import { useScreens } from './TVRemote'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import FreeElsewhere from './FreeElsewhere'
import { Frames, PreviewVideo, useFrames, usePreview } from './living'
import { Ambient, Card, epCode, minutes, playHref, Row, Scores, TitleMark, useExtras, useTint } from './parts'
import { onTV } from './tv'

// A movie's or a show's page: the banner with ratings and Play, then for a
// show its seasons and episodes, then cast and "More like this".
export default function TitlePage({ kind }: { kind: WatchKind }) {
  const prefs = usePrefs()
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
  // title's own preview clip, silent, or (when there can't be one) real
  // frames from the episode or film Play would start.
  const previewOf = title ? { kind, tmdbId, season: title.resumeSeason, episode: title.resumeEpisode } : null
  const preview = usePreview(previewOf, previewOf ? `${kind}-${tmdbId}-${title?.resumeSeason ?? 0}-${title?.resumeEpisode ?? 0}` : '')
  const frames = useFrames({ kind, tmdbId, title: title?.title ?? '', season: title?.resumeSeason, episode: title?.resumeEpisode }, !!title && preview.none)

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

  const [thumb, setThumbState] = useState<'' | 'up' | 'down'>('')
  const [offRow, setOffRow] = useState(false)
  const [reminding, setReminding] = useState(false)
  useEffect(() => {
    setThumbState(title?.thumb ?? '')
    setReminding(!!title?.reminding)
  }, [title])
  async function toggleRemind() {
    const next = !reminding
    setReminding(next)
    try {
      await api.setRemind(kind, tmdbId, next)
    } catch {
      setReminding(!next)
    }
  }
  async function thumbIt(t: 'up' | 'down') {
    const next = thumb === t ? '' : t
    const was = thumb
    setThumbState(next)
    try {
      await api.setThumb(kind, tmdbId, next)
    } catch {
      setThumbState(was)
    }
  }
  async function hideFromRow() {
    setOffRow(true)
    try {
      await api.hideFromContinue(kind, tmdbId)
    } catch {
      setOffRow(false)
    }
  }

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
      <header className={`wx-hero${preview.playing && !preview.over ? ' playing' : ''}${frames.length > 1 ? ' framing' : ''}`}>
        {title.backdropUrl && <div className="wx-hero-pic settle" style={{ backgroundImage: `url(${title.backdropUrl})` }} />}
        {preview.src && <PreviewVideo key={preview.src} src={preview.src} onPlaying={preview.onPlaying} onEnded={preview.onEnded} playing={preview.playing} over={preview.over} sound={prefs.on('previewSound')} />}
        {frames.length > 1 && <Frames urls={frames} />}
        {preview.waiting && !preview.playing && <span className="wx-trailer-wait" aria-hidden="true" />}
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
          {title.theaterNote && (
            <p className="wx-theaters" role="note">
              <Icon name="film" size={16} /> {title.theaterNote}. What's found now is usually a camera recording.
            </p>
          )}
          <div className="wx-actions">
            {title.notOut ? (
              <>
                <span className="wx-btn wx-not-out" role="status">
                  <Icon name="calendar" size={20} /> {title.notOut}
                </span>
                <button className="wx-btn play" onClick={() => void toggleRemind()} aria-pressed={reminding}>
                  <Icon name={reminding ? 'check' : 'clock'} size={20} /> {reminding ? 'Reminding you' : 'Remind me'}
                </button>
              </>
            ) : (
              <button className="wx-btn play" onClick={() => navigate(playHref(kind, tmdbId, title.resumeSeason, title.resumeEpisode))}>
                <Icon name="play" size={22} /> {playLabel}
              </button>
            )}
            <button className="wx-btn" onClick={() => void toggleList()} aria-pressed={inList}>
              <Icon name={inList ? 'check' : 'plus'} size={20} strokeWidth={2.5} /> My List
            </button>
            {title.theaterNote && (
              <button className="wx-btn" onClick={() => void toggleRemind()} aria-pressed={reminding} title="Tells you when it streams properly">
                <Icon name={reminding ? 'check' : 'clock'} size={18} /> {reminding ? 'Reminding you' : 'Remind me when it streams'}
              </button>
            )}
            {!title.notOut && <PlayOnTV kind={kind} tmdbId={tmdbId} season={title.resumeSeason} episode={title.resumeEpisode} />}
            <button className={`wx-btn wx-round${thumb === 'up' ? ' on' : ''}`} onClick={() => void thumbIt('up')} aria-pressed={thumb === 'up'} title="I like this" aria-label="I like this">
              <Icon name="thumbs-up" size={20} />
            </button>
            <button
              className={`wx-btn wx-round${thumb === 'down' ? ' on' : ''}`}
              onClick={() => void thumbIt('down')}
              aria-pressed={thumb === 'down'}
              title="Not for me: keeps it off Home"
              aria-label="Not for me"
            >
              <Icon name="thumbs-down" size={20} />
            </button>
            {(resuming || startedShow) && (
              <button className="wx-btn" disabled={offRow} onClick={() => void hideFromRow()} title="Where you were is kept">
                <Icon name={offRow ? 'check' : 'x'} size={18} /> {offRow ? 'Off Continue Watching' : 'Remove from Continue Watching'}
              </button>
            )}
          </div>
        </div>
      </header>

      <div className="wx-page" style={{ paddingTop: 10 }}>
        <div className="wx-facts">
          <div>
            {title.cast.length > 0 && !title.people?.length && (
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

        {title.people && title.people.length > 0 && <CastRow people={title.people} />}

        {kind === 'tv' && title.seasons && title.seasons.length > 0 && season !== null && (
          <Episodes tmdbId={tmdbId} seasons={title.seasons} season={season} onSeason={setSeason} />
        )}

        {title.collection && <Row title={title.collection.name} items={title.collection.parts} />}

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

// PlayOnTV sends the title to a household TV with Cue open (asking which,
// when there are several), then opens the remote. Only shown while a TV is on.
function PlayOnTV({ kind, tmdbId, season, episode }: { kind: WatchKind; tmdbId: number; season?: number; episode?: number }) {
  const screens = useScreens(5000)
  const navigate = useNavigate()
  const [picking, setPicking] = useState(false)
  const [error, setError] = useState('')
  if (onTV() || !screens || screens.length === 0) return null
  const send = async (id: string) => {
    setError('')
    try {
      await api.screenPlay(id, { kind, tmdbId, season, episode })
      navigate(`/watch/tv-remote/${id}`)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }
  return (
    <span className="wx-playon">
      <button className="wx-btn" onClick={() => (screens.length === 1 ? void send(screens[0].id) : setPicking((p) => !p))}>
        <Icon name="tv" size={20} /> {screens.length === 1 ? `Play on ${screens[0].name}` : 'Play on TV'}
      </button>
      {picking && (
        <span className="wx-playon-menu">
          {screens.map((s) => (
            <button key={s.id} onClick={() => void send(s.id)}>
              {s.name}
            </button>
          ))}
        </span>
      )}
      {error && <span className="wx-error">{error}</span>}
    </span>
  )
}

// CastRow: the cast's photos; each opens what else they're in.
function CastRow({ people }: { people: WatchPerson[] }) {
  const navigate = useNavigate()
  return (
    <section className="wx-cast">
      <h2>Cast</h2>
      <div className="wx-cast-scroll">
        {people.map((p) => (
          <button key={p.id} className="wx-cast-p" onClick={() => navigate(`/watch/person/${p.id}`)}>
            <span className="wx-cast-pic">{p.photo ? <img src={p.photo} alt="" loading="lazy" /> : <span>{p.name.slice(0, 1)}</span>}</span>
            <b>{p.name}</b>
            {p.character && <small>{p.character}</small>}
          </button>
        ))}
      </div>
    </section>
  )
}
