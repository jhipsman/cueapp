import { useEffect, useRef, useState } from 'react'
import { api, ApiError, saved, tidyServer, type Profile } from '../api'
import { focus, KEY, onKey } from '../focus'
import { Avatar, Btn, Spinner, useFocusOnShow } from '../ui'

// The first screens: where your Cue is, signing in, and who's watching.

function Logo() {
  return (
    <div className="start-logo">
      <img src="./largeIcon.png" alt="" />
      <b>cue</b>
    </div>
  )
}

// A text box the remote reaches; OK (or Enter on the TV's keyboard) submits.
function Field(p: { label: string; value: string; onChange: (v: string) => void; onEnter: () => void; type?: string; first?: boolean; placeholder?: string }) {
  return (
    <label className="field">
      <span>{p.label}</span>
      <input
        data-f=""
        data-first={p.first ? '' : undefined}
        type={p.type ?? 'text'}
        value={p.value}
        placeholder={p.placeholder}
        autoComplete="off"
        spellCheck={false}
        onChange={(e) => p.onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.keyCode === KEY.ok) {
            e.preventDefault()
            p.onEnter()
          }
        }}
      />
    </label>
  )
}

export function ServerScreen({ onDone }: { onDone: () => void }) {
  const [value, setValue] = useState(saved.server)
  const [problem, setProblem] = useState('')
  const [busy, setBusy] = useState(false)
  const ref = useFocusOnShow()
  async function go() {
    const s = tidyServer(value)
    if (!/^https?:\/\/[^\s/]+/i.test(s)) {
      setProblem("That doesn't look like a web address.")
      return
    }
    setBusy(true)
    setProblem('')
    try {
      await api.reachable(s)
      if (s !== saved.server) {
        saved.session = ''
        saved.profile = ''
      }
      saved.server = s
      onDone()
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="start" ref={ref}>
      <Logo />
      <h1>Where's your Cue?</h1>
      <p className="dim">Its web address, the one you open in a browser, like http://192.168.1.20:8080 or https://cue.example.com.</p>
      <Field label="Address" value={value} onChange={setValue} onEnter={() => void go()} first placeholder="http://" />
      <p className="problem">{problem}</p>
      <Btn primary onClick={() => void go()} disabled={busy}>
        {busy ? 'Connecting…' : 'Continue'}
      </Btn>
    </div>
  )
}

export function SignInScreen({ onDone, onChangeServer }: { onDone: () => void; onChangeServer: () => void }) {
  const [link, setLink] = useState<{ code: string; token: string } | null>(null)
  const [mode, setMode] = useState<'phone' | 'password' | 'code'>('phone')
  const [user, setUser] = useState('')
  const [pass, setPass] = useState('')
  const [code, setCode] = useState('')
  const [ticket, setTicket] = useState('')
  const [problem, setProblem] = useState('')
  const ref = useFocusOnShow(mode)
  const host = saved.server.replace(/^https?:\/\//, '')

  // A code for signing in from a phone, renewed when it runs out; asked
  // about every two seconds.
  useEffect(() => {
    if (mode !== 'phone') return
    let live = true
    let timer = 0
    const start = async () => {
      try {
        const l = await api.tvLinkStart()
        if (!live) return
        setLink(l)
        const poll = async () => {
          if (!live) return
          try {
            const a = await api.tvLinkPoll(l.token)
            if (a.ok) return onDone()
          } catch (e) {
            if (e instanceof ApiError && e.status === 410) return void start()
          }
          timer = window.setTimeout(poll, 2000)
        }
        timer = window.setTimeout(poll, 2000)
      } catch (e) {
        if (live) setProblem(e instanceof Error ? e.message : String(e))
      }
    }
    void start()
    return () => {
      live = false
      window.clearTimeout(timer)
    }
  }, [mode, onDone])

  async function signIn() {
    setProblem('')
    try {
      const a = await api.login(user.trim(), pass)
      if (a?.twoStep && a.ticket) {
        setTicket(a.ticket)
        setMode('code')
        return
      }
      onDone()
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e))
    }
  }
  async function sendCode() {
    setProblem('')
    try {
      await api.loginCode(ticket, code.trim())
      onDone()
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div className="start" ref={ref}>
      <Logo />
      {mode === 'phone' && (
        <>
          <h1>Sign in</h1>
          <div className="link">
            {link ? <img className="link-qr" src={api.tvLinkQR(link.code)} alt="" /> : <Spinner />}
            <div>
              <p>Scan the code with your phone, or open</p>
              <p className="link-url">{host}/link</p>
              <p>and enter</p>
              <p className="link-code">{link?.code ?? '······'}</p>
            </div>
          </div>
          <p className="problem">{problem}</p>
          <div className="btns">
            <Btn first onClick={() => setMode('password')}>
              Use a password instead
            </Btn>
            <Btn onClick={onChangeServer}>Change server</Btn>
          </div>
        </>
      )}
      {mode === 'password' && (
        <>
          <h1>Sign in to {host}</h1>
          <Field label="Username" value={user} onChange={setUser} onEnter={() => focus(document.querySelector<HTMLElement>('input[type=password]'))} first />
          <Field label="Password" type="password" value={pass} onChange={setPass} onEnter={() => void signIn()} />
          <p className="problem">{problem}</p>
          <div className="btns">
            <Btn primary onClick={() => void signIn()}>
              Sign in
            </Btn>
            <Btn onClick={() => setMode('phone')}>Use my phone</Btn>
          </div>
        </>
      )}
      {mode === 'code' && (
        <>
          <h1>Two-step sign-in</h1>
          <p className="dim">The code from your authenticator app (or a backup code).</p>
          <Field label="Code" value={code} onChange={setCode} onEnter={() => void sendCode()} first />
          <p className="problem">{problem}</p>
          <Btn primary onClick={() => void sendCode()}>
            Sign in
          </Btn>
        </>
      )}
    </div>
  )
}

export function ProfilesScreen({ onDone, onSignOut }: { onDone: () => void; onSignOut: () => void }) {
  const [list, setList] = useState<Profile[] | null>(null)
  const [problem, setProblem] = useState('')
  const [locked, setLocked] = useState<Profile | null>(null)
  const [pin, setPin] = useState('')
  const ref = useFocusOnShow(list ? (locked ? 'pin' : 'list') : 0)
  const pinRef = useRef(pin)
  pinRef.current = pin

  useEffect(() => {
    api
      .profiles()
      .then((a) => setList(a.profiles))
      .catch((e) => setProblem(e instanceof Error ? e.message : String(e)))
  }, [])

  async function pick(p: Profile, withPin = '') {
    if (p.hasPin && !withPin) {
      setLocked(p)
      setPin('')
      return
    }
    try {
      await api.choose(p, withPin)
      onDone()
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e))
      setPin('')
    }
  }

  // The remote's number keys type the PIN; Back leaves it.
  useEffect(() => {
    if (!locked) return
    return onKey((code) => {
      if (code >= 48 && code <= 57) {
        typed(String(code - 48))
        return true
      }
      if (code === KEY.back) {
        setLocked(null)
        return true
      }
      return false
    })
  })
  function typed(d: string) {
    const next = (pinRef.current + d).slice(0, 8)
    setPin(next)
    if (locked && next.length === 4) void pick(locked, next)
  }

  if (!list) return <div className="start">{problem ? <p className="problem">{problem}</p> : <Spinner />}</div>
  if (locked) {
    return (
      <div className="start" ref={ref}>
        <Avatar avatar={locked.avatar} name={locked.name} size={7} />
        <h1>{locked.name}'s PIN</h1>
        <p className="pin">{'●'.repeat(pin.length) + '○'.repeat(Math.max(0, 4 - pin.length))}</p>
        <div className="pad">
          {['1', '2', '3', '4', '5', '6', '7', '8', '9', '0'].map((d) => (
            <Btn key={d} k={d} first={d === '1'} onClick={() => typed(d)}>
              {d}
            </Btn>
          ))}
        </div>
        <p className="problem">{problem}</p>
      </div>
    )
  }
  return (
    <div className="start" ref={ref}>
      <h1>Who's watching?</h1>
      <div className="who" data-g="who">
        {list.map((p, i) => (
          <button key={p.id} className="who-tile" data-f="" data-k={String(p.id)} data-first={p.id === saved.profileId || (!saved.profileId && i === 0) ? '' : undefined} onClick={() => void pick(p)}>
            <Avatar avatar={p.avatar} name={p.name} size={9} />
            <span>{p.name}</span>
          </button>
        ))}
      </div>
      <p className="problem">{problem}</p>
      <Btn onClick={onSignOut}>Sign out</Btn>
    </div>
  )
}

