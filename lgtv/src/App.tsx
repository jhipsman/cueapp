import { useCallback, useEffect, useRef, useState } from 'react'
import { api, ApiError, saved, type Card } from './api'
import { focus, focusIn, KEY, move, onKey } from './focus'
import { HomeScreen } from './screens/Home'
import { Player } from './screens/Player'
import { SearchScreen } from './screens/Search'
import { ProfilesScreen, ServerScreen, SignInScreen } from './screens/Start'
import { TitleScreen, type PlayTarget } from './screens/TitlePage'
import { Avatar, Btn, Icon, Spinner } from './ui'

type Phase = 'loading' | 'server' | 'signin' | 'profiles' | 'main' | 'offline'
type Tab = 'search' | 'home' | 'movies' | 'shows'

const TABS: { id: Tab; label: string; icon: 'search' | 'home' | 'film' | 'tv' }[] = [
  { id: 'search', label: 'Search', icon: 'search' },
  { id: 'home', label: 'Home', icon: 'home' },
  { id: 'movies', label: 'Movies', icon: 'film' },
  { id: 'shows', label: 'Shows', icon: 'tv' },
]

export default function App() {
  const [phase, setPhase] = useState<Phase>('loading')
  const [problem, setProblem] = useState('')

  // Where to start: the server, signing in, a profile, or straight in.
  const start = useCallback(async () => {
    setPhase('loading')
    if (!saved.server) return setPhase('server')
    if (!saved.session) return setPhase('signin')
    try {
      await api.me()
      setPhase(saved.profile ? 'main' : 'profiles')
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        saved.session = ''
        return setPhase('signin')
      }
      setProblem(e instanceof Error ? e.message : String(e))
      setPhase('offline')
    }
  }, [])
  useEffect(() => void start(), [start])

  const signOut = useCallback(async () => {
    await api.logout()
    setPhase('signin')
  }, [])

  switch (phase) {
    case 'loading':
      return (
        <div className="start">
          <Spinner />
        </div>
      )
    case 'server':
      return <ServerScreen onDone={() => void start()} />
    case 'signin':
      return <SignInScreen onDone={() => setPhase('profiles')} onChangeServer={() => setPhase('server')} />
    case 'profiles':
      return <ProfilesScreen onDone={() => setPhase('main')} onSignOut={() => void signOut()} />
    case 'offline':
      return <Offline problem={problem} onRetry={() => void start()} onServer={() => setPhase('server')} />
    default:
      return <Main onProfiles={() => setPhase('profiles')} onSignedOut={() => setPhase('signin')} />
  }
}

function Offline({ problem, onRetry, onServer }: { problem: string; onRetry: () => void; onServer: () => void }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => void focusIn(ref.current), [])
  return (
    <div className="start" ref={ref}>
      <h1>Can’t reach Cue</h1>
      <p className="dim">{problem}</p>
      <div className="btns">
        <Btn primary first onClick={onRetry}>
          Try again
        </Btn>
        <Btn onClick={onServer}>Change server</Btn>
      </div>
    </div>
  )
}

