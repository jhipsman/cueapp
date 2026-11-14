import { useEffect, useRef, useState, type PointerEvent } from 'react'
import { Link } from 'react-router-dom'
import Icon from '../../components/Icon'
import { Bars, useCoverTint } from './MusicPages'
import { clock, sized, useMusic } from './player'
import './music.css'

// The bar along the bottom while music is on (above the phone's tabs), and,
// tapped, the whole screen: the cover big, the time bar, the controls and
// what's up next.

export default function PlayerBar() {
  const m = useMusic()
  const [open, setOpen] = useState(false)
  // Room under the page for the bar.
  useEffect(() => {
    const on = !!m.current
    document.documentElement.classList.toggle('mu-has-bar', on)
    return () => document.documentElement.classList.remove('mu-has-bar')
  }, [m.current])
  if (!m.current) return null
  const t = m.current
  const frac = m.length > 0 ? Math.min(1, m.pos / m.length) : 0
  return (
    <>
      <div className="mu-bar" role="region" aria-label="Now playing">
        <button className="mu-bar-open" onClick={() => setOpen(true)} aria-label="Open the player">
          <img src={sized(t.cover, 120)} alt="" />
          <span className="mu-bar-words">
            <strong>{t.title}</strong>
            <small>{m.note || t.artist.name}</small>
          </span>
        </button>
        <div className="mu-bar-controls">
          <button className="mu-bar-btn wide-only" onClick={m.prev} aria-label="Previous">
            <Icon name="skip-back" size={20} fill="currentColor" />
          </button>
          <button className="mu-bar-btn play" onClick={m.toggle} aria-label={m.playing ? 'Pause' : 'Play'}>
            {m.waiting && m.playing ? <span className="mu-spin" /> : <Icon name={m.playing ? 'pause' : 'play'} size={22} fill="currentColor" />}
          </button>
          <button className="mu-bar-btn" onClick={m.next} aria-label="Next">
            <Icon name="skip-forward" size={20} fill="currentColor" />
          </button>
        </div>
        <span className="mu-bar-progress" style={{ transform: `scaleX(${m.waiting ? m.fetchProgress : frac})` }} />
      </div>
      {open && <NowPlaying close={() => setOpen(false)} />}
    </>
  )
}

function NowPlaying({ close }: { close: () => void }) {
  const m = useMusic()
  const t = m.current
  const tint = useCoverTint(t?.cover)
  const [showQueue, setShowQueue] = useState(false)
  const shell = useRef<HTMLDivElement>(null)
  const [leaving, setLeaving] = useState(false)
  const shut = () => {
    setLeaving(true)
    window.setTimeout(close, 220)
  }
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') shut()
      if (e.key === ' ' && (e.target as HTMLElement).tagName !== 'INPUT') {
        e.preventDefault()
        m.toggle()
      }
    }
    window.addEventListener('keydown', onKey)
    shell.current?.querySelector<HTMLButtonElement>('.mu-np-play')?.focus()
    return () => window.removeEventListener('keydown', onKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [m.toggle])
  if (!t) return null
  const upNext = m.queue.slice(m.index + 1, m.index + 31)
  return (
    <div className={`mu-np${leaving ? ' leaving' : ''}`} ref={shell} role="dialog" aria-label="Now playing" style={tint ? { ['--mu-tint' as string]: tint } : undefined}>
      <div className="mu-np-top">
        <button className="mu-icon-btn" onClick={shut} aria-label="Close">
          <Icon name="chevron-down" size={26} />
        </button>
        <small>
          Playing from <Link to={`/watch/music/album/${t.albumId}`} onClick={shut}>{t.albumTitle}</Link>
        </small>
        <button className={`mu-icon-btn${showQueue ? ' on' : ''}`} onClick={() => setShowQueue((s) => !s)} aria-label="Up next">
          <Icon name="list" size={22} />
        </button>
      </div>
      {showQueue ? (
        <div className="mu-np-queue">
          <h3>Now playing</h3>
          <QueueRow i={m.index} />
          {upNext.length > 0 && <h3>Up next</h3>}
          {upNext.map((_, k) => (
            <QueueRow key={m.index + 1 + k} i={m.index + 1 + k} />
          ))}
        </div>
      ) : (
        <img className={`mu-np-cover${m.playing ? '' : ' paused'}`} src={sized(t.cover, 1000)} alt="" key={t.albumId} />
      )}
      <div className="mu-np-words">
        <div>
          <strong>{t.title}</strong>
          <Link to={`/watch/music/artist/${t.artist.id}`} onClick={shut}>
            {t.artist.name}
          </Link>
        </div>
      </div>
      {m.note && <p className="mu-np-note">{m.note}</p>}
      <Scrubber />
      <div className="mu-np-controls">
        <button className={`mu-icon-btn${m.shuffle ? ' on' : ''}`} onClick={() => m.setShuffle(!m.shuffle)} aria-label="Shuffle" aria-pressed={m.shuffle}>
          <Icon name="shuffle" size={22} />
        </button>
        <button className="mu-icon-btn big" onClick={m.prev} aria-label="Previous">
          <Icon name="skip-back" size={30} fill="currentColor" />
        </button>
        <button className="mu-np-play" onClick={m.toggle} aria-label={m.playing ? 'Pause' : 'Play'}>
          {m.waiting && m.playing ? <span className="mu-spin dark" /> : <Icon name={m.playing ? 'pause' : 'play'} size={32} fill="currentColor" />}
        </button>
        <button className="mu-icon-btn big" onClick={m.next} aria-label="Next">
          <Icon name="skip-forward" size={30} fill="currentColor" />
        </button>
        <button className={`mu-icon-btn${m.repeat !== 'off' ? ' on' : ''}`} onClick={m.cycleRepeat} aria-label={`Repeat: ${m.repeat}`}>
          <Icon name="repeat" size={22} />
          {m.repeat === 'one' && <i className="mu-one">1</i>}
        </button>
      </div>
    </div>
  )
}

