import { useSyncExternalStore, type ReactNode } from 'react'
import type { LiveChannel, LiveProgramme } from '../api'
import ChannelLogo from './ChannelLogo'

// The top of the guide: whatever the pointer (or the remote's highlight) is
// on, big — the channel, the show, how long, when it started, what it's
// about and how far in — the way a TV box's guide shows the highlighted
// show above the grid. Its own little store, so pointing at shows redraws
// this and not the whole guide.

export type Peek = { channel: LiveChannel; show?: LiveProgramme } | null

export function peekStore() {
  let value: Peek = null
  const subs = new Set<() => void>()
  return {
    get: () => value,
    set(p: Peek) {
      if (p?.channel.id === value?.channel.id && p?.show?.start === value?.show?.start) return
      value = p
      subs.forEach((f) => f())
    },
    subscribe(f: () => void) {
      subs.add(f)
      return () => subs.delete(f)
    },
  }
}
export type PeekStore = ReturnType<typeof peekStore>

const hhmm = (d: Date | string) => new Date(d).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

function length(ms: number): string {
  const m = Math.round(ms / 60000)
  if (m < 60) return `${m}m`
  return m % 60 ? `${Math.floor(m / 60)}h ${m % 60}m` : `${m / 60}h`
}

function dayOf(d: Date): string {
  const days = Math.round((new Date(d).setHours(0, 0, 0, 0) - new Date().setHours(0, 0, 0, 0)) / 86_400_000)
  return days === 0 ? '' : days === 1 ? 'tomorrow ' : days === -1 ? 'yesterday ' : d.toLocaleDateString([], { weekday: 'long' }) + ' '
}

export function PeekInfo({
  store,
  fallback,
  first,
  guideReady,
  now,
}: {
  store: PeekStore
  // What shows when the pointer isn't on the guide: the channel playing.
  fallback: ReactNode
  // Nothing playing either: this channel (the top one on now).
  first?: LiveChannel
  guideReady?: boolean
  // The page's clock (it ticks), for how far in.
  now: number
}) {
  const peek = useSyncExternalStore(store.subscribe, store.get)
  if (!peek && fallback) return <>{fallback}</>
  const p = peek ?? (first ? { channel: first } : null)
  if (!p) return null
  const c = p.channel
  const show = p.show ?? c.now
  const s = show ? Date.parse(show.start) : 0
  const e = show ? Date.parse(show.stop) : 0
  const onNow = !!show && s <= now && e > now
  const past = !!show && e <= now
  const pct = onNow ? (now - s) / (e - s) : 0
  return (
    <div className="wx-peek">
      <div className="wx-peek-chan">
        <span className="wx-peek-logo">
          <ChannelLogo name={c.name} src={c.logo} />
        </span>
        <span>
          <b>{c.name}</b>
          {c.num ? <small>Channel {c.num}</small> : null}
        </span>
        {onNow && (
          <span className="wx-live-badge">
            <i /> LIVE
          </span>
        )}
      </div>
      <h2>{show?.title ?? (guideReady ? c.name : 'The TV guide is loading…')}</h2>
      {show && (
        <div className="wx-peek-meta">
          <span>{length(e - s)}</span>
          <span>
            {onNow
              ? `Started at ${hhmm(show.start)}`
              : past
                ? `Was on ${dayOf(new Date(s))}at ${hhmm(show.start)}`
                : `Starts ${dayOf(new Date(s))}at ${hhmm(show.start)}`}
          </span>
          {onNow && <span>{Math.max(0, Math.round((e - now) / 60000))}m left</span>}
        </div>
      )}
      {show?.desc && <p className="wx-peek-desc">{show.desc}</p>}
      {onNow && (
        <span className="wx-peek-bar">
          <i style={{ width: `${pct * 100}%` }} />
        </span>
      )}
      {!show && guideReady && <p className="wx-peek-desc">No guide for this channel.</p>}
    </div>
  )
}
