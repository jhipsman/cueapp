import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import Icon from '../components/Icon'

// The player's controls, Netflix style, over a <video>: a back arrow, the
// time bar with what's loaded, play, 10 seconds back and forward, volume,
// the title in the middle, and on the right whatever the page adds (next
// episode, versions, channels), playback speed and full screen. They fade
// while the video plays and nothing moves.
//
// Keys: space or K plays and pauses, left and right skip 10 seconds, up and
// down change the volume (not live: there they change channel), F is full
// screen, M mutes. On a touch screen, a tap shows the controls and a double
// tap on either side skips 10 seconds.

export interface MenuItem {
  label: string
  hint?: string
  href?: string
  onClick?: () => void
  disabled?: boolean
}

interface Props {
  video: HTMLVideoElement | null
  title: string
  subtitle?: string
  onBack: () => void
  live?: boolean
  note?: string
  // Buttons for the right of the bottom row (next episode, channels...).
  actions?: ReactNode
  // The "Versions" (or other) menu, with its heading.
  menu?: { label: string; icon: 'layers' | 'tv'; heading?: string; items: MenuItem[] }
}

const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2]

function clock(sec: number): string {
  if (!isFinite(sec) || sec < 0) sec = 0
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}` : `${m}:${String(s).padStart(2, '0')}`
}

// Changes to the <video> element itself (it isn't React state).
const media = {
  seek(v: HTMLVideoElement, t: number) {
    v.currentTime = t
  },
  volume(v: HTMLVideoElement, n: number) {
    v.volume = n
    v.muted = n === 0
  },
  mute(v: HTMLVideoElement, on: boolean) {
    v.muted = on
  },
  speed(v: HTMLVideoElement, r: number) {
    v.playbackRate = r
  },
}

function Skip({ back }: { back?: boolean }) {
  return (
    <svg viewBox="0 0 24 24" width={30} height={30} fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <g transform={back ? undefined : 'translate(24 0) scale(-1 1)'}>
        <path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
        <path d="M3 3v5h5" />
      </g>
      <text x="12" y="15.5" textAnchor="middle" fontSize="8" fontWeight="700" fill="currentColor" stroke="none">
        10
      </text>
    </svg>
  )
}

export default function VideoControls({ video, title, subtitle, onBack, live, note, actions, menu }: Props) {
  const root = useRef<HTMLDivElement>(null)
  const bar = useRef<HTMLDivElement>(null)
  const [playing, setPlaying] = useState(false)
  const [time, setTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [loaded, setLoaded] = useState(0)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [waiting, setWaiting] = useState(true)
  const [full, setFull] = useState(false)
  const [idle, setIdle] = useState(false)
  const [open, setOpen] = useState<'menu' | 'speed' | null>(null)
  const [speed, setSpeed] = useState(1)
  const [hover, setHover] = useState<number | null>(null) // 0..1 along the bar
  const [dragging, setDragging] = useState(false)
  const [flash, setFlash] = useState<{ kind: 'play' | 'pause' | 'back' | 'fwd'; n: number } | null>(null)
  const idleTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const lastTap = useRef<{ t: number; x: number }>({ t: 0, x: 0 })

  // Follow the video.
  useEffect(() => {
    const v = video
    if (!v) return
    const sync = () => {
      setPlaying(!v.paused)
      setTime(v.currentTime)
      setDuration(v.duration)
      setVolume(v.volume)
      setMuted(v.muted)
      setSpeed(v.playbackRate)
      const b = v.buffered
      let end = 0
      for (let i = 0; i < b.length; i++) if (b.start(i) <= v.currentTime + 1) end = Math.max(end, b.end(i))
      setLoaded(end)
    }
    const wait = () => setWaiting(true)
    const go = () => {
      setWaiting(false)
      sync()
    }
    const evs = ['play', 'pause', 'timeupdate', 'durationchange', 'volumechange', 'progress', 'ratechange', 'loadedmetadata']
    evs.forEach((e) => v.addEventListener(e, sync))
    v.addEventListener('waiting', wait)
    v.addEventListener('playing', go)
    v.addEventListener('canplay', go)
    sync()
    // A video that was already playing (Live TV's window going full
    // screen) sends no "playing" again.
    setWaiting(v.readyState < 3)
    return () => {
      evs.forEach((e) => v.removeEventListener(e, sync))
      v.removeEventListener('waiting', wait)
      v.removeEventListener('playing', go)
      v.removeEventListener('canplay', go)
    }
  }, [video])

  useEffect(() => {
    const on = () => setFull(document.fullscreenElement !== null)
    document.addEventListener('fullscreenchange', on)
    return () => document.removeEventListener('fullscreenchange', on)
  }, [])

  // The controls fade a few seconds after the last movement while playing.
  const wake = useCallback(() => {
    setIdle(false)
    clearTimeout(idleTimer.current)
    idleTimer.current = setTimeout(() => setIdle(true), 3000)
  }, [])
  useEffect(() => {
    wake()
    return () => clearTimeout(idleTimer.current)
  }, [wake])
  const hidden = idle && playing && open === null && !dragging

  const toggle = useCallback(() => {
    if (!video) return
    if (video.paused) void video.play().catch(() => undefined)
    else video.pause()
    setFlash((f) => ({ kind: video.paused ? 'pause' : 'play', n: (f?.n ?? 0) + 1 }))
  }, [video])

  const skip = useCallback(
    (sec: number) => {
      if (!video || live) return
      media.seek(video, Math.max(0, Math.min(video.currentTime + sec, (video.duration || 0) - 1)))
      setFlash((f) => ({ kind: sec < 0 ? 'back' : 'fwd', n: (f?.n ?? 0) + 1 }))
      wake()
    },
    [video, live, wake],
  )

  const fullscreen = useCallback(() => {
    if (document.fullscreenElement) {
      void document.exitFullscreen().catch(() => undefined)
      return
    }
    const el = root.current?.parentElement
    if (el?.requestFullscreen) {
      void el.requestFullscreen().catch(() => undefined)
    } else {
      // iPhone: only the video itself can go full screen.
      ;(video as (HTMLVideoElement & { webkitEnterFullscreen?: () => void }) | null)?.webkitEnterFullscreen?.()
    }
  }, [video])

  const setVol = useCallback(
    (v: number) => {
      if (!video) return
      media.volume(video, Math.max(0, Math.min(1, v)))
    },
    [video],
  )

  // Keys.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!video || e.ctrlKey || e.metaKey || e.altKey) return
      const tag = (e.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA') return
      let handled = true
      switch (e.key) {
        case ' ':
        case 'k':
        case 'K':
          toggle()
          break
        case 'ArrowLeft':
          if (live) handled = false
          else skip(-10)
          break
        case 'ArrowRight':
          if (live) handled = false
          else skip(10)
          break
        case 'ArrowUp':
          if (live) handled = false
          else setVol(video.volume + 0.1)
          break
        case 'ArrowDown':
          if (live) handled = false
          else setVol(video.volume - 0.1)
          break
        case 'f':
        case 'F':
          fullscreen()
          break
        case 'm':
        case 'M':
          media.mute(video, !video.muted)
          break
        default:
          handled = false
      }
      if (handled) {
        e.preventDefault()
        wake()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [video, live, toggle, skip, setVol, fullscreen, wake])

  // The time bar: click or drag to jump.
  const fraction = (clientX: number) => {
    const r = bar.current?.getBoundingClientRect()
    if (!r || r.width === 0) return 0
    return Math.max(0, Math.min(1, (clientX - r.left) / r.width))
  }
  const seekTo = (f: number) => {
    if (video && isFinite(video.duration)) media.seek(video, f * video.duration)
  }
  const onBarDown = (e: React.PointerEvent) => {
    e.preventDefault()
    ;(e.target as HTMLElement).setPointerCapture?.(e.pointerId)
    setDragging(true)
    const f = fraction(e.clientX)
    setHover(f)
    seekTo(f)
  }
  const onBarMove = (e: React.PointerEvent) => {
    const f = fraction(e.clientX)
    setHover(f)
    if (dragging) seekTo(f)
  }
  const onBarUp = () => setDragging(false)

  // A click on the picture plays or pauses (a double click is full screen);
  // on a touch screen a tap shows the controls, a double tap on a side skips.
  const onSurface = (e: React.PointerEvent) => {
    if (open) {
      setOpen(null)
      return
    }
    const now = Date.now()
    const x = e.clientX / window.innerWidth
    const double = now - lastTap.current.t < 300
    lastTap.current = { t: now, x }
    if (e.pointerType === 'touch') {
      if (double && !live && (x < 0.4 || x > 0.6)) {
        skip(x < 0.4 ? -10 : 10)
        return
      }
      wake()
      return
    }
    if (double) {
      fullscreen()
      toggle() // undo the first click's pause
      return
    }
    toggle()
    wake()
  }

  const pct = (n: number) => (duration > 0 && isFinite(duration) ? `${Math.min(100, (n / duration) * 100)}%` : '0%')
  const showBar = !live && duration > 0 && isFinite(duration)
  const volIcon = muted || volume === 0 ? 'volume-x' : volume < 0.5 ? 'volume-low' : 'volume'

  return (
    <div
      ref={root}
      className={`vx${hidden ? ' vx-hidden' : ''}`}
      onPointerMove={(e) => {
        if (e.pointerType !== 'touch') wake()
      }}
    >
      <div className="vx-surface" onPointerUp={onSurface} />

      {waiting && video && (
        <div className="vx-center" aria-hidden="true">
          <div className="vx-spinner" />
        </div>
      )}
      {flash && !waiting && (
        <div key={flash.n} className="vx-flash" aria-hidden="true">
          {flash.kind === 'play' && <Icon name="play" size={44} fill="currentColor" />}
          {flash.kind === 'pause' && <Icon name="pause" size={44} fill="currentColor" />}
          {flash.kind === 'back' && <Skip back />}
          {flash.kind === 'fwd' && <Skip />}
        </div>
      )}

      <div className="vx-top">
        <button className="vx-icon" onClick={onBack} aria-label="Back">
          <Icon name="arrow-left" size={30} />
        </button>
        {note && <div className="vx-note">{note}</div>}
      </div>

      <div className="vx-bottom" onPointerUp={(e) => e.stopPropagation()}>
        {showBar && (
          <div className="vx-timeline">
            <div
              ref={bar}
              className={`vx-bar${dragging ? ' active' : ''}`}
              onPointerDown={onBarDown}
              onPointerMove={onBarMove}
              onPointerUp={onBarUp}
              onPointerLeave={() => !dragging && setHover(null)}
              role="slider"
              aria-label="Time"
              aria-valuemin={0}
              aria-valuemax={Math.floor(duration)}
              aria-valuenow={Math.floor(time)}
              aria-valuetext={`${clock(time)} of ${clock(duration)}`}
              tabIndex={0}
            >
              <div className="vx-track">
                <div className="vx-loaded" style={{ width: pct(loaded) }} />
                <div className="vx-played" style={{ width: pct(time) }} />
              </div>
              <div className="vx-thumb" style={{ left: pct(time) }} />
              {hover !== null && (
                <div className="vx-tip" style={{ left: `${hover * 100}%` }}>
                  {clock(hover * duration)}
                </div>
              )}
            </div>
            <span className="vx-remaining">{clock(duration - time)}</span>
          </div>
        )}

        <div className="vx-row">
          <div className="vx-left">
            <button className="vx-icon" onClick={toggle} aria-label={playing ? 'Pause' : 'Play'}>
              <Icon name={playing ? 'pause' : 'play'} size={30} fill="currentColor" />
            </button>
            {!live && (
              <>
                <button className="vx-icon" onClick={() => skip(-10)} aria-label="Back 10 seconds">
                  <Skip back />
                </button>
                <button className="vx-icon" onClick={() => skip(10)} aria-label="Forward 10 seconds">
                  <Skip />
                </button>
              </>
            )}
            <div className="vx-volume">
              <button className="vx-icon" onClick={() => video && media.mute(video, !video.muted)} aria-label={muted ? 'Unmute' : 'Mute'}>
                <Icon name={volIcon} size={28} />
              </button>
              <input
                type="range"
                min={0}
                max={1}
                step={0.05}
                value={muted ? 0 : volume}
                onChange={(e) => setVol(Number(e.target.value))}
                aria-label="Volume"
                style={{ ['--vx-vol' as string]: `${(muted ? 0 : volume) * 100}%` }}
              />
            </div>
            {live && (
              <span className="vx-live">
                <i /> LIVE
              </span>
            )}
          </div>

          <div className="vx-title">
            <b>{title}</b>
            {subtitle && <span>{subtitle}</span>}
          </div>

          <div className="vx-right">
            {actions}
            {menu && (
              <div className="vx-pop-wrap">
                <button className="vx-icon" onClick={() => setOpen(open === 'menu' ? null : 'menu')} aria-label={menu.label} aria-expanded={open === 'menu'}>
                  <Icon name={menu.icon} size={26} />
                </button>
                {open === 'menu' && (
                  <div className="vx-pop" role="menu">
                    {menu.heading && <div className="vx-pop-head">{menu.heading}</div>}
                    {menu.items.map((it) =>
                      it.href ? (
                        <a key={it.label} role="menuitem" href={it.href} onClick={() => setOpen(null)}>
                          {it.label}
                          {it.hint && <small>{it.hint}</small>}
                        </a>
                      ) : (
                        <button
                          key={it.label}
                          role="menuitem"
                          disabled={it.disabled}
                          onClick={() => {
                            setOpen(null)
                            it.onClick?.()
                          }}
                        >
                          {it.label}
                          {it.hint && <small>{it.hint}</small>}
                        </button>
                      ),
                    )}
                  </div>
                )}
              </div>
            )}
            {!live && (
              <div className="vx-pop-wrap">
                <button className="vx-icon" onClick={() => setOpen(open === 'speed' ? null : 'speed')} aria-label="Playback speed" aria-expanded={open === 'speed'}>
                  <Icon name="gauge" size={26} />
                </button>
                {open === 'speed' && (
                  <div className="vx-pop" role="menu">
                    <div className="vx-pop-head">Playback speed</div>
                    {SPEEDS.map((s) => (
                      <button
                        key={s}
                        role="menuitemradio"
                        aria-checked={speed === s}
                        className={speed === s ? 'on' : ''}
                        onClick={() => {
                          if (video) media.speed(video, s)
                          setOpen(null)
                        }}
                      >
                        {s === 1 ? '1x (normal)' : `${s}x`}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
            <button className="vx-icon" onClick={fullscreen} aria-label={full ? 'Exit full screen' : 'Full screen'}>
              <Icon name={full ? 'minimize' : 'maximize'} size={26} />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
