import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, type LiveChannel, type LiveChannels, type LiveProgramme } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { Spinner } from './parts'
import { useProfiles } from './profiles'

// How the guide is drawn: pixels per minute, how far back and ahead it
// reaches, and how many channels are drawn at a time (more as you scroll).
const PPM = 5
const HOURS = 6
const PAGE = 40

// The channel list order for a category: "fav" is the profile's favourites,
// "all" everything, anything else a provider category id.
export function channelsIn(data: LiveChannels | null, cat: string): LiveChannel[] {
  const all = data?.channels ?? []
  if (cat === 'fav') return all.filter((c) => c.favorite)
  if (cat === 'all' || !cat) return all
  return all.filter((c) => c.category === cat)
}

export function liveHref(id: string, cat: string): string {
  return `/watch/live/play/${encodeURIComponent(id)}?cat=${encodeURIComponent(cat)}`
}

const hhmm = (d: Date) => d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

// Live TV: the provider's channels as a TV guide. Logos down the left, what's
// on across a timeline, a line at the time now. Pick a channel or a show on
// now to watch it.
export default function LivePage() {
  useDocumentTitle('Live TV')
  const navigate = useNavigate()
  const { main, active } = useProfiles()
  const [params, setParams] = useSearchParams()
  const [data, setData] = useState<LiveChannels | null>(null)
  const [error, setError] = useState('')
  const [shown, setShown] = useState(PAGE)
  const [guide, setGuide] = useState<Record<string, LiveProgramme[]>>({})
  const asked = useRef(new Set<string>())
  const [now, setNow] = useState(() => Date.now())
  const scroller = useRef<HTMLDivElement>(null)
  const more = useRef<HTMLDivElement>(null)

  // The guide starts on the half hour just gone, and moves on with it.
  const start = useMemo(() => {
    const d = new Date(now)
    d.setMinutes(d.getMinutes() < 30 ? 0 : 30, 0, 0)
    return d.getTime()
    // Moves only on the half hour, so the grid doesn't jump.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Math.floor(now / 1_800_000)])
  const end = start + HOURS * 3_600_000
  const x = (t: number) => ((Math.max(t, start) - start) / 60_000) * PPM

  const load = useCallback(() => {
    api
      .liveChannels()
      .then((d) => {
        setData(d)
        setError('')
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])
  useEffect(load, [load])

  // The clock, and the guide once it has loaded on the server.
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 30_000)
    return () => clearInterval(t)
  }, [])
  useEffect(() => {
    if (!data?.configured || data.guideReady) return
    const t = setTimeout(load, 8000)
    return () => clearTimeout(t)
  }, [data, load])

  const favCount = data?.channels?.filter((c) => c.favorite).length ?? 0
  const cat = params.get('cat') ?? (favCount > 0 ? 'fav' : 'all')
  const list = useMemo(() => channelsIn(data, cat), [data, cat])
  const visible = list.slice(0, shown)

  useEffect(() => {
    setShown(PAGE)
    scroller.current?.scrollTo({ top: 0 })
  }, [cat])

  // Fetch the guide for channels as they come into view.
  useEffect(() => {
    if (!data?.guideReady) return
    const want = visible.map((c) => c.id).filter((id) => !asked.current.has(id))
    if (want.length === 0) return
    want.forEach((id) => asked.current.add(id))
    api
      .liveGuide(want, start, HOURS)
      .then((g) => setGuide((old) => ({ ...old, ...g.programmes })))
      .catch(() => want.forEach((id) => asked.current.delete(id)))
  }, [visible, data?.guideReady, start])
  useEffect(() => {
    asked.current.clear()
    setGuide({})
  }, [start])

  // More rows as the end of the list scrolls into view.
  useEffect(() => {
    const el = more.current
    if (!el) return
    const io = new IntersectionObserver((e) => {
      if (e.some((x) => x.isIntersecting)) setShown((n) => n + PAGE)
    })
    io.observe(el)
    return () => io.disconnect()
  }, [list.length, shown])

  async function toggleFavorite(c: LiveChannel) {
    const on = !c.favorite
    setData((d) => d && { ...d, channels: d.channels?.map((x) => (x.id === c.id ? { ...x, favorite: on } : x)) })
    try {
      await api.setLiveFavorite(c.id, on)
    } catch {
      setData((d) => d && { ...d, channels: d.channels?.map((x) => (x.id === c.id ? { ...x, favorite: !on } : x)) })
    }
  }

  if (error) {
    return (
      <div className="wx-page">
        <h2 style={{ marginTop: 0 }}>Live TV</h2>
        <p className="wx-error">{error}</p>
        <button className="wx-btn" onClick={load}>
          Try again
        </button>
      </div>
    )
  }
  if (!data) return <Spinner label="Loading the channels…" />
  if (!data.configured) {
    return (
      <div className="wx-page">
        <h2 style={{ marginTop: 0 }}>Live TV</h2>
        <p className="wx-dim">Live TV shows your IPTV provider&apos;s channels and TV guide.</p>
        {main && active?.id === main.id ? (
          <Link className="wx-btn" to="/settings/streaming">
            Add your IPTV login in Settings
          </Link>
        ) : (
          <p className="wx-dim">Ask {main?.name ?? 'the account owner'} to add the IPTV login in Settings.</p>
        )}
      </div>
    )
  }

  const cats = (data.categories ?? []).filter((c) => data.channels?.some((ch) => ch.category === c.id))
  const slots: number[] = []
  for (let t = start; t < end; t += 30 * 60_000) slots.push(t)
  const width = HOURS * 60 * PPM

  return (
    <div className="wx-page wx-live">
      <div className="wx-live-head">
        <h2>Live TV</h2>
        {!data.guideReady && <span className="wx-dim">The TV guide is loading; channels play already.</span>}
      </div>
      <div className="wx-chips" role="tablist" aria-label="Channel groups">
        {favCount > 0 && (
          <button role="tab" aria-selected={cat === 'fav'} className={cat === 'fav' ? 'active' : ''} onClick={() => setParams({ cat: 'fav' }, { replace: true })}>
            <Icon name="star" size={14} /> Favourites
          </button>
        )}
        <button role="tab" aria-selected={cat === 'all'} className={cat === 'all' ? 'active' : ''} onClick={() => setParams({ cat: 'all' }, { replace: true })}>
          All channels
        </button>
        {cats.map((c) => (
          <button key={c.id} role="tab" aria-selected={cat === c.id} className={cat === c.id ? 'active' : ''} onClick={() => setParams({ cat: c.id }, { replace: true })}>
            {c.name}
          </button>
        ))}
      </div>

      {list.length === 0 && <p className="wx-dim">No channels here. Star a channel to add it to Favourites.</p>}
      {list.length > 0 && (
        <div className="wx-guide" ref={scroller}>
          <div className="wx-guide-inner" style={{ width: `calc(var(--wx-chan-w) + ${width}px)` }}>
            <div className="wx-guide-times">
              <div className="wx-guide-corner">{hhmm(new Date(now))}</div>
              <div className="wx-guide-slots" style={{ width }}>
                {slots.map((t) => (
                  <span key={t} style={{ left: x(t) }}>
                    {hhmm(new Date(t))}
                  </span>
                ))}
              </div>
            </div>
            {visible.map((c) => {
              const progs = guide[c.id] ?? (c.now ? [c.now, ...(c.next ? [c.next] : [])] : [])
              return (
                <div className="wx-guide-row" key={c.id}>
                  <div className="wx-guide-chan">
                    <button className="wx-guide-chan-play" onClick={() => navigate(liveHref(c.id, cat))} title={`Watch ${c.name}`}>
                      <span className="wx-guide-logo">{c.logo ? <img src={c.logo} alt="" loading="lazy" /> : <Icon name="tv" size={20} />}</span>
                      <span className="wx-guide-name">
                        <small>{c.num || ''}</small>
                        {c.name}
                      </span>
                    </button>
                    <button
                      className={`wx-guide-star${c.favorite ? ' on' : ''}`}
                      onClick={() => void toggleFavorite(c)}
                      aria-label={c.favorite ? `Remove ${c.name} from Favourites` : `Add ${c.name} to Favourites`}
                      aria-pressed={c.favorite}
                    >
                      <Icon name="star" size={16} />
                    </button>
                  </div>
                  <div className="wx-guide-progs" style={{ width }}>
                    {progs.length === 0 && (
                      <button className="wx-prog none" style={{ left: 0, width }} onClick={() => navigate(liveHref(c.id, cat))}>
                        <b>{c.name}</b>
                        <small>{data.guideReady ? 'No guide for this channel' : ''}</small>
                      </button>
                    )}
                    {progs.map((p) => {
                      const s = Date.parse(p.start)
                      const e = Date.parse(p.stop)
                      if (e <= start || s >= end) return null
                      const onNow = s <= now && e > now
                      const left = x(s)
                      const w = Math.max(x(Math.min(e, end)) - left, 4)
                      return (
                        <button
                          key={p.start}
                          className={`wx-prog${onNow ? ' now' : ''}${e <= now ? ' past' : ''}`}
                          style={{ left, width: w }}
                          title={`${p.title} · ${hhmm(new Date(s))}–${hhmm(new Date(e))}${p.desc ? `\n${p.desc}` : ''}`}
                          tabIndex={w < 30 ? -1 : 0}
                          onClick={() => navigate(liveHref(c.id, cat))}
                        >
                            <span className="wx-prog-text">
                            <b>{p.title}</b>
                            <small>
                              {hhmm(new Date(s))}–{hhmm(new Date(e))}
                            </small>
                          </span>
                        </button>
                      )
                    })}
                  </div>
                </div>
              )
            })}
            <div className="wx-guide-now" style={{ left: `calc(var(--wx-chan-w) + ${x(now)}px)` }} aria-hidden="true" />
            {shown < list.length && <div ref={more} style={{ height: 40 }} />}
          </div>
        </div>
      )}
    </div>
  )
}
