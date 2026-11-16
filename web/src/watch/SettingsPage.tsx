import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, isAdmin } from '../api'
import { useAuth } from '../AuthContext'
import Icon, { type IconName } from '../components/Icon'
import { useModules } from '../ModulesContext'
import { Avatar, useProfiles } from './profiles'
import { usePrefs, type PrefKey } from './prefs'

// Settings (/watch/settings/{section}): everything an account sets for
// itself, in categories, as streaming apps lay it out. The server's own
// settings stay in the manager (Server settings, the owner's only).

interface Section {
  id: string
  title: string
  icon: IconName
  owner?: boolean // the account's main profile, administrators only
  body: () => ReactNode
}

function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export default function SettingsPage() {
  const { section } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const { active } = useProfiles()
  const owner = isAdmin(user) && !!active?.main
  const sections: Section[] = [
    { id: 'account', title: 'Account', icon: 'user', body: () => <AccountSection /> },
    { id: 'security', title: 'Sign-in & security', icon: 'shield', body: () => <SecuritySection /> },
    { id: 'profiles', title: 'Profiles & look', icon: 'grid', body: () => <ProfilesSection /> },
    { id: 'playback', title: 'Playback', icon: 'play', body: () => <PlaybackSection /> },
    { id: 'privacy', title: 'Privacy', icon: 'eye-off', body: () => <PrivacySection /> },
    { id: 'devices', title: 'TVs & devices', icon: 'tv', body: () => <DevicesSection /> },
    { id: 'family', title: 'Family', icon: 'heart', owner: true, body: () => <FamilySection /> },
    { id: 'help', title: 'Help & about', icon: 'info', body: () => <HelpSection owner={owner} /> },
  ].filter((s) => !s.owner || owner)
  const current = sections.find((s) => s.id === section)
  useEffect(() => {
    document.title = current ? `${current.title} · Settings · Cue` : 'Settings · Cue'
  }, [current])

  return (
    <div className={`wx-settings${current ? ' open' : ''}`}>
      <nav className="wx-set-nav" aria-label="Settings">
        <h1>Settings</h1>
        {sections.map((s) => (
          <Link key={s.id} to={`/watch/settings/${s.id}`} className={s.id === current?.id ? 'on' : ''} replace={!!current}>
            <Icon name={s.icon} size={18} />
            <span>{s.title}</span>
            <Icon name="chevron-right" size={16} className="wx-set-go" />
          </Link>
        ))}
      </nav>
      <section className="wx-set-body" key={current?.id ?? 'none'}>
        {current ? (
          <>
            <button className="wx-set-back" onClick={() => navigate('/watch/settings')}>
              <Icon name="chevron-left" size={18} /> Settings
            </button>
            <h2>{current.title}</h2>
            {current.body()}
          </>
        ) : (
          <p className="wx-set-hint">Pick a category.</p>
        )}
      </section>
    </div>
  )
}

// ---- Pieces ----

function Row({ title, text, children }: { title: string; text?: ReactNode; children?: ReactNode }) {
  return (
    <div className="wx-set-row">
      <div className="wx-set-words">
        <strong>{title}</strong>
        {text && <small>{text}</small>}
      </div>
      {children && <div className="wx-set-control">{children}</div>}
    </div>
  )
}

function Toggle({ k, title, text }: { k: PrefKey; title: string; text: string }) {
  const prefs = usePrefs()
  const on = prefs.on(k)
  const [problem, setProblem] = useState('')
  return (
    <Row title={title} text={problem || text}>
      <button
        role="switch"
        aria-checked={on}
        aria-label={title}
        className={`wx-switch${on ? ' on' : ''}`}
        onClick={() => {
          setProblem('')
          prefs.set(k, on ? '0' : '1').catch((e) => setProblem(errorText(e)))
        }}
      >
        <i />
      </button>
    </Row>
  )
}

function Go({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Link className="wx-btn small" to={to}>
      {children}
    </Link>
  )
}

