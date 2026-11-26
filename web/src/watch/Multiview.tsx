import { useEffect, useMemo, useState } from 'react'
import { api, type LiveChannel, type LivePlay } from '../api'
import Icon from '../components/Icon'
import ChannelLogo from './ChannelLogo'
import { useLiveStream } from './liveStream'

// Multiview: two or four channels at once, filling the screen. One has the
// sound (marked, with its border lit); clicking another gives it the sound.
// Each channel is its own connection to the provider: plans that allow
// fewer connections play only as many as they allow.

const hhmm = (d: Date | string) => new Date(d).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

function Tile({
  channel,
  heard,
  onHear,
  onPick,
  onRemove,
  onWatch,
}: {
  channel: LiveChannel | undefined
  heard: boolean
  onHear: () => void
  onPick: () => void
  onRemove: () => void
  onWatch: () => void
}) {
  const [video, setVideo] = useState<HTMLVideoElement | null>(null)
  const [play, setPlay] = useState<LivePlay | null>(null)
  const [problem, setProblem] = useState('')
  const id = channel?.id ?? ''
  useEffect(() => {
    setPlay(null)
    setProblem('')
    if (!id) return
    let current = true
    api
      .livePlay(id)
      .then((p) => current && setPlay(p))
      .catch((e) => current && setProblem(e instanceof Error ? e.message : String(e)))
    return () => {
      current = false
    }
  }, [id])
  const sources = useMemo(() => (play ? (play.alts?.length ? play.alts : [{ kind: 'hls' as const, url: play.url }]) : null), [play])
  useLiveStream(video, sources, setProblem)
  useEffect(() => {
    if (video) video.muted = !heard
  }, [video, heard])

  if (!channel) {
    return (
      <button className="wx-mv-tile empty" onClick={onPick}>
        <Icon name="plus" size={30} />
        <span>Add a channel</span>
      </button>
    )
  }
  return (
    <div className={`wx-mv-tile${heard ? ' heard' : ''}`} onClick={onHear} role="button" tabIndex={0} aria-label={`${channel.name}${heard ? ', sound on' : ': sound here'}`}>
      <video ref={setVideo} autoPlay playsInline muted={!heard} />
      {!play && !problem && <div className="wx-mv-note">Tuning in…</div>}
      {problem && <div className="wx-mv-note problem">{problem}</div>}
      <div className="wx-mv-label">
        <span className="wx-mv-logo">
          <ChannelLogo name={channel.name} src={channel.logo} />
        </span>
        <span className="wx-mv-text">
          <b>{channel.name}</b>
          {channel.now && (
            <small>
              {channel.now.title} · until {hhmm(channel.now.stop)}
            </small>
          )}
        </span>
        {heard && (
          <span className="wx-mv-sound" title="This one has the sound">
            <Icon name="volume" size={18} />
          </span>
        )}
      </div>
      <div className="wx-mv-tools" onClick={(e) => e.stopPropagation()}>
        <button onClick={onWatch} aria-label={`Watch ${channel.name} on its own`} title="Watch this one on its own">
          <Icon name="maximize" size={16} />
        </button>
        <button onClick={onPick} aria-label="Change channel" title="Change channel">
          <Icon name="refresh" size={16} />
        </button>
        <button onClick={onRemove} aria-label={`Take ${channel.name} out`} title="Take it out">
          <Icon name="x" size={16} />
        </button>
      </div>
    </div>
  )
}

export default function Multiview({
  channels,
  first,
  onClose,
  onWatch,
}: {
  channels: LiveChannel[]
  first: string[]
  onClose: () => void
  onWatch: (id: string) => void
}) {
  const [size, setSize] = useState<2 | 4>(first.length > 2 ? 4 : 2)
  const [slots, setSlots] = useState<string[]>(() => [...first, '', '', '', ''].slice(0, 4))
  const [heard, setHeard] = useState(0)
  const [picking, setPicking] = useState<number | null>(null)
  const [q, setQ] = useState('')
  const byId = useMemo(() => new Map(channels.map((c) => [c.id, c])), [channels])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (picking !== null) setPicking(null)
        else onClose()
      }
      // 1–4 give that tile the sound.
      const n = Number(e.key)
      if (picking === null && n >= 1 && n <= size) setHeard(n - 1)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [picking, size, onClose])

  const setSlot = (i: number, id: string) => setSlots((s) => s.map((x, j) => (j === i ? id : x)))
  const options = useMemo(() => {
    const words = q.trim().toLowerCase().split(/\s+/).filter(Boolean)
    return channels
      .filter((c) => !c.divider && !slots.includes(c.id))
      .filter((c) => words.every((w) => `${c.num} ${c.name} ${c.now?.title ?? ''}`.toLowerCase().includes(w)))
      .slice(0, 60)
  }, [channels, q, slots])

  return (
    <div className="wx-mv" role="dialog" aria-label="Multiview">
      <div className="wx-mv-bar">
        <button className="wx-btn small" onClick={onClose}>
          <Icon name="arrow-left" size={16} /> Back
        </button>
        <b>Multiview</b>
        <div className="wx-mv-sizes" role="group" aria-label="How many">
          <button className="wx-btn small" aria-pressed={size === 2} onClick={() => (setSize(2), setHeard((h) => Math.min(h, 1)))}>
            2
          </button>
          <button className="wx-btn small" aria-pressed={size === 4} onClick={() => setSize(4)}>
            4
          </button>
        </div>
        <small className="wx-dim">Click a channel for its sound. Each one is a connection to your provider.</small>
      </div>
      <div className={`wx-mv-grid n${size}`}>
        {slots.slice(0, size).map((id, i) => (
          <Tile
            key={i}
            channel={id ? byId.get(id) : undefined}
            heard={heard === i}
            onHear={() => setHeard(i)}
            onPick={() => (setQ(''), setPicking(i))}
            onRemove={() => setSlot(i, '')}
            onWatch={() => onWatch(id)}
          />
        ))}
      </div>
      {picking !== null && (
        <div className="wx-mv-picker" onClick={() => setPicking(null)}>
          <div className="wx-mv-picker-box" onClick={(e) => e.stopPropagation()}>
            <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Find a channel or show" autoFocus aria-label="Find a channel" />
            <div className="wx-mv-options">
              {options.map((c) => (
                <button
                  key={c.id}
                  onClick={() => {
                    setSlot(picking, c.id)
                    setPicking(null)
                  }}
                >
                  <span className="wx-mv-logo">
                    <ChannelLogo name={c.name} src={c.logo} />
                  </span>
                  <span className="wx-mv-text">
                    <b>
                      {c.num ? `${c.num}  ` : ''}
                      {c.name}
                    </b>
                    {c.now && <small>{c.now.title}</small>}
                  </span>
                </button>
              ))}
              {options.length === 0 && <p className="wx-dim">No channels like that.</p>}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