function Main({ onProfiles, onSignedOut }: { onProfiles: () => void; onSignedOut: () => void }) {
  const [tab, setTab] = useState<Tab>('home')
  const [stack, setStack] = useState<{ card: Card; from: HTMLElement | null }[]>([])
  const [playing, setPlaying] = useState<PlayTarget | null>(null)
  const [railOpen, setRailOpen] = useState(false)
  const [toast, setToast] = useState('')
  const rail = useRef<HTMLElement>(null)
  const content = useRef<HTMLDivElement>(null)
  const lastIn = useRef(new Map<Tab, HTMLElement>())
  const leaving = useRef(0)
  const tabTimer = useRef(0)
  const profileName = useRef('')
  const [me, setMe] = useState<{ name: string; avatar: string } | null>(null)

  useEffect(() => {
    api
      .profiles()
      .then((a) => {
        const p = a.profiles.find((x) => x.id === saved.profileId)
        if (p) {
          profileName.current = p.name
          setMe({ name: p.name, avatar: p.avatar })
        }
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 401) {
          saved.session = ''
          onSignedOut()
        }
      })
  }, [onSignedOut])

  // Start on Home's first row.
  useEffect(() => {
    const t = window.setTimeout(() => focusIn(content.current?.querySelector('[data-tab="home"]') ?? null), 400)
    return () => window.clearTimeout(t)
  }, [])

  // Where the highlight is in each tab, to go back to it from the menu.
  useEffect(() => {
    const on = (e: FocusEvent) => {
      const el = e.target as HTMLElement
      const inRail = !!rail.current?.contains(el)
      setRailOpen(inRail)
      const box = el.closest<HTMLElement>('[data-tab]')
      if (box) lastIn.current.set(box.getAttribute('data-tab') as Tab, el)
    }
    document.addEventListener('focusin', on)
    return () => document.removeEventListener('focusin', on)
  }, [])

  const railItem = (t: Tab) => rail.current?.querySelector<HTMLElement>(`[data-k="${t}"]`) ?? null
  // Into a tab's page: where the highlight was there, else its first thing.
  const intoContent = useCallback((t: Tab) => {
    const box = content.current?.querySelector<HTMLElement>(`[data-tab="${t}"]`) ?? null
    const last = lastIn.current.get(t)
    if (last && document.contains(last) && box?.contains(last)) focus(last)
    else focusIn(box)
  }, [])

  const openTitle = useCallback((card: Card) => {
    setStack((s) => [...s.slice(-7), { card, from: document.activeElement as HTMLElement | null }])
  }, [])
  const closeTitle = useCallback(() => {
    setStack((s) => {
      const top = s[s.length - 1]
      window.setTimeout(() => {
        if (top?.from && document.contains(top.from)) focus(top.from)
        else if (s.length <= 1) intoContent(tab)
      }, 30)
      return s.slice(0, -1)
    })
  }, [intoContent, tab])

  // The remote, for the menu and Back (screens take their keys first).
  useEffect(() =>
    onKey((code) => {
      if (playing) return false
      const active = document.activeElement as HTMLElement | null
      const inRail = !!(active && rail.current?.contains(active))
      if (code === KEY.back) {
        if (stack.length) {
          closeTitle()
        } else if (!inRail) {
          focus(railItem(tab))
        } else if (tab !== 'home') {
          setTab('home')
          focus(railItem('home'))
        } else if (Date.now() - leaving.current < 2500) {
          window.close()
        } else {
          leaving.current = Date.now()
          setToast('Press Back again to leave Cue')
          window.setTimeout(() => setToast(''), 2500)
        }
        return true
      }
      if (stack.length) return false
      if (code === KEY.right && inRail) {
        // Into the page of the menu item the highlight is on (now, not
        // when the menu would have switched to it).
        const t = (active?.getAttribute('data-k') ?? tab) as Tab
        if (!TABS.some((x) => x.id === t)) return true
        window.clearTimeout(tabTimer.current)
        setTab(t)
        window.setTimeout(() => intoContent(t), 40)
        return true
      }
      if (code === KEY.left && !inRail && active?.closest('[data-tab]')) {
        // Left from the first thing in a row: the menu, on this page's item.
        if (!move('left', (el) => !rail.current?.contains(el))) focus(railItem(tab))
        return true
      }
      return false
    }),
  )

  const pickTab = (t: Tab) => {
    window.clearTimeout(tabTimer.current)
    tabTimer.current = window.setTimeout(() => setTab(t), 180)
  }
  const top = stack[stack.length - 1]

  return (
    <div className={`main${top ? ' covered' : ''}`}>
      <nav className={`rail${railOpen ? ' open' : ''}`} ref={rail} data-g="rail" data-hidden={top ? '' : undefined}>
        <img className="rail-logo" src="./icon.png" alt="Cue" />
        <div className="rail-items">
          {TABS.map((t) => (
            <button
              key={t.id}
              className={`rail-item${tab === t.id ? ' on' : ''}`}
              data-f=""
              data-k={t.id}
              onFocus={() => pickTab(t.id)}
              onClick={() => {
                window.clearTimeout(tabTimer.current)
                setTab(t.id)
                window.setTimeout(() => intoContent(t.id), 40)
              }}
            >
              <Icon name={t.icon} />
              <span className="rail-label">{t.label}</span>
            </button>
          ))}
        </div>
        <button className="rail-item rail-me" data-f="" data-k="me" onClick={onProfiles}>
          {me ? <Avatar avatar={me.avatar} name={me.name} size={2.6} /> : <Avatar avatar="teal" name="?" size={2.6} />}
          <span className="rail-label">{me ? `${me.name} · Switch` : 'Profiles'}</span>
        </button>
      </nav>
      <div className="content" ref={content}>
        {TABS.map((t) => (
          <div key={t.id} className="tab" data-tab={t.id} data-hidden={tab !== t.id || top ? '' : undefined}>
            {t.id === 'search' ? <SearchScreen onPick={openTitle} /> : <HomeScreen kind={t.id === 'movies' ? 'movie' : t.id === 'shows' ? 'tv' : undefined} onPick={openTitle} />}
          </div>
        ))}
      </div>
      {stack.map((s, i) => (
        <div key={`${s.card.kind}-${s.card.tmdbId}-${i}`} className="page" data-hidden={i !== stack.length - 1 ? '' : undefined}>
          <TitleScreen card={s.card} onPick={openTitle} onPlay={setPlaying} />
        </div>
      ))}
      {playing && (
        <Player
          key={`${playing.kind}-${playing.tmdbId}-${playing.season}-${playing.episode}`}
          target={playing}
          onPlay={setPlaying}
          onClose={() => {
            const from = document.querySelector<HTMLElement>('.page:not([data-hidden]) [data-k="play"]')
            setPlaying(null)
            window.setTimeout(() => focus(from), 30)
          }}
        />
      )}
      {toast && <div className="toast">{toast}</div>}
    </div>
  )
}
