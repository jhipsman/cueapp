import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type LiveProgramme, type LiveReminder } from '../api'
import Icon from '../components/Icon'
import ChannelLogo from './ChannelLogo'
import { useProfiles } from './profiles'
import { onTV } from './tv'

// Live TV reminders: shows this profile asked to be told about. They're kept
// on the server (so a reminder set on the phone shows on the TV), and when
// one starts a card pops up wherever Watch is open, with Watch.

interface RemindersState {
  list: LiveReminder[]
  has: (channelId: string, start: string) => boolean
  toggle: (channelId: string, show: LiveProgramme) => Promise<void>
}

const Ctx = createContext<RemindersState | null>(null)
const key = (channelId: string, start: string) => `${channelId}|${Date.parse(start)}`
const DISMISSED = 'cue-reminders-shown'

function shown(): string[] {
  try {
    return JSON.parse(localStorage.getItem(DISMISSED) ?? '[]')
  } catch {
    return []
  }
}

export function RemindersProvider({ children }: { children: ReactNode }) {
  const { data, active } = useProfiles()
  const enabled = !!data?.liveTV && !!active
  const [list, setList] = useState<LiveReminder[]>([])
  const navigate = useNavigate()
  const [now, setNow] = useState(() => Date.now())
  const [gone, setGone] = useState<string[]>(shown)

  const load = useCallback(() => {
    if (!enabled) return
    api
      .liveReminders()
      .then(setList)
      .catch(() => undefined)
  }, [enabled])
  // Every minute: reminders set on another device show here too.
  useEffect(() => {
    load()
    const t = setInterval(() => {
      setNow(Date.now())
      load()
    }, 60_000)
    return () => clearInterval(t)
  }, [load])

  const keys = useMemo(() => new Set(list.map((r) => key(r.channelId, r.start))), [list])
  const has = useCallback((channelId: string, start: string) => keys.has(key(channelId, start)), [keys])
  const toggle = useCallback(
    async (channelId: string, show: LiveProgramme) => {
      if (keys.has(key(channelId, show.start))) {
        await api.removeLiveReminder(channelId, show.start)
      } else {
        await api.addLiveReminder({ channelId, start: show.start, stop: show.stop, title: show.title })
      }
      load()
    },
    [keys, load],
  )

  // The one starting now (or within a minute), not yet dismissed here.
  const due = list.find((r) => {
    const s = Date.parse(r.start)
    return s - 60_000 <= now && now < Date.parse(r.stop) && now - s < 30 * 60_000 && !gone.includes(key(r.channelId, r.start))
  })
  const dismiss = (r: LiveReminder) => {
    const next = [...gone, key(r.channelId, r.start)].slice(-50)
    setGone(next)
    try {
      localStorage.setItem(DISMISSED, JSON.stringify(next))
    } catch {
      // Private window: it may pop up again after a reload.
    }
  }

  const value = useMemo(() => ({ list, has, toggle }), [list, has, toggle])
  return (
    <Ctx.Provider value={value}>
      {children}
      {due && (
        <div className="wx-reminder" role="alert">
          <span className="wx-reminder-logo">
            <ChannelLogo name={due.channel ?? ''} src={due.logo} />
          </span>
          <div className="wx-reminder-body">
            <small>
              <Icon name="clock" size={13} /> {Date.parse(due.start) > now ? 'Starting in a minute' : 'Starting now'}
              {due.channel ? ` on ${due.channel}` : ''}
            </small>
            <b>{due.title}</b>
          </div>
          <div className="wx-reminder-actions">
            <button
              className="wx-btn play small"
              autoFocus={onTV()}
              onClick={() => {
                dismiss(due)
                navigate(`/watch/live?cat=all&ch=${encodeURIComponent(due.channelId)}${onTV() ? '' : '&full=1'}`)
              }}
            >
              <Icon name="play" size={14} /> Watch
            </button>
            <button className="wx-btn small" onClick={() => dismiss(due)}>
              Dismiss
            </button>
          </div>
        </div>
      )}
    </Ctx.Provider>
  )
}

export function useReminders(): RemindersState {
  const v = useContext(Ctx)
  return v ?? { list: [], has: () => false, toggle: async () => undefined }
}