function Note({ ok, children }: { ok?: boolean; children: ReactNode }) {
  return <p className={`wx-set-note${ok ? ' ok' : ''}`}>{children}</p>
}

// ---- Sections ----

function AccountSection() {
  const { user, setUser } = useAuth()
  const navigate = useNavigate()
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [again, setAgain] = useState('')
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [busy, setBusy] = useState(false)
  async function change(e: FormEvent) {
    e.preventDefault()
    if (next !== again) return setMsg({ ok: false, text: "The new passwords don't match." })
    setBusy(true)
    setMsg(null)
    try {
      await api.changePassword(cur, next)
      setCur('')
      setNext('')
      setAgain('')
      setMsg({ ok: true, text: 'Password changed.' })
    } catch (err) {
      setMsg({ ok: false, text: errorText(err) })
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      <Row title={user?.name || user?.username || ''} text={`Signed in as ${user?.username}`}>
        <Go to="/settings/profile">Edit name and email</Go>
      </Row>
      <form className="wx-set-form" onSubmit={change}>
        <strong>Change password</strong>
        <input type="password" placeholder="Current password" autoComplete="current-password" value={cur} onChange={(e) => setCur(e.target.value)} />
        <input type="password" placeholder="New password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} />
        <input type="password" placeholder="New password again" autoComplete="new-password" value={again} onChange={(e) => setAgain(e.target.value)} />
        {msg && <Note ok={msg.ok}>{msg.text}</Note>}
        <button className="wx-btn small play" disabled={busy || !cur || next.length < 8}>
          {busy ? 'Changing…' : 'Change password'}
        </button>
      </form>
      <Row title="Sign out" text="Signs this browser out. Your profiles and settings stay.">
        <button
          className="wx-btn small"
          onClick={() =>
            void api.logout().then(() => {
              setUser(null)
              navigate('/', { replace: true })
            })
          }
        >
          Sign out
        </button>
      </Row>
    </>
  )
}

function SecuritySection() {
  const [state, setState] = useState<{ on: boolean; backupLeft: number } | null>(null)
  const [setup, setSetup] = useState<{ secret: string; uri: string } | null>(null)
  const [code, setCode] = useState('')
  const [codes, setCodes] = useState<string[] | null>(null)
  const [password, setPassword] = useState('')
  const [asking, setAsking] = useState<'' | 'off' | 'backup'>('')
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)
  const load = () =>
    api
      .twoStep()
      .then(setState)
      .catch((e) => setMsg(errorText(e)))
  useEffect(() => {
    void load()
  }, [])
  async function run(work: () => Promise<void>) {
    setBusy(true)
    setMsg('')
    try {
      await work()
    } catch (e) {
      setMsg(errorText(e))
    } finally {
      setBusy(false)
    }
  }
  if (!state) return <div className="skeleton" style={{ height: 80, borderRadius: 12 }} />
  return (
    <>
      <Row
        title="Two-step sign-in"
        text={
          state.on
            ? `On. Signing in asks for a code from your authenticator app. ${state.backupLeft} backup ${state.backupLeft === 1 ? 'code' : 'codes'} left.`
            : 'After your password, Cue also asks for a code from an authenticator app on your phone (such as Google Authenticator, Microsoft Authenticator or your iPhone’s Passwords). A stolen password alone then can’t get in.'
        }
      >
        {!state.on && !setup && (
          <button className="wx-btn small play" disabled={busy} onClick={() => void run(async () => setSetup(await api.twoStepStart()))}>
            Turn on
          </button>
        )}
      </Row>
      {setup && !codes && (
        <div className="wx-set-panel">
          <ol>
            <li>Open your authenticator app and add an account.</li>
            <li>
              Scan this code, or on this phone <a href={setup.uri}>open it in the app</a>. Or type the key: <code>{setup.secret.match(/.{1,4}/g)?.join(' ')}</code>
            </li>
            <li>Type the six-digit code the app shows for Cue.</li>
          </ol>
          <img className="wx-set-qr" src="/api/auth/two-step/qr" alt="QR code to add Cue to your authenticator app" />
          <form
            className="wx-set-inline"
            onSubmit={(e) => {
              e.preventDefault()
              void run(async () => {
                const r = await api.twoStepConfirm(code)
                setCodes(r.backupCodes)
                setSetup(null)
                setCode('')
                await load()
              })
            }}
          >
            <input inputMode="numeric" autoComplete="one-time-code" placeholder="123456" value={code} onChange={(e) => setCode(e.target.value)} maxLength={6} aria-label="Code from the app" />
            <button className="wx-btn small play" disabled={busy || code.trim().length !== 6}>
              Turn on
            </button>
            <button type="button" className="wx-btn small" onClick={() => setSetup(null)}>
              Cancel
            </button>
          </form>
        </div>
      )}
      {codes && (
        <div className="wx-set-panel">
          <strong>Your backup codes</strong>
          <p>If you lose your phone, each of these signs you in once instead of a code. Keep them somewhere safe; they won't be shown again.</p>
          <div className="wx-set-codes">
            {codes.map((c) => (
              <code key={c}>{c}</code>
            ))}
          </div>
          <div className="wx-set-inline">
            <button className="wx-btn small" onClick={() => void navigator.clipboard?.writeText(codes.join('\n'))}>
              Copy
            </button>
            <button className="wx-btn small play" onClick={() => setCodes(null)}>
              I've saved them
            </button>
          </div>
        </div>
      )}
      {state.on && !codes && (
        <>
          {asking ? (
            <form
              className="wx-set-inline"
              onSubmit={(e) => {
                e.preventDefault()
                void run(async () => {
                  if (asking === 'off') await api.twoStepOff(password)
                  else setCodes((await api.twoStepBackup(password)).backupCodes)
                  setPassword('')
                  setAsking('')
                  await load()
                })
              }}
            >
              <input type="password" placeholder="Your password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} autoFocus />
              <button className="wx-btn small play" disabled={busy || !password}>
                {asking === 'off' ? 'Turn off' : 'Make new codes'}
              </button>
              <button type="button" className="wx-btn small" onClick={() => setAsking('')}>
                Cancel
              </button>
            </form>
          ) : (
            <div className="wx-set-inline">
              <button className="wx-btn small" onClick={() => setAsking('backup')}>
                New backup codes
              </button>
              <button className="wx-btn small" onClick={() => setAsking('off')}>
                Turn off
              </button>
            </div>
          )}
        </>
      )}
      {msg && <Note>{msg}</Note>}
      <Row title="Signing a TV in" text="TVs sign in by scanning a code with your phone, which is already signed in, so they don't need a code of their own." />
    </>
  )
}

