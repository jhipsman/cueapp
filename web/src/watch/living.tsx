import { useEffect, useRef, useState } from 'react'
import Icon from '../components/Icon'
import { api, type WatchCard } from '../api'
import { onTV } from './tv'

// Bringing a title's artwork alive, on Home's banner, a title's page and a
// card the mouse rests on: its own preview clip, silent, once the viewer has
// rested a moment; without one, real frames from the title itself.

// How long the banner rests before its preview is asked for, and how long a
// preview plays before the banner settles back to its picture.
const TRAILER_REST = 500
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

// usePreview: a title's own preview clip (cut on the server, kept),
// as the streaming services do it: asked for once the viewer has rested a
// moment, played in an ordinary video element, and shown only once it's
// playing, so nothing but the picture ever appears. While the server is
// still making it the page asks again every couple of seconds. none says
// there won't be one (the page shows real frames instead). ready asks only
// for a clip already made (a card the mouse passes over).
type PreviewOf = { kind: WatchCard['kind']; tmdbId: number; season?: number; episode?: number }
export function usePreview(of: PreviewOf | null, forKey: string, ready = false) {
  const [state, setState] = useState({ key: '', src: '', playing: false, waiting: false, none: false, over: false })
  useEffect(() => {
    setState({ key: forKey, src: '', playing: false, waiting: false, none: false, over: false })
    if (!of || !trailersAllowed()) return
    let stop = false
    let timer = 0
    let tries = 0
    const ask = async () => {
      if (stop) return
      try {
        const a = await api.watchPreview(of.kind, of.tmdbId, of.season, of.episode, ready)
        if (stop) return
        if (a.url) setState((s) => ({ ...s, src: a.url ?? '', waiting: true }))
        else if (a.pending && !ready && tries++ < 90) {
          setState((s) => ({ ...s, waiting: true }))
          timer = window.setTimeout(ask, 2000)
        } else setState((s) => ({ ...s, waiting: false, none: true }))
      } catch {
        if (!stop) setState((s) => ({ ...s, waiting: false, none: true }))
      }
    }
    timer = window.setTimeout(ask, ready ? 300 : TRAILER_REST)
    const end = window.setTimeout(() => setState((s) => ({ ...s, playing: false, over: true })), TRAILER_LENGTH)
    return () => {
      stop = true
      window.clearTimeout(timer)
      window.clearTimeout(end)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [forKey, ready])
  const current = state.key === forKey ? state : { key: forKey, src: '', playing: false, waiting: false, none: false, over: false }
  return {
    ...current,
    onPlaying: () => setState((s) => (s.src ? { ...s, playing: true, waiting: false } : s)),
  }
}

// The clip itself, looping and filling its box like a picture. With sound
// (a title's page, as on Netflix) it starts with sound when the browser
// allows it (it does once you've clicked on the site), otherwise muted; a
// button in the corner turns the sound on and off. Once the preview is over
// it fades away and stops.
export function PreviewVideo({ src, onPlaying, playing, over, sound = false }: { src: string; onPlaying: () => void; playing: boolean; over: boolean; sound?: boolean }) {
  const video = useRef<HTMLVideoElement>(null)
  const [muted, setMuted] = useState(true)
  const [hasSound, setHasSound] = useState(false)
  useEffect(() => {
    const v = video.current
    if (!v) return
    v.muted = true
    v.volume = 0.8
    if (!sound) {
      void v.play().catch(() => undefined)
      return
    }
    v.muted = false
    v.play()
      .then(() => setMuted(false))
      .catch(() => {
        v.muted = true
        setMuted(true)
        void v.play().catch(() => undefined)
      })
  }, [sound])
  // Fades the sound out with the picture, then stops.
  useEffect(() => {
    const v = video.current
    if (!v || !over) return
    let left = 10
    const t = window.setInterval(() => {
      v.volume = Math.max(0, v.volume - 0.08)
      if (--left <= 0) {
        window.clearInterval(t)
        v.pause()
      }
    }, 100)
    return () => window.clearInterval(t)
  }, [over])
  const toggle = () => {
    const v = video.current
    if (!v) return
    v.muted = !v.muted
    setMuted(v.muted)
    if (!v.muted && v.paused) void v.play().catch(() => undefined)
  }
  return (
    <>
      <div className="wx-hero-trailer" aria-hidden="true">
        <video
          ref={video}
          src={src}
          muted
          loop
          playsInline
          preload="auto"
          onPlaying={onPlaying}
          // Whether the clip has sound at all (some versions have none).
          onLoadedData={(e) => {
            const v = e.currentTarget as HTMLVideoElement & { mozHasAudio?: boolean; webkitAudioDecodedByteCount?: number; audioTracks?: { length: number } }
            setHasSound(v.mozHasAudio ?? (v.audioTracks ? v.audioTracks.length > 0 : true))
          }}
        />
      </div>
      {playing && !over && hasSound && (
        <button className="wx-trailer-sound" onClick={toggle} aria-label={muted ? 'Turn the sound on' : 'Mute'}>
          <Icon name={muted ? 'volume-x' : 'volume'} size={18} />
        </button>
      )}
    </>
  )
}

// A card's preview, while the mouse rests on it: only a clip already made.
export function CardPreview({ of }: { of: PreviewOf }) {
  const p = usePreview(of, `${of.kind}-${of.tmdbId}-${of.season ?? 0}-${of.episode ?? 0}`, true)
  if (!p.src) return null
  return (
    <span className={`wx-card-trailer${p.playing ? ' on' : ''}`} aria-hidden="true">
      <video src={p.src} muted autoPlay loop playsInline onPlaying={p.onPlaying} />
    </span>
  )
}
