import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, profileToken, type LiveChannel, type LiveChannels, type LivePlay, type LiveProgramme } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import ChannelLogo from './ChannelLogo'
import { recentChannels, rememberChannel, useLiveStream } from './liveStream'
import { useReminders } from './reminders'
import { Spinner } from './parts'
import { useProfiles } from './profiles'
import { savedTheme } from './theme'
import { onTV, type TVPlayerResult } from './tv'
import VideoControls from './VideoControls'

// How the guide is drawn: pixels per minute, how far ahead it reaches, and
// how many channels are drawn at a time (more as you scroll).
const PPM = 5
const HOURS = 6
const PAGE = 40

// The channel list order for a group: "fav" is the profile's favourites,
// "recent" the channels last watched on this device, "all" everything, and
// anything else a provider group id.
export function channelsIn(data: LiveChannels | null, cat: string): LiveChannel[] {
  const all = data?.channels ?? []
  if (cat === 'fav') return all.filter((c) => c.favorite)
  if (cat === 'recent') {
    const byId = new Map(all.map((c) => [c.id, c]))
    return recentChannels()
      .map((id) => byId.get(id))
      .filter((c): c is LiveChannel => !!c)
  }
  if (cat === 'all' || !cat) return all
  return all.filter((c) => c.category === cat)
}

// liveHref opens a channel full screen (from search, for one).
export function liveHref(id: string, cat: string): string {
  return `/watch/live?cat=${encodeURIComponent(cat)}&ch=${encodeURIComponent(id)}&full=1`
}

const hhmm = (d: Date | string) => new Date(d).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

// when says the day and time of a show: "Today 7:30 PM", "Tomorrow 1:00 PM".
function when(iso: string): string {
  const d = new Date(iso)
  const days = Math.round((new Date(d).setHours(0, 0, 0, 0) - new Date().setHours(0, 0, 0, 0)) / 86_400_000)
  const day = days === 0 ? 'Today' : days === 1 ? 'Tomorrow' : days === -1 ? 'Yesterday' : d.toLocaleDateString([], { weekday: 'long' })
  return `${day} ${hhmm(d)}`
}

// canCatchup says a show that has started can be played from its start: the
// provider keeps the channel, and the show is recent enough.
function canCatchup(c: LiveChannel, show: LiveProgramme): boolean {
  const days = c.catchupDays ?? 0
  const s = Date.parse(show.start)
  return days > 0 && s <= Date.now() && s > Date.now() - days * 86_400_000
}

