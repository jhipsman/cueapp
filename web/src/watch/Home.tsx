import { PartiesNow } from './party'
import { Fragment, useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchCard, type WatchHome, type WatchKind } from '../api'
import Icon from '../components/Icon'
import SetupCard from './SetupCard'
import { ReportNews } from './reports'
import { Greeting, NetworkStrip, TopTen } from './browse'
import { useDocumentTitle } from '../documentTitle'
import { Ambient, epCode, playHref, Row, SpotlightContext, TitleMark, titleHref, useExtras, useTint } from './parts'
import { Frames, PreviewVideo, useFrames, usePreview } from './living'
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
    hero = rows.flatMap((r) => r.items).find((c) => c.backdropUrl && c.overview && !c.quiet && c.progress === undefined && !c.episode)
  }
  const shown = (tv && spot) || hero
  // The Top 10 goes after Continue Watching and new episodes.
  const topAt = Math.max(0, rows.filter((r) => r.key === 'continue' || r.key === 'new-episodes' || r.key === 'out-now').length - 1)

  return (
    <div className={tv ? 'wx-tvhome' : 'wx-home'}>
      {/* The banner's picture, soft and dark, glowing behind the page. */}
      {shown?.backdropUrl && <Ambient url={shown.backdropUrl} />}
      {shown && <Banner card={shown} featured={shown === hero} />}
      <div className="wx-rows" style={shown ? undefined : { marginTop: 90 }}>
        {!only && <SetupCard />}
        {!only && !tv && <Greeting home={home} />}
        {/* The Films and Series homes: their own Top 10 first, then the networks. */}
        {only && home.top10 && <TopTen lists={home.top10} only={only} />}
        {only && !tv && home.top10 && <NetworkStrip kind={only} services={home.top10.map((l) => ({ key: l.key, name: l.name, logo: l.logo }))} />}
        {!only && <ReportNews />}
        {!only && <PartiesNow />}
        <SpotlightContext.Provider value={tv ? onSpot : null}>
          {rows.map((r, i) => (
            <Fragment key={r.key}>
              <Row rowKey={r.key} title={r.title} items={r.items} wide={r.key === 'continue' || r.key === 'new-episodes'} />
              {/* The Top 10 comes after what you're watching (or the first row). */}
              {home.top10 && !only && i === topAt && <TopTen lists={home.top10} only={only} />}
            </Fragment>
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
//
// The banner comes alive: its picture's colour tints the page, and once the
// viewer rests on it for a moment its preview fades in, silent, behind the
// logo (on a computer, not a phone: no surprise data use).
function Banner({ card, featured }: { card: WatchCard; featured: boolean }) {
  const navigate = useNavigate()
  // A title being watched carries on where it was; anything else starts at
  // the beginning (the first episode of a show).
  const resumes = !featured && (card.progress !== undefined || !!card.episode)
  const play = resumes ? playHref(card.kind, card.tmdbId, card.season, card.episode) : playHref(card.kind, card.tmdbId, 1, 1)
  const ex = useExtras(card)
  const logo = card.logoUrl !== undefined ? card.logoUrl : ex === undefined ? undefined : (ex?.logo ?? '')
  useTint(ex?.colour)
  // Its own preview clip (cut from the title on the server); when there
  // can't be one, real frames from the title itself, moving gently.
  const preview = usePreview(card, `${card.kind}-${card.tmdbId}-${card.season ?? 0}-${card.episode ?? 0}`)
  const frames = useFrames(card, preview.none)
  return (
    <header className={`wx-hero${onTV() ? '' : ' full'}${preview.playing && !preview.over ? ' playing' : ''}${frames.length > 1 ? ' framing' : ''}`}>
      {card.backdropUrl && <div className="wx-hero-pic drift" key={card.backdropUrl} style={{ backgroundImage: `url(${card.backdropUrl})` }} />}
      {preview.src && <PreviewVideo key={preview.src} src={preview.src} onPlaying={preview.onPlaying} onEnded={preview.onEnded} playing={preview.playing} over={preview.over} />}
      {frames.length > 1 && <Frames urls={frames} />}
      {preview.waiting && !preview.playing && <span className="wx-trailer-wait" aria-hidden="true" />}
      <div className="wx-hero-body rise" key={`${card.kind}-${card.tmdbId}-${card.episode ?? 0}`}>
        <TitleMark title={card.title} logo={logo} />
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