function QueueRow({ i }: { i: number }) {
  const m = useMusic()
  const t = m.queue[i]
  if (!t) return null
  const isCurrent = i === m.index
  return (
    <button className={`mu-song-main mu-q${isCurrent ? ' current' : ''}`} onClick={() => !isCurrent && m.jump(i)}>
      <img src={sized(t.cover, 120)} alt="" />
      <span className="mu-song-words">
        <strong>{t.title}</strong>
        <small>{t.artist.name}</small>
      </span>
      {isCurrent && m.playing ? <Bars /> : <span className="mu-song-len">{clock(t.length)}</span>}
    </button>
  )
}

// The time bar: drag or tap to move; the times either side.
function Scrubber() {
  const m = useMusic()
  const [drag, setDrag] = useState<number | null>(null)
  const bar = useRef<HTMLDivElement>(null)
  const at = (e: PointerEvent) => {
    const r = bar.current?.getBoundingClientRect()
    if (!r || m.length <= 0) return 0
    return Math.max(0, Math.min(1, (e.clientX - r.left) / r.width)) * m.length
  }
  const shown = drag ?? m.pos
  const frac = m.length > 0 ? shown / m.length : 0
  return (
    <div className="mu-scrub">
      <div
        className={`mu-scrub-bar${drag !== null ? ' active' : ''}`}
        ref={bar}
        role="slider"
        aria-label="Position"
        aria-valuemin={0}
        aria-valuemax={Math.round(m.length)}
        aria-valuenow={Math.round(shown)}
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'ArrowRight') m.seek(m.pos + 5)
          if (e.key === 'ArrowLeft') m.seek(m.pos - 5)
        }}
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId)
          setDrag(at(e))
        }}
        onPointerMove={(e) => drag !== null && setDrag(at(e))}
        onPointerUp={(e) => {
          if (drag !== null) m.seek(at(e))
          setDrag(null)
        }}
        onPointerCancel={() => setDrag(null)}
      >
        <span className="mu-scrub-fill" style={{ transform: `scaleX(${frac})` }} />
        <span className="mu-scrub-knob" style={{ left: `${frac * 100}%` }} />
      </div>
      <div className="mu-scrub-times">
        <span>{clock(shown)}</span>
        <span>{clock(m.length)}</span>
      </div>
    </div>
  )
}
