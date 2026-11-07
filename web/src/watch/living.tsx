import { useEffect, useRef, useState } from 'react'
import { api, type WatchCard } from '../api'
import { onTV } from './tv'

// Bringing a title's artwork alive, on Home's banner, a title's page and a
// card the mouse rests on: its YouTube trailer, muted, once the viewer has
// rested a moment; without one, real frames from the title itself.

// How long the banner rests before its trailer starts, how long it stays out
// of sight after YouTube says it's playing (so YouTube's own title and
// buttons are gone), and how long a trailer plays before the banner settles
// back to its picture.
const TRAILER_REST = 2500
const TRAILER_HIDDEN = 1200
const TRAILER_LENGTH = 60_000

// When a title has no trailer, the banner shows real moments from it: a few
// sharp frames Cue takes from the stream Play would use (looked up now, so
// Play then starts at once), crossfading with a slow drift. For a title
// being watched they lead up to where you stopped (a little "previously");
// for one not started, they come from early on, too early to spoil much.
export function useFrames(card: WatchCard, wanted: boolean): string[] {
  const key = `${card.kind}-${card.tmdbId}-${card.season ?? 0}-${card.episode ?? 0}`
  const [got, setGot] = useState<{ key: string; urls: string[] }>({ key: '', urls: [] })
  useEffect(() => {
    if (!wanted || !trailersAllowed()) return
    let live = true
    const t = window.setTimeout(async () => {
      try {
        const season = card.kind === 'tv' ? (card.season ?? 1) : 0
        const episode = card.kind === 'tv' ? (card.episode ?? 1) : 0
        const a = await api.playTMDB(card.kind, card.tmdbId, season, episode, 1, false, true)
        const token = a.thumbs || a.convert
        if (!live || !token || a.youtube) return
        const { duration } = await api.convertProbe(token)
        if (!live || !duration || duration < 120) return
        const at =
          card.progress !== undefined && card.progress > 0.02
            ? [75, 55, 35, 15].map((back) => card.progress! * duration - back).filter((x) => x > 5)
            : [0.1, 0.15, 0.2, 0.26, 0.32].map((f) => f * duration)
        const urls = at.map((sec) => `/api/thumb?u=${encodeURIComponent(token)}&t=${Math.floor(sec)}&w=1280`)
        // Shown once they've loaded, so nothing pops in half drawn.
        const ok = await Promise.all(
          urls.map(
            (u) =>
              new Promise<string | null>((done) => {
                const img = new Image()
                img.onload = () => done(u)
                img.onerror = () => done(null)
                img.src = u
              }),
          ),
        )
        if (live) setGot({ key, urls: ok.filter((u): u is string => !!u) })
      } catch {
        // no stream to take frames from: the banner keeps its picture
      }
    }, TRAILER_REST)
    return () => {
      live = false
      window.clearTimeout(t)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, wanted])
  return got.key === key ? got.urls : []
}

// Frames: one picture at a time, each drifting, crossfading to the next.
export function Frames({ urls }: { urls: string[] }) {
  const [at, setAt] = useState(0)
  useEffect(() => {
    const t = window.setInterval(() => setAt((i) => (i + 1) % urls.length), 4200)
    return () => window.clearInterval(t)
  }, [urls.length])
  return (
    <div className="wx-hero-frames" aria-hidden="true">
      {urls.map((u, i) => (
        <div key={u} className={i === at ? 'show' : ''} style={{ backgroundImage: `url(${u})` }} />
      ))}
    </div>
  )
}

// A computer with a mouse, a wide enough window, and no request for less
// motion: phones and TVs keep the still picture.
export function trailersAllowed(): boolean {
  return matchMedia('(hover: hover) and (min-width: 900px)').matches && !matchMedia('(prefers-reduced-motion: reduce)').matches && !onTV()
}

export function useTrailer(id: string | undefined, forKey: string) {
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

// CardTrailer plays a title's trailer inside a card the mouse rests on, as
// Netflix does: muted, filling the card, shown only once YouTube says it's
// playing (YouTube won't play in a player under 200 pixels, so it runs at
// twice the card's size and is drawn at half).
export function CardTrailer({ id }: { id: string }) {
  const frame = useRef<HTMLIFrameElement>(null)
  const [playing, setPlaying] = useState(false)
  useEffect(() => {
    let reveal = 0
    const onMsg = (e: MessageEvent) => {
      if (e.origin !== YT || e.source !== frame.current?.contentWindow || typeof e.data !== 'string') return
      let m: { event?: string; info?: unknown }
      try {
        m = JSON.parse(e.data)
      } catch {
        return
      }
      const state =
        m.event === 'onStateChange' ? m.info : m.event === 'infoDelivery' && m.info && typeof m.info === 'object' ? (m.info as { playerState?: number }).playerState : undefined
      if (state === 1) {
        window.clearTimeout(reveal)
        reveal = window.setTimeout(() => setPlaying(true), 900)
      }
    }
    window.addEventListener('message', onMsg)
    return () => {
      window.clearTimeout(reveal)
      window.removeEventListener('message', onMsg)
    }
  }, [])
  const q = new URLSearchParams({
    autoplay: '1', mute: '1', controls: '0', playsinline: '1', rel: '0', modestbranding: '1',
    iv_load_policy: '3', disablekb: '1', enablejsapi: '1', loop: '1', playlist: id, start: '6',
    origin: location.origin,
  })
  const listen = () => {
    for (const ms of [0, 500, 1500, 3000]) {
      window.setTimeout(() => frame.current?.contentWindow?.postMessage(JSON.stringify({ event: 'listening', id: 1, channel: 'widget' }), YT), ms)
    }
  }
  return (
    <span className={`wx-card-trailer${playing ? ' on' : ''}`} aria-hidden="true">
      <iframe
        ref={frame}
        src={`${YT}/embed/${encodeURIComponent(id)}?${q}`}
        title="Trailer"
        tabIndex={-1}
        allow="autoplay; encrypted-media"
        referrerPolicy="strict-origin-when-cross-origin"
        onLoad={listen}
      />
    </span>
  )
}

const YT = 'https://www.youtube-nocookie.com'