function ProfilesSection() {
  const { active, data } = useProfiles()
  return (
    <>
      {active && (
        <Row title={active.name} text="The profile you're watching as.">
          <Avatar profile={active} size={40} />
        </Row>
      )}
      {(data?.profiles.length ?? 0) > 1 && (
        <Row title="Switch profile">
          <Go to="/watch/who">Switch</Go>
        </Row>
      )}
      {active?.main && (
        <Row title="Manage profiles" text="Add, rename or remove profiles, and change their pictures.">
          <Go to="/watch/profiles">Manage</Go>
        </Row>
      )}
      <Row title="Colors" text="This profile's colors, on the web and the TV apps.">
        <Go to="/watch/theme">Change</Go>
      </Row>
    </>
  )
}

function PlaybackSection() {
  return (
    <>
      <Toggle k="autoplayNext" title="Play the next episode automatically" text="When an episode ends, the next one starts after a short countdown." />
      <Toggle k="previews" title="Previews" text="Short clips play on Home's banner and on a title's page after a moment." />
      <Toggle k="previewSound" title="Preview sound on a title's page" text="A title's preview plays with its sound, as on the streaming apps." />
      <Row title="Streaming quality" text="The picture to aim for, and how big a file may be.">
        <Go to="/watch/quality">Change</Go>
      </Row>
      <Row title="Subtitles" text="Turn them on in the player with CC; their size and position are kept for next time." />
    </>
  )
}

