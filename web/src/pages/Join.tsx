import { useEffect, useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useAuth } from '../AuthContext'
import { useDarkPage } from '../useDarkPage'
import CueWordmark from '../components/CueWordmark'
import PasswordStrength from '../components/PasswordStrength'
import { titleFor } from '../documentTitle'
import * as check from '../validate'
import { FieldError, FormProblem, useValidation } from '../useValidation'

// Joining from a family invite link (/join/{token}): your own account on
// this Cue, made by you. It watches with the household's streaming, and
// you're signed in straight away.
export default function Join() {
  // Shown before sign-in, outside the router's routes: the code is read
  // from the address itself.
  const token = decodeURIComponent(useLocation().pathname.split('/join/')[1] ?? '').split('/')[0]
  const { refresh } = useAuth()
  const navigate = useNavigate()
  useDarkPage(true)
  const [state, setState] = useState<'checking' | 'ok' | 'ended'>('checking')
  const [problem, setProblem] = useState('')
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [again, setAgain] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    document.title = titleFor('Join')
    api
      .checkInvite(token)
      .then(() => setState('ok'))
      .catch((e) => {
        setProblem(e instanceof Error ? e.message : String(e))
        setState('ended')
      })
  }, [token])

  const v = useValidation({
    username: check.username(username),
    password: check.password(password),
    again: check.passwordsMatch(password, again),
  })

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!v.attempt()) return
    setProblem('')
    setBusy(true)
    try {
      await api.joinWithInvite(token, { name: name.trim(), username: username.trim(), password })
      await refresh()
      navigate('/', { replace: true })
    } catch (err) {
      setProblem(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="auth-page">
      <form className="auth-card" onSubmit={onSubmit}>
        <h1 className="sr-only">Make your Cue account</h1>
        <div className="auth-logo">
          <CueWordmark className="auth-wordmark" />
        </div>
        {state === 'checking' && <p className="auth-sub">Checking your invite…</p>}
        {state === 'ended' && (
          <>
            <p className="auth-sub">{problem || 'This invite link has ended. Ask for a new one.'}</p>
            <Link className="btn-link primary-look" to="/login">
              Sign in instead
            </Link>
          </>
        )}
        {state === 'ok' && (
          <>
            <p className="auth-sub">You're invited. Make your own account to start watching.</p>
            {problem && <p className="error-text">{problem}</p>}
            <label>
              Your name
              <input value={name} onChange={(e) => setName(e.target.value)} autoFocus autoComplete="name" placeholder="What everyone calls you" maxLength={80} />
            </label>
            <label>
              Username
              <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoCapitalize="none" spellCheck={false} {...v.bind('username', username, setUsername)} />
              <FieldError v={v} name="username" />
            </label>
            <label>
              Password
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" {...v.bind('password')} />
              <PasswordStrength password={password} />
              <FieldError v={v} name="password" />
            </label>
            <label>
              Password again
              <input type="password" value={again} onChange={(e) => setAgain(e.target.value)} autoComplete="new-password" {...v.bind('again')} />
              <FieldError v={v} name="again" />
            </label>
            <button className="primary" type="submit" disabled={busy}>
              {busy ? 'Making your account…' : 'Make my account'}
            </button>
            <FormProblem v={v} verb="make your account" />
            <p className="auth-sub" style={{ marginBottom: 0 }}>
              Already have one? <Link to="/login">Sign in</Link>
            </p>
          </>
        )}
      </form>
    </main>
  )
}
