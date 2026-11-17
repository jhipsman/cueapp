import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type RemoteAction, type Screen } from '../api'
import Icon from '../components/Icon'
import { epCode } from './parts'

// Send to TV: the household's TVs with Cue open, and a remote for each:
// what it's playing, play and pause, back 10 s, forward 30 s, a time bar to
// jump with, and stop. A title's page sends it there (Play on TV).

function clock(sec: number): string {
  const s = Math.max(0, Math.floor(sec))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = String(s % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

// useScreens is the TVs that are on, asked again every couple of seconds.
export function useScreens(every = 2000): Screen[] | null {
  const [list, setList] = useState<Screen[] | null>(null)
  useEffect(() => {
    let live = true
    const load = () =>
      api
        .screens()
        .then((l) => live && setList(l))
        .catch(() => live && setList([]))
    void load()
    const t = window.setInterval(load, every)
    return () => {
      live = false
      window.clearInterval(t)
    }
  }, [every])
  return list
}

export default function TVRemote() {
  const { id } = useParams()
  const navigate = useNavigate()
  const screens = useScreens()
  const [tick, setTick] = useState(() => Date.now())
  const [error, setError] = useState('')
  useEffect(() => {
    const t = window.setInterval(() => setTick(Date.now()), 500)
    return () => window.clearInterval(t)
  }, [])
  if (!screens) return <div className="wx-page" />
  const sc = screens.find((s) => s.id === id) ?? (screens.length === 1 ? screens[0] : undefined)

  if (!sc) {
    return (
      <div className="wx-page wx-remote">
        <h1>TV remote</h1>
        {screens.length === 0 ? (
          <p className="wx-dim">No TV has Cue open right now. Open Cue on the TV and it shows up here.</p>
        ) : (
          <div className="wx-remote-list">
            {screens.map((s) => (
              <button key={s.id} className="wx-btn" onClick={() => navigate(`/watch/tv-remote/${s.id}`)}>
                <Icon name="tv" size={20} /> {s.name}
                {s.now.title ? <span className="wx-dim"> · {s.now.title}</span> : null}
              </button>
            ))}
          </div>
        )}
      </div>
    )
  }

  const now = sc.now
  const pos = (now.pos ?? 0) + (now.playing && now.at ? (tick - now.at) / 1000 : 0)
  const len = now.duration ?? 0
  const press = (action: RemoteAction, at = 0) => {
    setError('')
    api.screenControl(sc.id, action, at).catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }

  return (
    <div className="wx-page wx-remote">
      <p className="wx-dim">
        <Icon name="tv" size={16} /> {sc.name}
        {sc.profile ? ` · ${sc.profile}` : ''}
      </p>
      {now.title ? (
        <>
          <h1>{now.title}</h1>
          {now.kind === 'tv' && now.episode ? <p className="wx-dim">{epCode(now.season, now.episode)}</p> : null}
          <div className="wx-remote-bar">
            <span>{clock(pos)}</span>
            <input
              type="range"
              min={0}
              max={Math.max(1, Math.floor(len))}
              value={Math.min(Math.floor(pos), Math.floor(len))}
              onChange={(e) => press('seek', Number(e.target.value))}
              aria-label="Jump to"
            />
            <span>{clock(len)}</span>
          </div>
          <div className="wx-remote-keys">
            <button className="wx-btn wx-round" onClick={() => press('back')} aria-label="Back 10 seconds">
              −10
            </button>
            <button className="wx-btn play wx-remote-main" onClick={() => press(now.playing ? 'pause' : 'play')} aria-label={now.playing ? 'Pause' : 'Play'}>
              <Icon name={now.playing ? 'pause' : 'play'} size={30} />
            </button>
            <button className="wx-btn wx-round" onClick={() => press('forward')} aria-label="Forward 30 seconds">
              +30
            </button>
          </div>
          <button className="wx-btn" onClick={() => press('stop')}>
            <Icon name="stop" size={18} /> Stop
          </button>
        </>
      ) : (
        <p className="wx-dim">Nothing's playing on {sc.name}. Pick something and press Play on TV.</p>
      )}
      {error && <p className="wx-error">{error}</p>}
      {screens.length > 1 && (
        <button className="wx-btn" style={{ marginTop: 24 }} onClick={() => navigate('/watch/tv-remote')}>
          Other TVs
        </button>
      )}
    </div>
  )
}