function PrivacySection() {
  const { active } = useProfiles()
  const [sure, setSure] = useState(false)
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)
  return (
    <>
      <Row
        title="What Cue keeps"
        text="What each profile has watched and how far, for Continue Watching and Your year. It's kept on your own server, and only your household's accounts can see it."
      />
      <Row title={`Clear ${active?.name ?? 'this profile'}'s watch history`} text="Empties Continue Watching and forgets what this profile has watched. Other profiles keep theirs.">
        {sure ? (
          <span className="wx-set-inline">
            <button
              className="wx-btn small play"
              onClick={() =>
                void api
                  .clearWatchHistory()
                  .then(() => setMsg({ ok: true, text: 'Watch history cleared.' }))
                  .catch((e) => setMsg({ ok: false, text: errorText(e) }))
                  .finally(() => setSure(false))
              }
            >
              Clear it
            </button>
            <button className="wx-btn small" onClick={() => setSure(false)}>
              Cancel
            </button>
          </span>
        ) : (
          <button className="wx-btn small" onClick={() => setSure(true)}>
            Clear
          </button>
        )}
      </Row>
      {msg && <Note ok={msg.ok}>{msg.text}</Note>}
    </>
  )
}

function DevicesSection() {
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)
  return (
    <>
      <Row title="Sign in a TV" text="Open Cue on the TV and scan its code with this phone, or type the code here.">
        <Go to="/watch/link">Sign in a TV</Go>
      </Row>
      <Row title="Get Cue on a TV" text="Step by step for a Fire TV or Google TV, with pictures.">
        <a className="wx-btn small" href="/setup-tv">
          Show me
        </a>
      </Row>
      <Row title="Sign out everywhere else" text="Signs your account out of every other browser, phone and TV. Use it after signing in on someone else's device.">
        <button
          className="wx-btn small"
          onClick={() =>
            void api
              .signOutOthers()
              .then((r) => setMsg({ ok: true, text: r.signedOut ? `Signed out of ${r.signedOut} other ${r.signedOut === 1 ? 'place' : 'places'}.` : 'You weren’t signed in anywhere else.' }))
              .catch((e) => setMsg({ ok: false, text: errorText(e) }))
          }
        >
          Sign out others
        </button>
      </Row>
      {msg && <Note ok={msg.ok}>{msg.text}</Note>}
    </>
  )
}

function FamilySection() {
  return (
    <>
      <Row title="Family code and invites" text="The code your family types to make their own accounts, and invite links.">
        <Go to="/settings/profile">Open</Go>
      </Row>
      <Row title="Accounts" text="Everyone's accounts: reset a password, change what one may do, or remove one.">
        <Go to="/settings/profile">Open</Go>
      </Row>
    </>
  )
}

function HelpSection({ owner }: { owner: boolean }) {
  const { streamingOnly } = useModules()
  const [version, setVersion] = useState('')
  useEffect(() => {
    api
      .version()
      .then((v) => setVersion(v.version))
      .catch(() => undefined)
  }, [])
  return (
    <>
      <Row title="How Cue works" text="The short tour from when you first signed in.">
        <Go to="/watch?tour=1">Show the tour</Go>
      </Row>
      <Row title="Your year in Cue" text="What you've watched this year.">
        <Go to="/watch/year">Open</Go>
      </Row>
      {owner && (
        <Row title="Server settings" text="Streaming sources, torrent sites, keys and the server itself.">
          <Go to={streamingOnly ? '/settings/streaming' : '/'}>Open</Go>
        </Row>
      )}
      {version && <Row title="Version" text={version} />}
    </>
  )
}