// Live TV, laid out like a TV box's guide:
//
//   the channel playing (a window at the top left) | what's on it now and next
//   the groups (a side menu)                         | the guide
//
// Picking a channel in the guide tunes the window to it, so you can flip
// through channels without leaving the guide; picking the one playing (or
// Full screen) fills the screen, and Back returns to the guide with it still
// playing. On a TV, full screen is the app's own player.
export default function LivePage() {
  const navigate = useNavigate()
  const { main, active } = useProfiles()
  const [params, setParams] = useSearchParams()
  const [data, setData] = useState<LiveChannels | null>(null)
  const [error, setError] = useState('')
  const [shown, setShown] = useState(PAGE)
  const [find, setFind] = useState('')
  const [guide, setGuide] = useState<Record<string, LiveProgramme[]>>({})
  const asked = useRef(new Set<string>())
  const [now, setNow] = useState(() => Date.now())
  const scroller = useRef<HTMLDivElement>(null)
  const more = useRef<HTMLDivElement>(null)

  // What's playing: the channel (?ch=) and whether it fills the screen.
  const ch = params.get('ch') ?? ''
  const full = params.get('full') === '1'
  // Catch-up: a show that has aired, from its start (?at= and ?end=, ms).
  const at = Number(params.get('at') ?? 0)
  const atEnd = Number(params.get('end') ?? 0)
  const catchup = at > 0 && atEnd > at
  // Where in the catch-up show the stream starts, in seconds (moved by
  // scrubbing: providers start a catch-up stream at a whole minute).
  const [cuOffset, setCuOffset] = useState(0)
  const atRef = useRef(at)
  const atEndRef = useRef(atEnd)
  const cuOffsetRef = useRef(cuOffset)
  atRef.current = at
  atEndRef.current = atEnd
  cuOffsetRef.current = cuOffset
  // A show picked that hasn't started: its details, with Remind me.
  const [picked, setPicked] = useState<{ channel: LiveChannel; show: LiveProgramme } | null>(null)
  // How far the guide is moved back or on, in 3-hour steps.
  const [shift, setShift] = useState(0)
  const reminders = useReminders()
  const [play, setPlay] = useState<LivePlay | null>(null)
  const [playError, setPlayError] = useState('')
  const [videoEl, setVideoEl] = useState<HTMLVideoElement | null>(null)
  const videoRef = useCallback((el: HTMLVideoElement | null) => setVideoEl(el), [])
  // On a TV, while the app's own player has the channel, the window rests
  // (one connection at a time).
  const [onTVPlayer, setOnTVPlayer] = useState(false)
  const nativeNext = useRef(false)

  // The guide starts on the half hour just gone, and moves on with it.
  const start = useMemo(() => {
    const d = new Date(now)
    d.setMinutes(d.getMinutes() < 30 ? 0 : 30, 0, 0)
    return d.getTime() + shift * 3 * 3_600_000
    // Moves only on the half hour, so the grid doesn't jump.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Math.floor(now / 1_800_000), shift])
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
  const recentCount = useMemo(() => channelsIn(data, 'recent').length, [data, ch]) // eslint-disable-line react-hooks/exhaustive-deps
  const cat = params.get('cat') ?? (favCount > 0 ? 'fav' : 'all')
  const list = useMemo(() => channelsIn(data, cat), [data, cat])
  const visible = list.slice(0, shown)
  const channel = data?.channels?.find((c) => c.id === ch)
  useDocumentTitle(channel ? channel.name : 'Live TV')

  useEffect(() => {
    setShown(PAGE)
    scroller.current?.scrollTo({ top: 0 })
  }, [cat])

  // Tune: look the channel's address up, and remember it as recent. Moving
  // within a catch-up show (cuOffset) asks again from there, without
  // clearing the picture first.
  const tuned = useRef('')
  useEffect(() => {
    const key = `${ch}|${at}|${atEnd}`
    if (tuned.current !== key) {
      setPlay(null)
      tuned.current = key
    }
    setPlayError('')
    if (!ch) return
    rememberChannel(ch)
    let current = true
    ;(catchup ? api.liveCatchup(ch, at + cuOffset * 1000, atEnd) : api.livePlay(ch))
      .then((p) => current && setPlay(p))
      .catch((e) => current && setPlayError(e instanceof Error ? e.message : String(e)))
    return () => {
      current = false
    }
  }, [ch, catchup, at, atEnd, cuOffset])

  // The stream, in the window (or full screen: the same video, so going
  // full screen and back never restarts it).
  const sources = useMemo(() => (play && !onTVPlayer ? (play.alts?.length ? play.alts : [{ kind: 'hls' as const, url: play.url }]) : null), [play, onTVPlayer])
  useLiveStream(videoEl, sources, setPlayError)

  const setPlaying = useCallback(
    (id: string, opts: { full?: boolean; push?: boolean; from?: LiveProgramme; range?: { at: number; end: number } } = {}) => {
      const next: Record<string, string> = { cat, ch: id }
      if (opts.full) next.full = '1'
      const range = opts.from ? { at: Date.parse(opts.from.start), end: Date.parse(opts.from.stop) } : opts.range
      if (range) {
        next.at = String(range.at)
        next.end = String(range.end)
      }
      // A different show (or live) starts at its beginning.
      if (!range || range.at !== at || id !== ch) setCuOffset(0)
      setPicked(null)
      setParams(next, { replace: !opts.push })
    },
    [cat, setParams, at, ch],
  )

  // On a TV, full screen is the app's player, straight from the provider.
  const openTVPlayer = useCallback(
    (p: LivePlay, c: LiveChannel | undefined) => {
      setOnTVPlayer(true)
      window.cueTvPlayerDone = (r: TVPlayerResult) => {
        window.cueTvPlayerDone = undefined
        setOnTVPlayer(false)
        if (r.reason === 'channel-up' || r.reason === 'channel-down') {
          if (list.length < 2) return
          const i = list.findIndex((x) => x.id === p.id)
          const n = list[((i < 0 ? 0 : i) + (r.reason === 'channel-up' ? 1 : -1) + list.length) % list.length]
          nativeNext.current = true
          setPlaying(n.id)
          return
        }
        if (r.reason === 'error') setPlayError(`This channel won't play.${r.message ? ' ' + r.message : ''}`)
      }
      window.CueTV?.play(
        JSON.stringify({
          url: p.direct || new URL(p.url, window.location.href).toString(),
          alts: p.directAlts ?? [],
          title: `${p.num ? p.num + '  ' : ''}${p.name}`,
          subtitle: p.catchup ? 'Catch-up' : c?.now ? `${c.now.title} · until ${hhmm(c.now.stop)}` : '',
          live: !p.catchup,
          catchup: !!p.catchup,
          // For moving through a catch-up show: the TV asks Cue for the
          // stream from a new minute, like the browser does.
          id: p.id,
          showStart: p.catchup ? atRef.current : 0,
          showStop: p.catchup ? atEndRef.current : 0,
          offsetSec: p.catchup ? cuOffsetRef.current : 0,
          token: profileToken(),
          accent: savedTheme().accent, // the profile's color, for the player
        }),
      )
    },
    [list, setPlaying],
  )
  // After changing channel in the TV's player, it opens again on the new one.
  useEffect(() => {
    if (onTV() && play && nativeNext.current) {
      nativeNext.current = false
      openTVPlayer(play, channel)
    }
  }, [play, channel, openTVPlayer])
  // Opened full screen (from search) on a TV: straight to the app's player.
  useEffect(() => {
    if (onTV() && full && play && !onTVPlayer) {
      setPlaying(play.id)
      openTVPlayer(play, channel)
    }
    // Once per channel.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [play])

  const goFull = useCallback(() => {
    if (!ch) return
    if (onTV()) {
      if (play) openTVPlayer(play, channel)
      return
    }
    setPlaying(ch, { full: true, push: true, range: catchup ? { at, end: atEnd } : undefined })
  }, [ch, play, channel, openTVPlayer, setPlaying, catchup, at, atEnd])

  const exitFull = useCallback(() => {
    if (window.history.state?.idx > 0) navigate(-1)
    else setPlaying(ch)
  }, [navigate, setPlaying, ch])

  // Pick in the guide: a channel, or a show on now, tunes the window (the
  // one playing again: full screen). A show that has aired plays from its
  // start when the provider keeps the channel; one that hasn't started shows
  // its details, with Remind me.
  const pick = (c: LiveChannel, show?: LiveProgramme) => {
    if (show) {
      const s = Date.parse(show.start)
      const e = Date.parse(show.stop)
      if (s > Date.now()) {
        setPicked({ channel: c, show })
        return
      }
      if (e <= Date.now()) {
        if (canCatchup(c, show)) setPlaying(c.id, { from: show })
        return
      }
    }
    if (c.id === ch && play && !catchup) goFull()
    else setPlaying(c.id)
  }

  const zap = useCallback(
    (step: number) => {
      if (list.length < 2) return
      const i = list.findIndex((c) => c.id === ch)
      const n = list[((i < 0 ? 0 : i) + step + list.length) % list.length]
      setPlaying(n.id, { full })
    },
    [list, ch, full, setPlaying],
  )

  // Full screen in a browser: up and down change channel, Escape leaves.
  useEffect(() => {
    if (!full || onTV()) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.fullscreenElement) exitFull()
      else if (e.key === 'ArrowUp' || e.key === 'PageUp' || e.key === 'ChannelUp') {
        e.preventDefault()
        zap(1)
      } else if (e.key === 'ArrowDown' || e.key === 'PageDown' || e.key === 'ChannelDown') {
        e.preventDefault()
        zap(-1)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [full, zap, exitFull])

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

  const counts = new Map<string, number>()
  for (const c of data.channels ?? []) counts.set(c.category, (counts.get(c.category) ?? 0) + 1)
  const cats = (data.categories ?? []).filter((c) => counts.has(c.id))
  const groups = [
    ...(recentCount > 0 ? [{ id: 'recent', name: 'Recent', count: recentCount, fixed: true }] : []),
    ...(favCount > 0 ? [{ id: 'fav', name: 'Favourites', count: favCount, fixed: true }] : []),
    { id: 'all', name: 'All channels', count: data.channels?.length ?? 0, fixed: true },
    ...cats.map((c) => ({ id: c.id, name: c.name, count: counts.get(c.id) ?? 0, fixed: false })),
  ]
  const findText = find.trim().toLowerCase()
  // How far back the guide goes: the longest catch-up of any channel.
  const maxBack = Math.min(3, Math.max(0, ...(data.channels ?? []).map((c) => c.catchupDays ?? 0))) * 86_400_000
  const slots: number[] = []
  for (let t = start; t < end; t += 30 * 60_000) slots.push(t)
  const width = HOURS * 60 * PPM
  const last = recentChannels().map((id) => data.channels?.find((c) => c.id === id)).find(Boolean)

  const nowShow = channel?.now
  const catchupShow = catchup ? (guide[ch] ?? []).find((p) => Date.parse(p.start) === at) ?? (nowShow && Date.parse(nowShow.start) === at ? nowShow : undefined) : undefined
  const progress = nowShow ? Math.min(1, Math.max(0, (now - Date.parse(nowShow.start)) / (Date.parse(nowShow.stop) - Date.parse(nowShow.start)))) : 0
  const browserFull = full && !onTV()

  return (
    <div className={`wx-page wx-live${browserFull ? ' is-full' : ''}`}>
      {/* The channel playing, and what's on it. */}
      <section className="wx-live-top" aria-label="Now playing">
        <div className={`wx-live-preview${browserFull ? ' is-full' : ''}`}>
          {ch && !onTVPlayer && <video ref={videoRef} autoPlay playsInline onClick={() => !browserFull && goFull()} />}
          {!ch && (
            <div className="wx-live-empty">
              <Icon name="tv" size={34} />
              <p>Pick a channel in the guide to start watching.</p>
              {last && (
                <button className="wx-btn small" onClick={() => setPlaying(last.id)}>
                  <Icon name="play" size={14} /> {last.name}
                </button>
              )}
            </div>
          )}
          {ch && onTVPlayer && (
            <div className="wx-live-empty">
              <p>Playing full screen</p>
            </div>
          )}
          {ch && !play && !playError && !onTVPlayer && <div className="wx-live-tuning">Tuning in…</div>}
          {playError && <div className="wx-live-problem">{playError}</div>}
          {ch && play && !browserFull && !onTVPlayer && (
            <button className="wx-live-expand" onClick={goFull} aria-label="Full screen">
              <Icon name="maximize" size={18} />
            </button>
          )}
          {browserFull && play && (
            <VideoControls
              video={videoEl}
              live={!catchup}
              timeline={
                catchup
                  ? { offset: cuOffset, duration: (atEnd - at) / 1000, onSeek: (sec) => setCuOffset(Math.max(0, Math.floor(sec / 60) * 60)) }
                  : undefined
              }
              title={catchup && catchupShow ? catchupShow.title : `${play.num ? play.num + '  ' : ''}${play.name}`}
              subtitle={catchup ? `${play.name} · catch-up from ${when(new Date(at).toISOString())}` : nowShow ? `${nowShow.title} · until ${hhmm(nowShow.stop)}` : undefined}
              onBack={exitFull}
              actions={
                <>
                  <ChannelLogo className="wx-live-logo" name={play.name} src={play.logo || channel?.logo} />
                  {list.length > 1 && !catchup && (
                    <>
                      <button className="vx-text-btn" onClick={() => zap(-1)} aria-label="Previous channel">
                        <Icon name="chevron-down" size={22} />
                        <span>Ch</span>
                      </button>
                      <button className="vx-text-btn" onClick={() => zap(1)} aria-label="Next channel">
                        <Icon name="chevron-up" size={22} />
                        <span>Ch</span>
                      </button>
                    </>
                  )}
                </>
              }
            />
          )}
        </div>

        <div className="wx-live-info">
          {picked ? (
            // A show that hasn't started: when, what, and Remind me.
            <>
              <div className="wx-live-info-chan">
                <span className="wx-live-info-logo">
                  <ChannelLogo name={picked.channel.name} src={picked.channel.logo} />
                </span>
                <span>
                  <small>{picked.channel.num ? `Channel ${picked.channel.num}` : 'Channel'}</small>
                  <b>{picked.channel.name}</b>
                </span>
                <span className="wx-live-badge later">COMING UP</span>
              </div>
              <h2>{picked.show.title}</h2>
              <div className="wx-live-time">
                <span>
                  {when(picked.show.start)} – {hhmm(picked.show.stop)}
                </span>
              </div>
              {picked.show.desc && <p className="wx-live-desc">{picked.show.desc}</p>}
              <div className="wx-live-actions">
                <button className="wx-btn play small" autoFocus={onTV()} onClick={() => void reminders.toggle(picked.channel.id, picked.show)}>
                  <Icon name={reminders.has(picked.channel.id, picked.show.start) ? 'check' : 'clock'} size={16} />
                  {reminders.has(picked.channel.id, picked.show.start) ? 'Reminder set' : 'Remind me'}
                </button>
                <button className="wx-btn small" onClick={() => setPlaying(picked.channel.id)}>
                  <Icon name="play" size={16} /> Watch {picked.channel.name} now
                </button>
                <button className="wx-btn small" onClick={() => setPicked(null)}>
                  Close
                </button>
              </div>
            </>
          ) : channel ? (
            <>
              <div className="wx-live-info-chan">
                <span className="wx-live-info-logo">
                  <ChannelLogo name={channel.name} src={channel.logo} />
                </span>
                <span>
                  <small>{channel.num ? `Channel ${channel.num}` : 'Channel'}</small>
                  <b>{channel.name}</b>
                </span>
                {catchup ? (
                  <span className="wx-live-badge again">
                    <Icon name="refresh" size={11} /> CATCH-UP
                  </span>
                ) : (
                  <span className="wx-live-badge">
                    <i /> LIVE
                  </span>
                )}
              </div>
              {catchup ? (
                <>
                  <h2>{catchupShow?.title ?? 'A past show'}</h2>
                  <div className="wx-live-time">
                    <span>
                      {when(new Date(at).toISOString())} – {hhmm(new Date(atEnd))}
                    </span>
                  </div>
                  {catchupShow?.desc && <p className="wx-live-desc">{catchupShow.desc}</p>}
                </>
              ) : nowShow ? (
                <>
                  <h2>{nowShow.title}</h2>
                  <div className="wx-live-time">
                    <span>{hhmm(nowShow.start)}</span>
                    <div className="wx-live-progress">
                      <div style={{ width: `${progress * 100}%` }} />
                    </div>
                    <span>{hhmm(nowShow.stop)}</span>
                  </div>
                  {nowShow.desc && <p className="wx-live-desc">{nowShow.desc}</p>}
                </>
              ) : (
                <h2>{data.guideReady ? 'No guide for this channel' : 'The TV guide is loading…'}</h2>
              )}
              {!catchup && channel.next && (
                <p className="wx-live-next">
                  <b>Next</b> {hhmm(channel.next.start)} · {channel.next.title}
                </p>
              )}
              <div className="wx-live-actions">
                <button className="wx-btn play small" onClick={goFull} disabled={!play}>
                  <Icon name="maximize" size={16} /> Full screen
                </button>
                {catchup ? (
                  <button className="wx-btn small" onClick={() => setPlaying(channel.id)}>
                    <Icon name="tv" size={16} /> Back to live
                  </button>
                ) : (
                  nowShow &&
                  canCatchup(channel, nowShow) && (
                    <button className="wx-btn small" onClick={() => setPlaying(channel.id, { from: nowShow })}>
                      <Icon name="refresh" size={16} /> From the start
                    </button>
                  )
                )}
                <button className="wx-btn small" onClick={() => void toggleFavorite(channel)} aria-pressed={channel.favorite}>
                  <Icon name="star" size={16} /> {channel.favorite ? 'Favourite' : 'Add to Favourites'}
                </button>
              </div>
            </>
          ) : (
            <>
              <h2>Live TV</h2>
              <p className="wx-dim">
                {data.channels?.length ?? 0} channels. Pick one to watch it here while you browse the guide; pick it again, or Full screen, to fill the screen.
              </p>
              {!data.guideReady && <p className="wx-dim">The TV guide is loading; channels play already.</p>}
            </>
          )}
        </div>
      </section>

      <div className="wx-live-body">
        {/* The groups: a side menu (on a phone, a picker). */}
        <nav className="wx-live-side" aria-label="Channel groups">
          {cats.length > 12 && (
            <input className="wx-live-find" value={find} onChange={(e) => setFind(e.target.value)} placeholder="Find a group" aria-label="Find a group" />
          )}
          <ul>
            {groups
              .filter((g) => !findText || g.name.toLowerCase().includes(findText) || g.fixed)
              .map((g) => (
                <li key={g.id}>
                  <button
                    className={cat === g.id ? 'active' : ''}
                    aria-current={cat === g.id ? 'true' : undefined}
                    onClick={() => setParams(ch ? { cat: g.id, ch } : { cat: g.id }, { replace: true })}
                  >
                    {g.id === 'fav' && <Icon name="star" size={15} />}
                    {g.id === 'recent' && <Icon name="clock" size={15} />}
                    <span>{g.name}</span>
                    <small>{g.count}</small>
                  </button>
                </li>
              ))}
          </ul>
        </nav>
        <label className="wx-live-select">
          <span className="sr-only">Channel group</span>
          <select value={cat} onChange={(e) => setParams(ch ? { cat: e.target.value, ch } : { cat: e.target.value }, { replace: true })}>
            {groups.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name} ({g.count})
              </option>
            ))}
          </select>
        </label>

        <div className="wx-live-main">
          {list.length === 0 && <p className="wx-dim">No channels here. Star a channel to add it to Favourites.</p>}
          {list.length > 0 && (
            <div className="wx-guide-bar">
              <button className="wx-btn small" onClick={() => setShift((n) => n - 1)} disabled={start - 3 * 3_600_000 < Date.now() - maxBack}>
                <Icon name="chevron-left" size={16} /> Earlier
              </button>
              <button className="wx-btn small" onClick={() => setShift(0)} disabled={shift === 0}>
                Now
              </button>
              <button className="wx-btn small" onClick={() => setShift((n) => n + 1)} disabled={shift >= 8}>
                Later <Icon name="chevron-right" size={16} />
              </button>
              <span className="wx-dim">{when(new Date(start).toISOString())}</span>
              {maxBack > 0 && shift === 0 && <span className="wx-dim wx-guide-bar-hint">⟲ shows on channels that keep them can be watched again</span>}
            </div>
          )}
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
                  const playing = c.id === ch
                  return (
                    <div className={`wx-guide-row${playing ? ' playing' : ''}`} key={c.id}>
                      <div className="wx-guide-chan">
                        <button className="wx-guide-chan-play" onClick={() => pick(c)} title={playing ? `${c.name}: full screen` : `Watch ${c.name}`}>
                          <span className="wx-guide-logo">
                            <ChannelLogo name={c.name} src={c.logo} />
                          </span>
                          <span className="wx-guide-name">
                            <small>{playing ? '▶ Playing' : c.num || ''}</small>
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
                          <button className="wx-prog none" style={{ left: 0, width }} onClick={() => pick(c)}>
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
                          const again = e <= now && canCatchup(c, p)
                          const reminded = s > now && reminders.has(c.id, p.start)
                          const isPicked = picked?.channel.id === c.id && picked.show.start === p.start
                          const isCatchupPlaying = catchup && playing && Date.parse(p.start) === at
                          return (
                            <button
                              key={p.start}
                              className={`wx-prog${onNow ? ' now' : ''}${e <= now ? ' past' : ''}${again ? ' again' : ''}${isPicked || isCatchupPlaying ? ' picked' : ''}`}
                              style={{ left, width: w }}
                              title={`${p.title} · ${hhmm(new Date(s))}–${hhmm(new Date(e))}${p.desc ? `\n${p.desc}` : ''}`}
                              tabIndex={w < 30 ? -1 : 0}
                              onClick={() => pick(c, p)}
                            >
                              <span className="wx-prog-text">
                                <b>
                                  {again && <Icon name="refresh" size={12} />}
                                  {reminded && <Icon name="clock" size={12} />}
                                  {p.title}
                                </b>
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
      </div>
    </div>
  )
}
