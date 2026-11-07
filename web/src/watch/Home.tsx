import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchCard, type WatchHome, type WatchKind } from '../api'
import Icon from '../components/Icon'
import SetupCard from './SetupCard'
import { useDocumentTitle } from '../documentTitle'
import { Ambient, epCode, playHref, Row, SpotlightContext, TitleMark, titleHref, useExtras, useTint } from './parts'
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
    <div className={tv ? 'wx-tvhome' : 'wx-home'}>
      {/* The banner's picture, soft and dark, glowing behind the page. */}
      {shown?.backdropUrl && <Ambient url={shown.backdropUrl} />}
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
//
// The banner comes alive: its picture's colour tints the page, and once the
// viewer rests on it for a moment its trailer fades in, muted, behind the
// logo (on a computer, not a phone: no surprise data use, no sound).
function Banner({ card, featured }: { card: WatchCard; featured: boolean }) {
  const navigate = useNavigate()
  // A title being watched carries on where it was; anything else starts at
  // the beginning (the first episode of a show).
  const resumes = !featured && (card.progress !== undefined || !!card.episode)
  const play = resumes ? playHref(card.kind, card.tmdbId, card.season, card.episode) : playHref(card.kind, card.tmdbId, 1, 1)
  const ex = useExtras(card)
  const logo = card.logoUrl !== undefined ? card.logoUrl : ex === undefined ? undefined : (ex?.logo ?? '')
  useTint(ex?.colour)
  const trailer = useTrailer(ex?.trailer, `${card.kind}-${card.tmdbId}`)
  return (
    <header className={`wx-hero${onTV() ? '' : ' inset'}${trailer.playing ? ' playing' : ''}`} ref={trailer.box}>
      {card.backdropUrl && <div className="wx-hero-pic drift" key={card.backdropUrl} style={{ backgroundImage: `url(${card.backdropUrl})` }} />}
      {trailer.src && (
        <div className="wx-hero-trailer" aria-hidden="true">
          <iframe
            ref={trailer.frame}
            key={trailer.src}
            src={trailer.src}
            title="Trailer"
            tabIndex={-1}
            allow="autoplay; encrypted-media"
            // The site sends no referrer, and YouTube's player refuses to
            // start without one (its error 153): send the site's address only.
            referrerPolicy="strict-origin-when-cross-origin"
            onLoad={trailer.loaded}
          />
        </div>
      )}
      {trailer.waiting && <span className="wx-trailer-wait" aria-hidden="true" />}
      {trailer.playing && (
        <button className="wx-trailer-sound" onClick={trailer.toggleSound} aria-label={trailer.sound ? 'Mute the trailer' : 'Play the trailer with sound'}>
          <Icon name={trailer.sound ? 'volume' : 'volume-x'} size={18} />
        </button>
      )}
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

// How long the banner rests before its trailer starts, how long it stays out
// of sight after YouTube says it's playing (so YouTube's own title and
// buttons are gone), and how long a trailer plays before the banner settles
// back to its picture.
const TRAILER_REST = 2500
const TRAILER_HIDDEN = 1200
const TRAILER_LENGTH = 60_000

// A computer with a mouse, a wide enough window, and no request for less
// motion: phones and TVs keep the still picture.
function trailersAllowed(): boolean {
  return matchMedia('(hover: hover) and (min-width: 900px)').matches && !matchMedia('(prefers-reduced-motion: reduce)').matches && !onTV()
}

function useTrailer(id: string | undefined, forKey: string) {
  const box = useRef<HTMLElement>(null)
  const frame = useRef<HTMLIFrameElement>(null)
  const [state, setState] = useState<{ key: string; src: string; playing: boolean; waiting: boolean; sound: boolean }>({
    key: '',
    src: '',
    playing: false,
    waiting: false,
    sound: false,
  })
  useEffect(() => {
    setState({ key: forKey, src: '', playing: false, waiting: false, sound: false })
    if (!id || !trailersAllowed()) return
    let rest = 0
    let stopAt = 0
    let visible = true
    const arm = () => {
      window.clearTimeout(rest)
      if (!visible || document.hidden) return
      setState((s) => ({ ...s, waiting: true }))
      rest = window.setTimeout(() => {
        const q = new URLSearchParams({
          autoplay: '1', mute: '1', controls: '0', playsinline: '1', rel: '0', modestbranding: '1',
          iv_load_policy: '3', disablekb: '1', enablejsapi: '1', loop: '1', playlist: id, start: '4',
          origin: location.origin,
        })
        setState((s) => ({ ...s, waiting: false, src: `https://www.youtube-nocookie.com/embed/${id}?${q}` }))
        stopAt = window.setTimeout(stop, TRAILER_LENGTH)
      }, TRAILER_REST)
    }
    const stop = () => {
      window.clearTimeout(rest)
      window.clearTimeout(stopAt)
      setState((s) => ({ ...s, src: '', playing: false, waiting: false, sound: false }))
    }
    const seen = new IntersectionObserver(
      ([e]) => {
        visible = e.intersectionRatio > 0.5
        if (visible) arm()
        else stop()
      },
      { threshold: [0, 0.5, 1] },
    )
    if (box.current) seen.observe(box.current)
    const onHide = () => (document.hidden ? stop() : arm())
    document.addEventListener('visibilitychange', onHide)
    arm()
    return () => {
      stop()
      seen.disconnect()
      document.removeEventListener('visibilitychange', onHide)
    }
  }, [id, forKey])
  const command = (func: string) =>
    frame.current?.contentWindow?.postMessage(JSON.stringify({ event: 'command', func, args: [] }), 'https://www.youtube-nocookie.com')
  // The trailer shows only once YouTube says it's playing; if it can't play
  // (an error, or not allowed outside YouTube) the banner keeps its picture.
  useEffect(() => {
    let reveal = 0
    const onMsg = (e: MessageEvent) => {
      if (e.origin !== 'https://www.youtube-nocookie.com' || e.source !== frame.current?.contentWindow || typeof e.data !== 'string') return
      let m: { event?: string; info?: unknown }
      try {
        m = JSON.parse(e.data)
      } catch {
        return
      }
      const state =
        m.event === 'onStateChange' ? m.info : m.event === 'infoDelivery' && m.info && typeof m.info === 'object' ? (m.info as { playerState?: number }).playerState : undefined
      if (m.event === 'onError') {
        window.clearTimeout(reveal)
        setState((s) => ({ ...s, src: '', playing: false }))
      } else if (state === 1) {
        window.clearTimeout(reveal)
        reveal = window.setTimeout(() => setState((s) => (s.src ? { ...s, playing: true } : s)), TRAILER_HIDDEN)
      }
    }
    window.addEventListener('message', onMsg)
    return () => {
      window.clearTimeout(reveal)
      window.removeEventListener('message', onMsg)
    }
  }, [])
  return {
    box,
    frame,
    src: state.key === forKey ? state.src : '',
    playing: state.key === forKey && state.playing,
    waiting: state.key === forKey && state.waiting,
    sound: state.sound,
    // The player loads out of sight; it's asked to say how it's doing (a
    // few times: it may not be ready at the first).
    loaded: () => {
      for (const ms of [0, 500, 1500, 3000]) {
        window.setTimeout(
          () => frame.current?.contentWindow?.postMessage(JSON.stringify({ event: 'listening', id: 1, channel: 'widget' }), 'https://www.youtube-nocookie.com'),
          ms,
        )
      }
    },
    toggleSound: () =>
      setState((s) => {
        command(s.sound ? 'mute' : 'unMute')
        return { ...s, sound: !s.sound }
      }),
  }
}
