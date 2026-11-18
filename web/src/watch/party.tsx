import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type PartyMsg, type PartyNow, type PartyState, type PartyTitle } from '../api'
import Icon from '../components/Icon'
import { playHref } from './parts'
import { Avatar } from './profiles'

// Watch parties (the server side is party.go). Everyone plays their own
// stream; what's shared is where the party is and whether it's playing.
// Anyone's play, pause or jump is sent to the server and comes back to the
// others as a "state" event; each player follows it, and quietly corrects
// itself if it drifts more than a couple of seconds.

export const REACTIONS = ['😂', '😮', '😍', '😢', '👏', '🔥', '😱', '❤️']

// clientId tells this tab apart from the same person's other tabs.
function clientId(): string {
  try {
    let id = sessionStorage.getItem('cue.party.client')
    if (!id) {
      id = (crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`).replace(/[^a-zA-Z0-9-]/g, '')
      sessionStorage.setItem('cue.party.client', id)
    }
    return id
  } catch {
    return `tab-${Date.now()}`
  }
}

export function partyLink(code: string): string {
  return `${location.origin}/watch/party/${code}`
}

interface SyncOpts {
  code: string | null
  video: HTMLVideoElement | null
  title: PartyTitle
  // Where the picture is, in seconds into the title (a converted stream
  // starts part-way in), and how to move it.
  position: () => number
  seek: (sec: number) => void
  // The party moved to another episode: open it (with the party).
  follow: (t: PartyTitle) => void
  // Where to start once joined (before the video has loaded).
  startAt: (sec: number) => void
  // Only the chat (a phone beside a TV): nothing to keep in step.
  chatOnly?: boolean
  // Moving costs a restart (Cue converting the video as it plays: a jump
  // starts the conversion again), so only a real difference is worth one.
  costlySeek?: boolean
  // Whether a point can be reached without that restart (a converted video
  // that's already made that far): then a jump is cheap after all.
  canSeek?: (sec: number) => boolean
}

export interface Floating {
  id: number
  emoji: string
  name: string
  left: number
  sway: number // which way it drifts: -1 or 1
}

export function useParty(o: SyncOpts) {
  const client = useRef(clientId()).current
  const [state, setState] = useState<PartyState | null>(null)
  const [msgs, setMsgs] = useState<PartyMsg[]>([])
  const [floating, setFloating] = useState<Floating[]>([])
  const [problem, setProblem] = useState('')
  const [unread, setUnread] = useState(0)
  const [needsTap, setNeedsTap] = useState(false)
  // The episode this player last moved on from, and when.
  const left = useRef<{ season: number; episode: number; at: number } | null>(null)
  const stateRef = useRef<PartyState | null>(null)
  const got = useRef(0) // when the last state arrived (performance.now())
  const applying = useRef(0) // until when the player's own events are ours, not the viewer's
  // A converted video: when it last jumped by itself.
  const jumped = useRef({ at: -Infinity })
  const opts = useRef(o)
  opts.current = o
  const { code, video } = o

  // Bring this player in line with the party.
  const apply = useCallback((s: PartyState) => {
    const v = opts.current.video
    if (!v) return
    // Held (waiting for someone's player to load) counts as paused.
    const going = s.playing && !s.held
    const target = s.pos + (going ? (performance.now() - got.current) / 1000 : 0)
    const here = opts.current.position()
    // Paused, everyone stops on the same frame; playing, a second or so
    // either way is fine. A converted video starts again on every jump, so
    // it only jumps for a real difference (and never while it's loading).
    // Play and pause alone never restart it: it plays on from where it is.
    // When it does jump, it aims a few seconds ahead, for the time the new
    // stream takes to start, and ignores its own events until it's loaded.
    // A converted one that jumped by itself lately doesn't again for a
    // while, however far out it is: chasing the party with restarts that
    // each take longer than the gap only starts it over and over.
    // A converted video that has already made the point it needs jumps
    // there at once, like any other.
    const costly = !!opts.current.costlySeek && !opts.current.canSeek?.(target)
    if (costly && (v.readyState < 2 || v.seeking)) return
    // Small differences while playing are closed by speed (drift, below),
    // not by jumping.
    const slack = costly ? 12 : going ? 4 : 0.25
    const rested = performance.now() - jumped.current.at > 60_000
    if (Math.abs(here - target) > slack && (!costly || rested)) {
      applying.current = performance.now() + (costly ? 35_000 : 1200)
      if (costly) {
        // The party waits while it loads (wait/ready), so it aims where
        // the party is now.
        jumped.current = { at: performance.now() }
        opts.current.seek(target)
        return
      }
      opts.current.seek(target)
    }
    if (going && v.paused) {
      applying.current = performance.now() + 1200
      // A phone that wasn't tapped yet may refuse to start with sound.
      v.play()
        .then(() => setNeedsTap(false))
        .catch((e: unknown) => {
          if (e instanceof DOMException && e.name === 'NotAllowedError') setNeedsTap(true)
        })
    } else if (!going && !v.paused) {
      applying.current = performance.now() + 1200
      v.pause()
    }
  }, [])

  // Join, then listen.
  useEffect(() => {
    if (!code) return
    let es: EventSource | null = null
    let dead = false
    setProblem('')
    api
      .joinParty(code, client)
      .then((r) => {
        if (dead) return
        got.current = performance.now()
        stateRef.current = r.party
        setState(r.party)
        setMsgs(r.msgs)
        opts.current.startAt(r.party.pos)
        es = new EventSource(`/api/party/${encodeURIComponent(code)}/events?client=${encodeURIComponent(client)}`)
        es.addEventListener('state', (e) => {
          const s = JSON.parse((e as MessageEvent).data) as PartyState
          got.current = performance.now()
          const before = stateRef.current
          stateRef.current = s
          setState(s)
          const t = opts.current.title
          if (opts.current.chatOnly) return
          // Word still naming the episode this player just moved on from
          // (sent before the party heard about Next): not a reason to go
          // back to it, which would play the same episode over again.
          const l = left.current
          if (l && performance.now() - l.at < 15000 && s.title.tmdbId === t.tmdbId && (s.title.season ?? 0) === l.season && (s.title.episode ?? 0) === l.episode) return
          if (s.title.tmdbId !== t.tmdbId || (s.title.season ?? 0) !== (t.season ?? 0) || (s.title.episode ?? 0) !== (t.episode ?? 0)) {
            opts.current.follow(s.title)
            return
          }
          // Someone else did something, or this is the first word on opening.
          if (s.by !== client || !before) apply(s)
        })
        es.addEventListener('msg', (e) => {
          const m = JSON.parse((e as MessageEvent).data) as PartyMsg
          if (m.kind === 'reaction') {
            const f = { id: m.id, emoji: m.text, name: m.name ?? '', left: 6 + Math.random() * 34, sway: Math.random() < 0.5 ? -1 : 1 }
            setFloating((list) => [...list.slice(-20), f])
            window.setTimeout(() => setFloating((list) => list.filter((x) => x.id !== f.id)), 4300)
            return
          }
          setMsgs((list) => (list.some((x) => x.id === m.id) ? list : [...list.slice(-149), m]))
          if (m.kind === 'chat') setUnread((n) => n + 1)
        })
        es.onerror = () => {
          // EventSource reconnects by itself; a party that ended answers 404.
          void api.getParty(code).catch((err) => {
            if (!dead) setProblem(err instanceof Error ? err.message : String(err))
            es?.close()
          })
        }
      })
      .catch((e) => !dead && setProblem(e instanceof Error ? e.message : String(e)))
    return () => {
      dead = true
      es?.close()
    }
  }, [code, client, apply])

  // Leaving the player leaves the party.
  useEffect(() => {
    if (!code) return
    const bye = () => void api.partyLeave(code, client).catch(() => undefined)
    window.addEventListener('pagehide', bye)
    return () => {
      window.removeEventListener('pagehide', bye)
      bye()
    }
  }, [code, client])

  // What the viewer does is sent to the party.
  useEffect(() => {
    if (!code || !video) return
    const mine = () => performance.now() > applying.current
    const send = (action: 'play' | 'pause' | 'seek') => {
      // A converted video that is still catching up doesn't pull everyone
      // back to where it is on play or pause.
      let pos = opts.current.position()
      const s = stateRef.current
      if (opts.current.costlySeek && s && action !== 'seek') {
        const there = s.pos + (s.playing ? (performance.now() - got.current) / 1000 : 0)
        if (Math.abs(pos - there) > 15) pos = there
      }
      void api
        .partyState(code, { client, action, pos })
        .then((s) => {
          got.current = performance.now()
          stateRef.current = s
          setState(s)
        })
        .catch(() => undefined)
    }
    // A new stream loading (the start, a converted stream jumping) isn't
    // the viewer: give it a moment.
    const loaded = () => (applying.current = performance.now() + 2500)
    const onPlay = () => mine() && send('play')
    const onPause = () => mine() && !video.ended && send('pause')
    const onSeeked = () => mine() && send('seek')
    // Once loaded, line up with the party (only if it's really elsewhere:
    // a converted video's new stream already starts where it was asked).
    const onLoaded = () => {
      loaded()
      const s = stateRef.current
      if (s) apply(s)
    }
    video.addEventListener('loadedmetadata', onLoaded)
    video.addEventListener('play', onPlay)
    video.addEventListener('pause', onPause)
    video.addEventListener('seeked', onSeeked)
    return () => {
      video.removeEventListener('loadedmetadata', onLoaded)
      video.removeEventListener('play', onPlay)
      video.removeEventListener('pause', onPause)
      video.removeEventListener('seeked', onSeeked)
    }
  }, [code, video, client, apply])

  // Drift: a player a little behind or ahead catches up by playing a touch
  // faster or slower (nobody notices 5%), which needs no jump, so even a
  // converted video lines up to within a fraction of a second. Only a big
  // difference is jumped.
  useEffect(() => {
    if (!code) return
    const t = window.setInterval(() => {
      const s = stateRef.current
      const v = opts.current.video
      if (!v) return
      const steady = () => {
        if (v.playbackRate !== 1) v.playbackRate = 1
      }
      // Not while it's buffering or jumping: that would only make it worse.
      if (!s || !s.playing || s.held || v.paused || v.seeking || v.readyState < 3 || performance.now() < applying.current) return steady()
      const target = s.pos + (performance.now() - got.current) / 1000
      const off = opts.current.position() - target // above 0: ahead
      const costly = !!opts.current.costlySeek && !opts.current.canSeek?.(target)
      if (Math.abs(off) > (costly ? 12 : 4)) {
        steady()
        apply(s)
        return
      }
      if (Math.abs(off) < 0.2) return steady()
      // Speeding up a converted video only while it has some made ahead
      // (else it would just stall).
      let ahead = 0
      for (let i = 0; i < v.buffered.length; i++) if (v.buffered.start(i) <= v.currentTime + 0.5) ahead = Math.max(ahead, v.buffered.end(i) - v.currentTime)
      if (off < 0 && ahead < 3) return steady()
      const step = Math.abs(off) > 2 && !costly ? 0.1 : 0.05
      v.playbackRate = off < 0 ? 1 + step : 1 - step
    }, 1000)
    return () => {
      window.clearInterval(t)
      const v = opts.current.video
      if (v) v.playbackRate = 1
    }
  }, [code, apply])

  // Wait for everyone: this player tells the party when it's loading (the
  // party then holds everyone, for half a minute at most) and when it's
  // ready again. A blip under a second isn't worth stopping everyone for.
  useEffect(() => {
    if (!code || !video || opts.current.chatOnly) return
    let said = false
    let timer = 0
    const tell = (action: 'wait' | 'ready') => {
      said = action === 'wait'
      void api.partyState(code, { client, action, pos: opts.current.position() }).catch(() => undefined)
    }
    const loading = () => {
      window.clearTimeout(timer)
      timer = window.setTimeout(() => {
        if (!said && video.readyState < 3 && stateRef.current?.playing) tell('wait')
      }, 800)
    }
    const ready = () => {
      window.clearTimeout(timer)
      if (said) tell('ready')
    }
    // Just joined (or a new stream): loading until it can play.
    if (video.readyState < 3) tell('wait')
    video.addEventListener('waiting', loading)
    video.addEventListener('emptied', loading)
    video.addEventListener('playing', ready)
    video.addEventListener('canplay', ready)
    return () => {
      window.clearTimeout(timer)
      video.removeEventListener('waiting', loading)
      video.removeEventListener('emptied', loading)
      video.removeEventListener('playing', ready)
      video.removeEventListener('canplay', ready)
      if (said) tell('ready')
    }
  }, [code, video, client])

  // This player moved to another episode (Next episode): the party follows.
  const { kind, tmdbId, season, episode } = o.title
  useEffect(() => {
    const s = stateRef.current
    if (!code || !s || kind !== 'tv' || opts.current.chatOnly) return
    if ((s.title.season ?? 0) !== season || (s.title.episode ?? 0) !== episode) {
      // Remember the episode left, so a late word about it doesn't pull
      // this player back (see the state handler).
      if (s.title.tmdbId === tmdbId) left.current = { season: s.title.season ?? 0, episode: s.title.episode ?? 0, at: performance.now() }
      void api
        .partyState(code, { client, action: 'episode', pos: 0, title: opts.current.title })
        .then((ns) => {
          got.current = performance.now()
          stateRef.current = ns
          setState(ns)
        })
        .catch(() => undefined)
    }
  }, [code, client, kind, tmdbId, season, episode])

  const chat = useCallback((text: string) => (code ? api.partyChat(code, client, text) : Promise.resolve({ ok: false })), [code, client])
  const react = useCallback((emoji: string) => code && void api.partyReact(code, client, emoji).catch(() => undefined), [code, client])
  // tap starts the video from a tap, which a phone always allows.
  const tap = () => {
    setNeedsTap(false)
    const v = opts.current.video
    if (v) void v.play().catch(() => undefined)
  }
  // startPoint is where a converted video should start to meet the party:
  // where it is now (the party waits while it loads; counted as a jump, so
  // it isn't chased again at once). Undefined outside a party.
  const startPoint = (): number | undefined => {
    const s = stateRef.current
    if (!code || !s) return undefined
    const going = s.playing && !s.held
    const now = s.pos + (going ? (performance.now() - got.current) / 1000 : 0)
    if (going) jumped.current = { at: performance.now() }
    return now
  }
  // Who the party is waiting for, while it's held.
  const waitingFor = state?.held ? state.members.filter((m) => m.waiting).map((m) => m.name) : []
  return { client, state, msgs, floating, problem, chat, react, unread, seen: () => setUnread(0), needsTap, tap, startPoint, waitingFor }
}

// startParty makes a party for what's playing; answers its code.
export async function startParty(title: PartyTitle, pos: number, playing: boolean): Promise<string> {
  const s = await api.startParty({ ...title, pos, playing, client: clientId() })
  return s.code
}

// ---- The sidebar ----

export function PartyPanel({ party, code, onClose }: { party: ReturnType<typeof useParty>; code: string; onClose: () => void }) {
  const [text, setText] = useState('')
  const [copied, setCopied] = useState(false)
  const list = useRef<HTMLDivElement>(null)
  const { msgs, state, seen } = party
  useEffect(() => {
    seen()
    const el = list.current
    if (el) el.scrollTop = el.scrollHeight
  }, [msgs, seen])
  const send = (e: FormEvent) => {
    e.preventDefault()
    const t = text.trim()
    if (!t) return
    setText('')
    void party.chat(t).catch(() => setText(t))
  }
  const share = async () => {
    const link = partyLink(code)
    const nav = navigator as Navigator & { share?: (d: { title: string; url: string }) => Promise<void> }
    try {
      if (nav.share && matchMedia('(hover: none)').matches) await nav.share({ title: 'Watch with me on Cue', url: link })
      else {
        await navigator.clipboard.writeText(link)
        setCopied(true)
        window.setTimeout(() => setCopied(false), 2000)
      }
    } catch {
      /* cancelled */
    }
  }
  return (
    <aside className="wx-party" aria-label="Watch party" onPointerUp={(e) => e.stopPropagation()}>
      <header>
        <div>
          <strong>Watch party</strong>
          <small>
            {state ? `${state.members.length} watching` : 'Joining…'} · code {code}
          </small>
        </div>
        <button className="wx-party-x" onClick={onClose} aria-label="Hide the chat">
          <Icon name="x" size={18} />
        </button>
      </header>
      <div className="wx-party-faces">
        {state?.members.map((m) => (
          <span key={`${m.profile}-${m.name}`} title={m.host ? `${m.name} (started it)` : m.name}>
            <Avatar profile={m} size={24} />
          </span>
        ))}
        <button className="wx-party-invite" onClick={() => void share()}>
          <Icon name="plus" size={14} /> {copied ? 'Link copied' : 'Invite'}
        </button>
      </div>
      <p className="wx-party-tip">Everyone on this Cue sees the party on Home and can join with one press, even on a TV.</p>
      <div className="wx-party-msgs" ref={list} aria-live="polite">
        {party.problem && <p className="wx-party-event">{party.problem}</p>}
        {msgs.map((m) =>
          m.kind === 'event' ? (
            <p key={m.id} className="wx-party-event">
              {m.text}
            </p>
          ) : (
            <div key={m.id} className="wx-party-msg">
              <Avatar profile={{ name: m.name ?? '', avatar: m.avatar ?? '' }} size={22} />
              <div>
                <b>{m.name}</b>
                <p>{m.text}</p>
              </div>
            </div>
          ),
        )}
      </div>
      <div className="wx-party-reacts">
        {REACTIONS.map((r) => (
          <button key={r} onClick={() => party.react(r)} aria-label={`React ${r}`}>
            {r}
          </button>
        ))}
      </div>
      <form className="wx-party-say" onSubmit={send}>
        <input value={text} onChange={(e) => setText(e.target.value)} placeholder="Say something" maxLength={300} aria-label="Message" />
        <button aria-label="Send" disabled={!text.trim()}>
          <Icon name="send" size={18} />
        </button>
      </form>
    </aside>
  )
}

// Reactions, as in a Teams meeting: each pops up on the picture with who
// sent it, drifts up with a gentle sway, and fades away.
export function FloatingReactions({ list }: { list: Floating[] }) {
  return (
    <div className="wx-party-floats" aria-hidden="true">
      {list.map((f) => (
        <span key={f.id} className="wx-react" style={{ left: `${f.left}%`, ['--sway' as string]: f.sway }}>
          <span className="wx-react-sway">
            <b className="wx-react-emoji">{f.emoji}</b>
            <small>{f.name}</small>
          </span>
        </span>
      ))}
    </div>
  )
}

// ---- Joining from a link (/watch/party/{code}) ----

export function PartyJoin() {
  const { code = '' } = useParams()
  const navigate = useNavigate()
  const [problem, setProblem] = useState('')
  const chatOnly = new URLSearchParams(location.search).get('chat') === '1'
  // From the TV's Listen on phone: the phone plays the sound, picture hidden.
  const listen = new URLSearchParams(location.search).get('listen') === '1'
  useEffect(() => {
    if (chatOnly) return
    api
      .getParty(code)
      .then((s) => navigate(`${playHref(s.title.kind, s.title.tmdbId, s.title.season, s.title.episode)}?party=${s.code}${listen ? '&listen=1' : ''}`, { replace: true }))
      .catch((e) => setProblem(e instanceof Error ? e.message : String(e)))
  }, [code, navigate, chatOnly, listen])
  if (chatOnly) return <PartyChatOnly code={code.toUpperCase()} />
  return (
    <div className="wx-party-join">
      {problem ? (
        <>
          <p>{problem}</p>
          <button className="wx-btn" onClick={() => navigate('/watch', { replace: true })}>
            Go to Home
          </button>
        </>
      ) : (
        <p>Joining the watch party…</p>
      )}
    </div>
  )
}

// ---- Home: the parties going on now ----

export function PartiesNow() {
  const [list, setList] = useState<PartyNow[]>([])
  const navigate = useNavigate()
  useEffect(() => {
    let live = true
    const load = () =>
      api
        .partiesNow()
        .then((l) => live && setList(l))
        .catch(() => undefined)
    void load()
    const t = window.setInterval(load, 20_000)
    return () => {
      live = false
      window.clearInterval(t)
    }
  }, [])
  if (list.length === 0) return null
  return (
    <section className="wx-parties" aria-label="Watch parties" data-row="parties">
      {list.map((p) => {
        const what = p.title.kind === 'tv' && p.title.season ? `${p.title.title} · S${p.title.season}:E${p.title.episode}` : p.title.title
        return (
          <button
            key={p.code}
            className="wx-party-card"
            onClick={() => navigate(`${playHref(p.title.kind, p.title.tmdbId, p.title.season, p.title.episode)}?party=${p.code}`)}
          >
            <span className="wx-party-card-faces">
              {p.faces.map((m, i) => (
                <Avatar key={i} profile={m} size={34} />
              ))}
            </span>
            <span className="wx-party-card-words">
              <small>
                <i className="wx-party-live" /> Watch party · {p.members} watching
              </small>
              <strong>
                {p.host?.name ?? 'Someone'} is watching {what}
              </strong>
            </span>
            <span className="wx-btn small play">Join</span>
          </button>
        )
      })}
    </section>
  )
}

// The chat on its own, for a phone beside a TV (the TV's QR code opens it):
// type, react and see who did what, while the TV plays.
function PartyChatOnly({ code }: { code: string }) {
  const navigate = useNavigate()
  const party = useParty({
    code,
    video: null,
    title: { kind: 'movie', tmdbId: 0, title: '' },
    position: () => 0,
    seek: () => undefined,
    follow: () => undefined,
    startAt: () => undefined,
    chatOnly: true,
  })
  const t = party.state?.title
  return (
    <div className="wx-party-phone">
      {t && (
        <p className="wx-party-phone-what">
          Watching {t.title}
          {t.kind === 'tv' && t.season ? ` · S${t.season}:E${t.episode}` : ''}
        </p>
      )}
      <PartyPanel party={party} code={code} onClose={() => navigate('/watch')} />
    </div>
  )
}
